/*
Copyright 2025 The Kubernetes Authors.
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
	"math/rand"
	"strconv"

	// "crypto/sha256"
	// "encoding/hex"
	"fmt"

	// "fmt"

	// "fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/external-dns/source/informers"

	// "k8s.io/apimachinery/pkg/fields"
	// v1 "k8s.io/client-go/applyconfigurations/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	// "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/registry/generic"
	// "k8s.io/client-go/tools/cache"

	// discoveryv1 "k8s.io/api/discovery/v1"

	// "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestEndpointsForHostname(t *testing.T) {
	tests := []struct {
		name             string
		hostname         string
		targets          endpoint.Targets
		ttl              endpoint.TTL
		providerSpecific endpoint.ProviderSpecific
		setIdentifier    string
		resource         string
		expected         []*endpoint.Endpoint
	}{
		{
			name:     "A record targets",
			hostname: "example.com",
			targets:  endpoint.Targets{"192.0.2.1", "192.0.2.2"},
			ttl:      endpoint.TTL(300),
			providerSpecific: endpoint.ProviderSpecific{
				{Name: "provider", Value: "value"},
			},
			setIdentifier: "identifier",
			resource:      "resource",
			expected: []*endpoint.Endpoint{
				{
					DNSName:          "example.com",
					Targets:          endpoint.Targets{"192.0.2.1", "192.0.2.2"},
					RecordType:       endpoint.RecordTypeA,
					RecordTTL:        endpoint.TTL(300),
					ProviderSpecific: endpoint.ProviderSpecific{{Name: "provider", Value: "value"}},
					SetIdentifier:    "identifier",
					Labels:           map[string]string{endpoint.ResourceLabelKey: "resource"},
				},
			},
		},
		{
			name:     "AAAA record targets",
			hostname: "example.com",
			targets:  endpoint.Targets{"2001:db8::1", "2001:db8::2"},
			ttl:      endpoint.TTL(300),
			providerSpecific: endpoint.ProviderSpecific{
				{Name: "provider", Value: "value"},
			},
			setIdentifier: "identifier",
			resource:      "resource",
			expected: []*endpoint.Endpoint{
				{
					DNSName:          "example.com",
					Targets:          endpoint.Targets{"2001:db8::1", "2001:db8::2"},
					RecordType:       endpoint.RecordTypeAAAA,
					RecordTTL:        endpoint.TTL(300),
					ProviderSpecific: endpoint.ProviderSpecific{{Name: "provider", Value: "value"}},
					SetIdentifier:    "identifier",
					Labels:           map[string]string{endpoint.ResourceLabelKey: "resource"},
				},
			},
		},
		{
			name:     "CNAME record targets",
			hostname: "example.com",
			targets:  endpoint.Targets{"cname.example.com"},
			ttl:      endpoint.TTL(300),
			providerSpecific: endpoint.ProviderSpecific{
				{Name: "provider", Value: "value"},
			},
			setIdentifier: "identifier",
			resource:      "resource",
			expected: []*endpoint.Endpoint{
				{
					DNSName:          "example.com",
					Targets:          endpoint.Targets{"cname.example.com"},
					RecordType:       endpoint.RecordTypeCNAME,
					RecordTTL:        endpoint.TTL(300),
					ProviderSpecific: endpoint.ProviderSpecific{{Name: "provider", Value: "value"}},
					SetIdentifier:    "identifier",
					Labels:           map[string]string{endpoint.ResourceLabelKey: "resource"},
				},
			},
		},
		{
			name:             "No targets",
			hostname:         "example.com",
			targets:          endpoint.Targets{},
			ttl:              endpoint.TTL(300),
			providerSpecific: endpoint.ProviderSpecific{},
			setIdentifier:    "",
			resource:         "",
			expected:         []*endpoint.Endpoint(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := endpointsForHostname(tt.hostname, tt.targets, tt.ttl, tt.providerSpecific, tt.setIdentifier, tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEndpointTargetsFromServices(t *testing.T) {
	tests := []struct {
		name      string
		services  []*corev1.Service
		namespace string
		selector  map[string]string
		expected  endpoint.Targets
		wantErr   bool
	}{
		{
			name:      "no services",
			services:  []*corev1.Service{},
			namespace: "default",
			selector:  map[string]string{"app": "nginx"},
			expected:  endpoint.Targets{},
			wantErr:   false,
		},
		{
			name: "matching service with external IPs",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc1",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector:    map[string]string{"app": "nginx"},
						ExternalIPs: []string{"192.0.2.1", "158.123.32.23"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc2",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector:    map[string]string{"app": "nginx"},
						ExternalIPs: []string{"192.0.2.1", "158.123.32.23"},
					},
				},
				// {
				// 	ObjectMeta: metav1.ObjectMeta{
				// 		Name:      "svc3",
				// 		Namespace: "default",
				// 	},
				// 	Spec: corev1.ServiceSpec{
				// 		Selector:    map[string]string{"app": "nginx3"},
				// 		ExternalIPs: []string{"192.0.2.1", "158.123.32.23"},
				// 	},
				// },
			},
			namespace: "default",
			selector:  map[string]string{"app": "nginx"},
			expected:  endpoint.Targets{"192.0.2.1", "158.123.32.23"},
			wantErr:   false,
		},
		{
			name: "matching service with load balancer IP",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc2",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "nginx"},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "192.0.2.2"},
							},
						},
					},
				},
			},
			namespace: "default",
			selector:  map[string]string{"app": "nginx"},
			expected:  endpoint.Targets{"192.0.2.2"},
			wantErr:   false,
		},
		{
			name: "matching service with load balancer hostname",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc3",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "nginx"},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{Hostname: "lb.example.com"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc4",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "web"},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{Hostname: "lb.example.com"},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc5",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "nginx"},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{Hostname: "lb.example.com"},
							},
						},
					},
				},
			},
			namespace: "default",
			selector:  map[string]string{"app": "nginx"},
			expected:  endpoint.Targets{"lb.example.com"},
			wantErr:   false,
		},
		{
			name: "no matching services",
			services: []*corev1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc4",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{"app": "apache"},
					},
				},
			},
			namespace: "default",
			selector:  map[string]string{"app": "nginx"},
			expected:  endpoint.Targets{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()
			informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(client, 0, kubeinformers.WithNamespace(tt.namespace))
			// kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			//					options.LabelSelector = metav1.FormatLabelSelector(metav1.SetAsLabelSelector(tt.selector))
			//				})
			serviceInformer := informerFactory.Core().V1().Services()

			for _, svc := range tt.services {
				_, err := client.CoreV1().Services(tt.namespace).Create(context.Background(), svc, metav1.CreateOptions{})
				assert.NoError(t, err)

				// err = serviceInformer.Informer().AddIndexers(cache.Indexers{
				// 	"bySelector": func(obj any) ([]string, error) {
				// 		fmt.Println("adding indexer for serviceID")
				// 		svcSlice, ok := obj.(*corev1.Service)
				// 		if !ok {
				// 			return nil, nil
				// 		}
				// 		serviceName, ok := svcSlice.Labels[corev1.LabelMetadataName]
				// 		if !ok {
				// 			fmt.Println("hm not adding indexer for serviceID, no name label")
				// 			return nil, nil
				// 		}
				// 		key := cache.ObjectName{Namespace: svcSlice.Namespace, Name: serviceName}.String()
				// 		fmt.Println("adding indexer for serviceID:", key)
				// 		return []string{key}, nil
				// 	},
				// })

				// Register the index BEFORE the informer runs!
				BySelectorIndex := "bySelector"
				err = serviceInformer.Informer().AddIndexers(cache.Indexers{
					BySelectorIndex: func(obj interface{}) ([]string, error) {
						svc := obj.(*corev1.Service)
						return []string{labels.Set(svc.Spec.Selector).String()}, nil
					},
				})
				if err != nil {
					panic(err)
				}

				err = serviceInformer.Informer().GetIndexer().Add(svc)
				assert.NoError(t, err)

				r, err := serviceInformer.Informer().GetIndexer().ByIndex(BySelectorIndex, "app=nginx")
				assert.NoError(t, err)
				fmt.Println("results:", len(r))
			}

			result, err := EndpointTargetsFromServices(serviceInformer, tt.namespace, tt.selector)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEndpointTargetsFromServicesPods(t *testing.T) {
	client := fake.NewClientset()

	services := []*corev1.Service{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-edge",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "nginx", "env": "prod"},
				ExternalIPs: []string{"192.0.2.3"},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-internal",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "nginx"}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-edge-secondary",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "nginx", "env": "prod"}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-edge-fail-over",
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "nginx", "env": "prod"}},
		},
	}

	// ─── Informer with custom “bySelector” indexer ──────────────────────────────
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(client, 0, kubeinformers.WithNamespace("default"))
	svcInformer := informerFactory.Core().V1().Services()

	err := svcInformer.Informer().AddIndexers(cache.Indexers{
		informers.SpecSelectorIndex: func(obj interface{}) ([]string, error) {
			svc := obj.(*corev1.Service)
			return []string{labels.Set(svc.Spec.Selector).String()}, nil
		},
	})
	assert.NoError(t, err)

	for _, svc := range services {
		_, err := client.CoreV1().Services(svc.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	go informerFactory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.Informer().HasSynced)

	// ─── Lookup all services that share selector app=nginx ─────────────────────
	keys := svcInformer.Informer().GetIndexer().GetIndexers()
	fmt.Println("keys:", keys)

	key := labels.Set(map[string]string{"app": "nginx", "env": "prod"}).String() // "app=nginx"
	objs, _ := svcInformer.Informer().GetIndexer().ByIndex(informers.SpecSelectorIndex, key)

	fmt.Printf("Services with selector %q:\n", key)
	for _, o := range objs {
		fmt.Println("-", o.(*corev1.Service).Labels)
	}
}

func populateWithServices(b *testing.B, client *fake.Clientset, correct, random int) coreinformers.ServiceInformer {
	informerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(client, 0, kubeinformers.WithNamespace("default"))
	svcInformer := informerFactory.Core().V1().Services()

	_, err := svcInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
			},
		},
	)
	assert.NoError(b, err)

	err = svcInformer.Informer().AddIndexers(informers.ServiceIndexers)
	assert.NoError(b, err)

	svc := generateServices(correct, random)
	for _, svc := range svc {
		_, err := client.CoreV1().Services(svc.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
		assert.NoError(b, err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	go informerFactory.Start(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.Informer().HasSynced)
	return svcInformer
}

func BenchmarkMyFunctionWithIndexing(b *testing.B) {
	client := fake.NewClientset()

	svcInformer := populateWithServices(b, client, 50, 40000)

	key := labels.Set(map[string]string{"app": "nginx", "env": "prod"}).String()

	for b.Loop() {
		svc, _ := svcInformer.Informer().GetIndexer().ByIndex(informers.SpecSelectorIndex, key)
		assert.Len(b, svc, 50)
	}
}

func BenchmarkMyFunctionWithoutIndexing(b *testing.B) {
	// Setup code might run multiple times
	client := fake.NewClientset()
	svcInformer := populateWithServices(b, client, 50, 40000)

	sel := map[string]string{"app": "nginx", "env": "prod"}

	for b.Loop() {
		svc, _ := svcInformer.Lister().Services("").List(labels.Everything())
		count := 0
		for _, svc := range svc {
			if !MatchesServiceSelector(sel, svc.Spec.Selector) {
				continue
			}
			count++
			// Simulate some processing
			_ = svc.Name // Just to use the service and avoid compiler optimization
		}
		assert.Equal(b, 50, count)
	}
}

func TestGenerateServices(t *testing.T) {
	services := generateServices(6, 44)

	assert.Len(t, services, 50, "should generate 50 services")
}

func randomLabels() map[string]string {
	return map[string]string{
		fmt.Sprintf("key%d", rand.Intn(100)): fmt.Sprintf("value%d", rand.Intn(100)),
	}
}

func generateServices(correct, random int) []*corev1.Service {
	var services []*corev1.Service

	// 5 services with specific labels
	for i := 0; i < correct; i++ {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-svc-" + strconv.Itoa(i),
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "nginx", "env": "prod"},
			},
		}
		services = append(services, svc)
	}

	for i := 0; i < random; i++ {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-svc-" + strconv.Itoa(i),
				Namespace: "default",
			},
			Spec: corev1.ServiceSpec{
				Selector: randomLabels(),
			},
		}
		services = append(services, svc)
	}

	// Shuffle the services to ensure randomness
	for i := 0; i < 3; i++ {
		rand.Shuffle(len(services), func(i, j int) {
			services[i], services[j] = services[j], services[i]
		})
	}

	return services
}

// 7408231982
// 7373094914
