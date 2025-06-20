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
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/linki/instrumented_http"
	openshift "github.com/openshift/client-go/route/clientset/versioned"
	log "github.com/sirupsen/logrus"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
)

// ErrSourceNotFound is returned when a requested source doesn't exist.
var ErrSourceNotFound = errors.New("source not found")

// Config holds shared configuration options for all Sources.
type Config struct {
	Namespace                      string
	AnnotationFilter               string
	LabelFilter                    labels.Selector
	IngressClassNames              []string
	FQDNTemplate                   string
	CombineFQDNAndAnnotation       bool
	IgnoreHostnameAnnotation       bool
	IgnoreNonHostNetworkPods       bool
	IgnoreIngressTLSSpec           bool
	IgnoreIngressRulesSpec         bool
	ListenEndpointEvents           bool
	GatewayName                    string
	GatewayNamespace               string
	GatewayLabelFilter             string
	Compatibility                  string
	PodSourceDomain                string
	PublishInternal                bool
	PublishHostIP                  bool
	AlwaysPublishNotReadyAddresses bool
	ConnectorServer                string
	CRDSourceAPIVersion            string
	CRDSourceKind                  string
	KubeConfig                     string
	APIServerURL                   string
	ServiceTypeFilter              []string
	CFAPIEndpoint                  string
	CFUsername                     string
	CFPassword                     string
	GlooNamespaces                 []string
	SkipperRouteGroupVersion       string
	RequestTimeout                 time.Duration
	DefaultTargets                 []string
	ForceDefaultTargets            bool
	OCPRouterName                  string
	UpdateEvents                   bool
	ResolveLoadBalancerHostname    bool
	TraefikDisableLegacy           bool
	TraefikDisableNew              bool
	ExcludeUnschedulable           bool
	ExposeInternalIPv6             bool
}

func NewSourceConfig(cfg *externaldns.Config) *Config {
	// error is explicitly ignored because the filter is already validated in validation.ValidateConfig
	labelSelector, _ := labels.Parse(cfg.LabelFilter)
	return &Config{
		Namespace:                      cfg.Namespace,
		AnnotationFilter:               cfg.AnnotationFilter,
		LabelFilter:                    labelSelector,
		IngressClassNames:              cfg.IngressClassNames,
		FQDNTemplate:                   cfg.FQDNTemplate,
		CombineFQDNAndAnnotation:       cfg.CombineFQDNAndAnnotation,
		IgnoreHostnameAnnotation:       cfg.IgnoreHostnameAnnotation,
		IgnoreNonHostNetworkPods:       cfg.IgnoreNonHostNetworkPods,
		IgnoreIngressTLSSpec:           cfg.IgnoreIngressTLSSpec,
		IgnoreIngressRulesSpec:         cfg.IgnoreIngressRulesSpec,
		ListenEndpointEvents:           cfg.ListenEndpointEvents,
		GatewayName:                    cfg.GatewayName,
		GatewayNamespace:               cfg.GatewayNamespace,
		GatewayLabelFilter:             cfg.GatewayLabelFilter,
		Compatibility:                  cfg.Compatibility,
		PodSourceDomain:                cfg.PodSourceDomain,
		PublishInternal:                cfg.PublishInternal,
		PublishHostIP:                  cfg.PublishHostIP,
		AlwaysPublishNotReadyAddresses: cfg.AlwaysPublishNotReadyAddresses,
		ConnectorServer:                cfg.ConnectorSourceServer,
		CRDSourceAPIVersion:            cfg.CRDSourceAPIVersion,
		CRDSourceKind:                  cfg.CRDSourceKind,
		KubeConfig:                     cfg.KubeConfig,
		APIServerURL:                   cfg.APIServerURL,
		ServiceTypeFilter:              cfg.ServiceTypeFilter,
		CFAPIEndpoint:                  cfg.CFAPIEndpoint,
		CFUsername:                     cfg.CFUsername,
		CFPassword:                     cfg.CFPassword,
		GlooNamespaces:                 cfg.GlooNamespaces,
		SkipperRouteGroupVersion:       cfg.SkipperRouteGroupVersion,
		RequestTimeout:                 cfg.RequestTimeout,
		DefaultTargets:                 cfg.DefaultTargets,
		ForceDefaultTargets:            cfg.ForceDefaultTargets,
		OCPRouterName:                  cfg.OCPRouterName,
		UpdateEvents:                   cfg.UpdateEvents,
		ResolveLoadBalancerHostname:    cfg.ResolveServiceLoadBalancerHostname,
		TraefikDisableLegacy:           cfg.TraefikDisableLegacy,
		TraefikDisableNew:              cfg.TraefikDisableNew,
		ExcludeUnschedulable:           cfg.ExcludeUnschedulable,
		ExposeInternalIPv6:             cfg.ExposeInternalIPV6,
	}
}

// ClientGenerator provides clients
type ClientGenerator interface {
	KubeClient() (kubernetes.Interface, error)
	GatewayClient() (gateway.Interface, error)
	IstioClient() (istioclient.Interface, error)
	CloudFoundryClient(cfAPPEndpoint string, cfUsername string, cfPassword string) (*cfclient.Client, error)
	DynamicKubernetesClient() (dynamic.Interface, error)
	OpenShiftClient() (openshift.Interface, error)
}

// SingletonClientGenerator stores provider clients and guarantees that only one instance of client
// will be generated
type SingletonClientGenerator struct {
	KubeConfig      string
	APIServerURL    string
	RequestTimeout  time.Duration
	kubeClient      kubernetes.Interface
	gatewayClient   gateway.Interface
	istioClient     *istioclient.Clientset
	cfClient        *cfclient.Client
	dynKubeClient   dynamic.Interface
	openshiftClient openshift.Interface
	kubeOnce        sync.Once
	gatewayOnce     sync.Once
	istioOnce       sync.Once
	cfOnce          sync.Once
	dynCliOnce      sync.Once
	openshiftOnce   sync.Once
}

// KubeClient generates a kube client if it was not created before
func (p *SingletonClientGenerator) KubeClient() (kubernetes.Interface, error) {
	var err error
	p.kubeOnce.Do(func() {
		p.kubeClient, err = NewKubeClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.kubeClient, err
}

// GatewayClient generates a gateway client if it was not created before
func (p *SingletonClientGenerator) GatewayClient() (gateway.Interface, error) {
	var err error
	p.gatewayOnce.Do(func() {
		p.gatewayClient, err = newGatewayClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.gatewayClient, err
}

func newGatewayClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (gateway.Interface, error) {
	config, err := instrumentedRESTConfig(kubeConfig, apiServerURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	client, err := gateway.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Infof("Created GatewayAPI client %s", config.Host)
	return client, nil
}

// IstioClient generates an istio go client if it was not created before
func (p *SingletonClientGenerator) IstioClient() (istioclient.Interface, error) {
	var err error
	p.istioOnce.Do(func() {
		p.istioClient, err = NewIstioClient(p.KubeConfig, p.APIServerURL)
	})
	return p.istioClient, err
}

// CloudFoundryClient generates a cf client if it was not created before
func (p *SingletonClientGenerator) CloudFoundryClient(cfAPIEndpoint string, cfUsername string, cfPassword string) (*cfclient.Client, error) {
	var err error
	p.cfOnce.Do(func() {
		p.cfClient, err = NewCFClient(cfAPIEndpoint, cfUsername, cfPassword)
	})
	return p.cfClient, err
}

// NewCFClient return a new CF client object.
func NewCFClient(cfAPIEndpoint string, cfUsername string, cfPassword string) (*cfclient.Client, error) {
	c := &cfclient.Config{
		ApiAddress: "https://" + cfAPIEndpoint,
		Username:   cfUsername,
		Password:   cfPassword,
	}
	client, err := cfclient.NewClient(c)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// DynamicKubernetesClient generates a dynamic client if it was not created before
func (p *SingletonClientGenerator) DynamicKubernetesClient() (dynamic.Interface, error) {
	var err error
	p.dynCliOnce.Do(func() {
		p.dynKubeClient, err = NewDynamicKubernetesClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.dynKubeClient, err
}

// OpenShiftClient generates an openshift client if it was not created before
func (p *SingletonClientGenerator) OpenShiftClient() (openshift.Interface, error) {
	var err error
	p.openshiftOnce.Do(func() {
		p.openshiftClient, err = NewOpenShiftClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.openshiftClient, err
}

// ByNames returns multiple Sources given multiple names.
func ByNames(ctx context.Context, p ClientGenerator, names []string, cfg *Config) ([]Source, error) {
	sources := []Source{}
	for _, name := range names {
		source, err := BuildWithConfig(ctx, name, p, cfg)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}

	return sources, nil
}

// BuildWithConfig allows generating a Source implementation from the shared config
func BuildWithConfig(ctx context.Context, source string, p ClientGenerator, cfg *Config) (Source, error) {
	switch source {
	case "node":
		client, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		return NewNodeSource(ctx, client, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.LabelFilter, cfg.ExposeInternalIPv6, cfg.ExcludeUnschedulable, cfg.CombineFQDNAndAnnotation)
	case "service":
		client, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		return NewServiceSource(ctx, client, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.Compatibility, cfg.PublishInternal, cfg.PublishHostIP, cfg.AlwaysPublishNotReadyAddresses, cfg.ServiceTypeFilter, cfg.IgnoreHostnameAnnotation, cfg.LabelFilter, cfg.ResolveLoadBalancerHostname, cfg.ListenEndpointEvents, cfg.ExposeInternalIPv6)
	case "ingress":
		client, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		return NewIngressSource(ctx, client, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation, cfg.IgnoreIngressTLSSpec, cfg.IgnoreIngressRulesSpec, cfg.LabelFilter, cfg.IngressClassNames)
	case "pod":
		client, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		return NewPodSource(ctx, client, cfg.Namespace, cfg.Compatibility, cfg.IgnoreNonHostNetworkPods, cfg.PodSourceDomain, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation)
	case "gateway-httproute":
		return NewGatewayHTTPRouteSource(p, cfg)
	case "gateway-grpcroute":
		return NewGatewayGRPCRouteSource(p, cfg)
	case "gateway-tlsroute":
		return NewGatewayTLSRouteSource(p, cfg)
	case "gateway-tcproute":
		return NewGatewayTCPRouteSource(p, cfg)
	case "gateway-udproute":
		return NewGatewayUDPRouteSource(p, cfg)
	case "istio-gateway":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		istioClient, err := p.IstioClient()
		if err != nil {
			return nil, err
		}
		return NewIstioGatewaySource(ctx, kubernetesClient, istioClient, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation)
	case "istio-virtualservice":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		istioClient, err := p.IstioClient()
		if err != nil {
			return nil, err
		}
		return NewIstioVirtualServiceSource(ctx, kubernetesClient, istioClient, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation)
	case "cloudfoundry":
		cfClient, err := p.CloudFoundryClient(cfg.CFAPIEndpoint, cfg.CFUsername, cfg.CFPassword)
		if err != nil {
			return nil, err
		}
		return NewCloudFoundrySource(cfClient)
	case "ambassador-host":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewAmbassadorHostSource(ctx, dynamicClient, kubernetesClient, cfg.Namespace, cfg.AnnotationFilter, cfg.LabelFilter)
	case "contour-httpproxy":
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewContourHTTPProxySource(ctx, dynamicClient, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation)
	case "gloo-proxy":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewGlooSource(dynamicClient, kubernetesClient, cfg.GlooNamespaces)
	case "traefik-proxy":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewTraefikSource(ctx, dynamicClient, kubernetesClient, cfg.Namespace, cfg.AnnotationFilter, cfg.IgnoreHostnameAnnotation, cfg.TraefikDisableLegacy, cfg.TraefikDisableNew)
	case "openshift-route":
		ocpClient, err := p.OpenShiftClient()
		if err != nil {
			return nil, err
		}
		return NewOcpRouteSource(ctx, ocpClient, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation, cfg.LabelFilter, cfg.OCPRouterName)
	case "fake":
		return NewFakeSource(cfg.FQDNTemplate)
	case "connector":
		return NewConnectorSource(cfg.ConnectorServer)
	case "crd":
		client, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		crdClient, scheme, err := NewCRDClientForAPIVersionKind(client, cfg.KubeConfig, cfg.APIServerURL, cfg.CRDSourceAPIVersion, cfg.CRDSourceKind)
		if err != nil {
			return nil, err
		}
		return NewCRDSource(crdClient, cfg.Namespace, cfg.CRDSourceKind, cfg.AnnotationFilter, cfg.LabelFilter, scheme, cfg.UpdateEvents)
	case "skipper-routegroup":
		apiServerURL := cfg.APIServerURL
		tokenPath := ""
		token := ""
		restConfig, err := GetRestConfig(cfg.KubeConfig, cfg.APIServerURL)
		if err == nil {
			apiServerURL = restConfig.Host
			tokenPath = restConfig.BearerTokenFile
			token = restConfig.BearerToken
		}
		return NewRouteGroupSource(cfg.RequestTimeout, token, tokenPath, apiServerURL, cfg.Namespace, cfg.AnnotationFilter, cfg.FQDNTemplate, cfg.SkipperRouteGroupVersion, cfg.CombineFQDNAndAnnotation, cfg.IgnoreHostnameAnnotation)
	case "kong-tcpingress":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewKongTCPIngressSource(ctx, dynamicClient, kubernetesClient, cfg.Namespace, cfg.AnnotationFilter, cfg.IgnoreHostnameAnnotation)
	case "f5-virtualserver":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewF5VirtualServerSource(ctx, dynamicClient, kubernetesClient, cfg.Namespace, cfg.AnnotationFilter)
	case "f5-transportserver":
		kubernetesClient, err := p.KubeClient()
		if err != nil {
			return nil, err
		}
		dynamicClient, err := p.DynamicKubernetesClient()
		if err != nil {
			return nil, err
		}
		return NewF5TransportServerSource(ctx, dynamicClient, kubernetesClient, cfg.Namespace, cfg.AnnotationFilter)
	}

	return nil, ErrSourceNotFound
}

func instrumentedRESTConfig(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*rest.Config, error) {
	config, err := GetRestConfig(kubeConfig, apiServerURL)
	if err != nil {
		return nil, err
	}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return instrumented_http.NewTransport(rt, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		})
	}
	config.Timeout = requestTimeout
	return config, nil
}

// GetRestConfig returns the rest clients config to get automatically
// data if you run inside a cluster or by passing flags.
func GetRestConfig(kubeConfig, apiServerURL string) (*rest.Config, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}
	log.Debugf("apiServerURL: %s", apiServerURL)
	log.Debugf("kubeConfig: %s", kubeConfig)

	// evaluate whether to use kubeConfig-file or serviceaccount-token
	var (
		config *rest.Config
		err    error
	)
	if kubeConfig == "" {
		log.Infof("Using inCluster-config based on serviceaccount-token")
		config, err = rest.InClusterConfig()
	} else {
		log.Infof("Using kubeConfig")
		config, err = clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	}
	if err != nil {
		return nil, err
	}

	return config, nil
}

// NewKubeClient returns a new Kubernetes client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func NewKubeClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*kubernetes.Clientset, error) {
	log.Infof("Instantiating new Kubernetes client")
	config, err := instrumentedRESTConfig(kubeConfig, apiServerURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Infof("Created Kubernetes client %s", config.Host)
	return client, nil
}

// NewIstioClient returns a new Istio client object. It uses the configured
// KubeConfig attribute to connect to the cluster. If KubeConfig isn't provided
// it defaults to using the recommended default.
// NB: Istio controls the creation of the underlying Kubernetes client, so we
// have no ability to tack on transport wrappers (e.g., Prometheus request
// wrappers) to the client's config at this level. Furthermore, the Istio client
// constructor does not expose the ability to override the Kubernetes API server endpoint,
// so the apiServerURL config attribute has no effect.
func NewIstioClient(kubeConfig string, apiServerURL string) (*istioclient.Clientset, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}

	restCfg, err := clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	if err != nil {
		return nil, err
	}

	ic, err := istioclient.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create istio client: %w", err)
	}

	return ic, nil
}

// NewDynamicKubernetesClient returns a new Dynamic Kubernetes client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func NewDynamicKubernetesClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (dynamic.Interface, error) {
	config, err := instrumentedRESTConfig(kubeConfig, apiServerURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Infof("Created Dynamic Kubernetes client %s", config.Host)
	return client, nil
}

// NewOpenShiftClient returns a new Openshift client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func NewOpenShiftClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*openshift.Clientset, error) {
	config, err := instrumentedRESTConfig(kubeConfig, apiServerURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	client, err := openshift.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Infof("Created OpenShift client %s", config.Host)
	return client, nil
}
