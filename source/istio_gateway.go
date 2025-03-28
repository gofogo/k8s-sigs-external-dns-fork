/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package source

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"text/template"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/source/istio"
	// TODO: rename to istiov1
	v1 "istio.io/client-go/pkg/apis/networking/v1"
	// TODO: rename to istiov1alpha3
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	istioinformers "istio.io/client-go/pkg/informers/externalversions"
	networkingv1informer "istio.io/client-go/pkg/informers/externalversions/networking/v1"
	networkingv1alpha3informer "istio.io/client-go/pkg/informers/externalversions/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/external-dns/endpoint"
)

// IstioGatewayIngressSource is the annotation used to determine if the gateway is implemented by an Ingress object
// instead of a standard LoadBalancer service type
const IstioGatewayIngressSource = "external-dns.alpha.kubernetes.io/ingress"

// gatewaySource is an implementation of Source for Istio Gateway objects.
// The gateway implementation uses the spec.servers.hosts values for the hostnames.
// Use targetAnnotationKey to explicitly set Endpoint.
type gatewaySource struct {
	kubeClient               kubernetes.Interface
	istioClient              istioclient.Interface
	namespace                string
	annotationFilter         string
	fqdnTemplate             *template.Template
	combineFQDNAnnotation    bool
	ignoreHostnameAnnotation bool
	serviceInformer          coreinformers.ServiceInformer
	gateway                  istio.Gateway
	gatewayInformerV1        networkingv1informer.GatewayInformer
	// TODO: keep for backward compatibility. TBD removal in future releases
	gatewayInformerV1Alpha3 networkingv1alpha3informer.GatewayInformer
}

// NewIstioGatewaySource creates a new gatewaySource with the given config.
func NewIstioGatewaySource(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	istioClient istioclient.Interface,
	namespace string,
	annotationFilter string,
	fqdnTemplate string,
	combineFQDNAnnotation bool,
	ignoreHostnameAnnotation bool,
) (Source, error) {
	tmpl, err := parseTemplate(fqdnTemplate)
	if err != nil {
		return nil, err
	}

	// IS networking.istio.io/v1 or networking.istio.io/v1alpha3
	version := "v1"
	_, e := istioClient.Discovery().ServerResourcesForGroupVersion("networking.istio.io/v1")
	if e != nil {
		log.Warningf("istio CRD version 'networking.istio.io/v1' not installed %s", e)
		log.Warning("in future releases only 'networking.istio.io/v1' will be supported")
		_, e = istioClient.Discovery().ServerResourcesForGroupVersion("networking.istio.io/v1alpha3")
		if e != nil {
			return nil, fmt.Errorf("istio CRD version 'networking.istio.io/v1alpha3' not installed %s", e)
		} else {
			version = "v1alpha3"
		}
	}

	// Use shared informers to listen for add/update/delete of services/pods/nodes in the specified namespace.
	// Set resync period to 0, to prevent processing when nothing has changed
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 0, kubeinformers.WithNamespace(namespace))
	serviceInformer := informerFactory.Core().V1().Services()
	istioInformerFactory := istioinformers.NewSharedInformerFactory(istioClient, 0)

	// Add default resource event handlers to properly initialize informer.
	serviceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// TODO: test this
				if log.IsLevelEnabled(log.DebugLevel) {
					service, ok := obj.(*corev1.Service)
					if !ok {
						log.Errorf("event handler not added. unexpected type '%s' when should be 'service/v1'", reflect.TypeOf(obj).String())
						return
					} else {
						log.Debugf("event handler added for 'service/v1' in 'namespace:%s' with 'name:%s'.", service.Name, service.Namespace)
					}
				}
			},
		},
	)

	var gwInformerV1 networkingv1informer.GatewayInformer
	var gwInformerV1Alpha3 networkingv1alpha3informer.GatewayInformer

	if version == "v1" {
		gwInformerV1 = istioInformerFactory.Networking().V1().Gateways()
		gwInformerV1.Informer().AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					// TODO: test this
					if log.IsLevelEnabled(log.DebugLevel) {
						service, ok := obj.(*v1.Gateway)
						if !ok {
							log.Errorf("event handler not added. unexpected type '%s' when should be 'gateway.networking.istio.io/v1", reflect.TypeOf(obj).String())
							return
						} else {
							log.Debugf("event handler added for 'gateway.networking.istio.io/v1' in 'namespace:%s' with 'name:%s'", service.Name, service.Namespace)
						}
					}
				},
			},
		)
	} else {
		gwInformerV1Alpha3 = istioInformerFactory.Networking().V1alpha3().Gateways()
		gwInformerV1Alpha3.Informer().AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					// TODO: test this
					log.Debug("gateway added V1alpha3")
					service, ok := obj.(*networkingv1alpha3.Gateway)
					if !ok {
						log.Errorf("event handler not added. unexpected type '%s' when should be 'gateway.networking.istio.io/v1alpha3'", reflect.TypeOf(obj).String())
						return
					} else {
						log.Debugf("event handler added for 'gateway.networking.istio.io/v1alpha3' in 'namespace:%s' with 'name:%s'", service.Name, service.Namespace)
					}
				},
			},
		)
	}

	informerFactory.Start(ctx.Done())
	istioInformerFactory.Start(ctx.Done())

	// wait for the local cache to be populated.
	if err := waitForCacheSync(context.Background(), informerFactory); err != nil {
		return nil, err
	}
	if err := waitForCacheSync(context.Background(), istioInformerFactory); err != nil {
		return nil, err
	}

	return &gatewaySource{
		kubeClient:               kubeClient,
		istioClient:              istioClient,
		namespace:                namespace,
		annotationFilter:         annotationFilter,
		fqdnTemplate:             tmpl,
		combineFQDNAnnotation:    combineFQDNAnnotation,
		ignoreHostnameAnnotation: ignoreHostnameAnnotation,
		serviceInformer:          serviceInformer,
		gateway:                  istio.NewGateway(kubeClient),
		gatewayInformerV1:        gwInformerV1,
		gatewayInformerV1Alpha3:  gwInformerV1Alpha3,
	}, nil
}

// Endpoints returns endpoint objects for each host-target combination that should be processed.
// Retrieves all gateway resources in the source's namespace(s).
func (sc *gatewaySource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint
	var err error

	if sc.gatewayInformerV1 != nil {
		endpoints, err = sc.ExecuteV1(ctx, endpoints)
		if err != nil {
			return nil, err
		}
	} else {
		endpoints, err = sc.ExecuteV1Alpha3(ctx, endpoints)
		if err != nil {
			return nil, err
		}
	}

	for _, ep := range endpoints {
		sort.Sort(ep.Targets)
	}

	return endpoints, nil
}

func (sc *gatewaySource) ExecuteV1(ctx context.Context, endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	apiVersion := "networking.istio.io/v1"
	gwList, err := sc.istioClient.NetworkingV1().Gateways(sc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	gateways := gwList.Items
	gateways, err = istio.FilterByAnnotationsV1(sc.annotationFilter, gateways)
	if err != nil {
		return nil, err
	}
	log.Debugf("found '%d' 'gateway.%s'", len(gateways), apiVersion)

	for _, gateway := range gateways {
		log.Debugf("processing 'gateway.%s' with 'name:%s' in 'namespace:%s'", apiVersion, gateway.Name, gateway.Namespace)
		// Check controller annotation to see if we are responsible.
		controller, ok := gateway.Annotations[controllerAnnotationKey]
		if ok && controller != controllerAnnotationValue {
			log.Debugf("Skipping gateway %s/%s because controller value does not match, found: %s, required: %s",
				gateway.Namespace, gateway.Name, controller, controllerAnnotationValue)
			continue
		}
		gwHostnames, err := sc.hostNamesFromGatewayV1(gateway)
		if err != nil {
			return nil, err
		}

		// apply template if host is missing on gateway
		if (sc.combineFQDNAnnotation || len(gwHostnames) == 0) && sc.fqdnTemplate != nil {
			iHostnames, err := execTemplate(sc.fqdnTemplate, gateway)
			if err != nil {
				return nil, err
			}

			if sc.combineFQDNAnnotation {
				gwHostnames = append(gwHostnames, iHostnames...)
			} else {
				gwHostnames = iHostnames
			}
		}

		if len(gwHostnames) == 0 {
			log.Debugf("No hostnames could be generated from gateway %s/%s", gateway.Namespace, gateway.Name)
			continue
		}

		gwEndpoints, err := sc.endpointsFromGatewayV1(ctx, gwHostnames, gateway)
		if err != nil {
			return nil, err
		}

		if len(gwEndpoints) == 0 {
			log.Debugf("No endpoints could be generated from gateway %s/%s", gateway.Namespace, gateway.Name)
			continue
		}

		log.Debugf("Endpoints generated from gateway: %s/%s: %v", gateway.Namespace, gateway.Name, gwEndpoints)
		endpoints = append(endpoints, gwEndpoints...)
	}
	return endpoints, nil
}

func (sc *gatewaySource) ExecuteV1Alpha3(ctx context.Context, endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	apiVersion := "networking.istio.io/v1alpha3"
	gwList, err := sc.istioClient.NetworkingV1alpha3().Gateways(sc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	gateways := gwList.Items
	gateways, err = istio.FilterByAnnotationsV1Alpha3(sc.annotationFilter, gateways)
	if err != nil {
		return nil, err
	}
	log.Debugf("found '%d' 'gateway.%s'", len(gateways), apiVersion)

	for _, gateway := range gateways {
		log.Debugf("processing 'gateway.%s' with 'name:%s' in 'namespace:%s'", apiVersion, gateway.Name, gateway.Namespace)
		// Check controller annotation to see if we are responsible.
		controller, ok := gateway.Annotations[controllerAnnotationKey]
		if ok && controller != controllerAnnotationValue {
			log.Debugf("Skipping gateway %s/%s because controller value does not match, found: %s, required: %s",
				gateway.Namespace, gateway.Name, controller, controllerAnnotationValue)
			continue
		}
		gwHostnames, err := sc.hostNamesFromGatewayV1Alpha3(gateway)
		if err != nil {
			return nil, err
		}

		// apply template if host is missing on gateway
		if (sc.combineFQDNAnnotation || len(gwHostnames) == 0) && sc.fqdnTemplate != nil {
			iHostnames, err := execTemplate(sc.fqdnTemplate, gateway)
			if err != nil {
				return nil, err
			}

			if sc.combineFQDNAnnotation {
				gwHostnames = append(gwHostnames, iHostnames...)
			} else {
				gwHostnames = iHostnames
			}
		}

		if len(gwHostnames) == 0 {
			log.Debugf("No hostnames could be generated from gateway %s/%s", gateway.Namespace, gateway.Name)
			continue
		}

		gwEndpoints, err := sc.endpointsFromGatewayV1Alpha3(ctx, gwHostnames, gateway)
		if err != nil {
			return nil, err
		}

		if len(gwEndpoints) == 0 {
			log.Debugf("No endpoints could be generated from gateway %s/%s", gateway.Namespace, gateway.Name)
			continue
		}

		log.Debugf("Endpoints generated from gateway: %s/%s: %v", gateway.Namespace, gateway.Name, gwEndpoints)
		endpoints = append(endpoints, gwEndpoints...)
	}
	return endpoints, nil
}
