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

package integration

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	openshift "github.com/openshift/client-go/route/clientset/versioned"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/wrappers"
)

// TestScenarios represents the root structure of the YAML file.
type TestScenarios struct {
	Scenarios []Scenario `json:"scenarios"`
}

// Scenario represents a single test scenario.
type Scenario struct {
	Name      string                    `json:"name"`
	Sources   []string                  `json:"sources"`
	Config    ScenarioConfig            `json:"config"`
	Resources []k8sruntime.RawExtension `json:"resources"`
	Expected  []*endpoint.Endpoint      `json:"expected"`
}

// ScenarioConfig holds the wrapper configuration for a scenario.
type ScenarioConfig struct {
	DefaultTargets      []string `json:"defaultTargets"`
	ForceDefaultTargets bool     `json:"forceDefaultTargets"`
}

// LoadScenarios loads test scenarios from the YAML file.
func LoadScenarios(filename string) (*TestScenarios, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var scenarios TestScenarios
	if err := yaml.Unmarshal(data, &scenarios); err != nil {
		return nil, err
	}

	return &scenarios, nil
}

// GetScenariosPath returns the absolute path to the scenarios directory.
func GetScenariosPath() string {
	_, currentFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(currentFile), "scenarios")
}

// ParsedResources holds the parsed Kubernetes resources from a scenario.
type ParsedResources struct {
	Ingresses []*networkingv1.Ingress
	Services  []*corev1.Service
}

// ParseResources parses the raw resources from a scenario into typed objects.
func ParseResources(resources []k8sruntime.RawExtension) (*ParsedResources, error) {
	parsed := &ParsedResources{}

	for _, raw := range resources {
		// First unmarshal to get the kind
		var typeMeta metav1.TypeMeta
		if err := yaml.Unmarshal(raw.Raw, &typeMeta); err != nil {
			return nil, err
		}

		switch typeMeta.Kind {
		case "Ingress":
			var ingress networkingv1.Ingress
			if err := yaml.Unmarshal(raw.Raw, &ingress); err != nil {
				return nil, err
			}
			parsed.Ingresses = append(parsed.Ingresses, &ingress)
		case "Service":
			var svc corev1.Service
			if err := yaml.Unmarshal(raw.Raw, &svc); err != nil {
				return nil, err
			}
			parsed.Services = append(parsed.Services, &svc)
		}
	}

	return parsed, nil
}

// CreateFakeClient creates a fake Kubernetes client.
func CreateFakeClient() *fake.Clientset {
	return fake.NewClientset()
}

// PopulateResources creates the resources in the fake client using the API.
// This must be called BEFORE creating sources so the informers can see the resources.
func PopulateResources(ctx context.Context, client *fake.Clientset, resources *ParsedResources) error {
	for _, ing := range resources.Ingresses {
		created, err := client.NetworkingV1().Ingresses(ing.Namespace).Create(ctx, ing, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		// Update status separately since Create doesn't set status
		if len(ing.Status.LoadBalancer.Ingress) > 0 {
			created.Status = ing.Status
			_, err = client.NetworkingV1().Ingresses(ing.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	for _, svc := range resources.Services {
		created, err := client.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		// Update status separately since Create doesn't set status
		if len(svc.Status.LoadBalancer.Ingress) > 0 {
			created.Status = svc.Status
			_, err = client.CoreV1().Services(svc.Namespace).UpdateStatus(ctx, created, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// MockClientGenerator implements source.ClientGenerator for testing.
type MockClientGenerator struct {
	mock.Mock
	kubeClient kubernetes.Interface
}

func (m *MockClientGenerator) KubeClient() (kubernetes.Interface, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.kubeClient = args.Get(0).(kubernetes.Interface)
		return m.kubeClient, nil
	}
	return nil, args.Error(1)
}

func (m *MockClientGenerator) GatewayClient() (gateway.Interface, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(gateway.Interface), nil
}

func (m *MockClientGenerator) IstioClient() (istioclient.Interface, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(istioclient.Interface), nil
}

func (m *MockClientGenerator) DynamicKubernetesClient() (dynamic.Interface, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(dynamic.Interface), nil
}

func (m *MockClientGenerator) OpenShiftClient() (openshift.Interface, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(openshift.Interface), nil
}

// NewMockClientGenerator creates a MockClientGenerator that returns the provided fake client.
func NewMockClientGenerator(client *fake.Clientset) *MockClientGenerator {
	m := new(MockClientGenerator)
	m.On("KubeClient").Return(client, nil)
	return m
}

// CreateSourceConfig creates a source.Config for testing with the given sources and scenario config.
func CreateSourceConfig(sourceTypes []string, scenarioCfg ScenarioConfig) *source.Config {
	return &source.Config{
		ServiceTypeFilter:   []string{"LoadBalancer", "ExternalName"},
		LabelFilter:         labels.Everything(),
		DefaultTargets:      scenarioCfg.DefaultTargets,
		ForceDefaultTargets: scenarioCfg.ForceDefaultTargets,
	}
}

// CreateWrappedSource creates sources using source.BuildWithConfig and wraps them with wrappers.WrapSources.
func CreateWrappedSource(ctx context.Context, client *fake.Clientset, sourceTypes []string, scenarioCfg ScenarioConfig) (source.Source, error) {
	clientGen := NewMockClientGenerator(client)
	cfg := CreateSourceConfig(sourceTypes, scenarioCfg)

	var sources []source.Source
	for _, name := range sourceTypes {
		src, err := source.BuildWithConfig(ctx, name, clientGen, cfg)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}

	opts := wrappers.NewConfig(
		wrappers.WithDefaultTargets(cfg.DefaultTargets),
		wrappers.WithForceDefaultTargets(cfg.ForceDefaultTargets),
	)

	return wrappers.WrapSources(sources, opts)
}

// TODO: copied from source/wrappers/source_test.go - unify
func validateEndpoints(t *testing.T, endpoints, expected []*endpoint.Endpoint) {
	t.Helper()

	if len(endpoints) != len(expected) {
		t.Fatalf("expected %d endpoints, got %d", len(expected), len(endpoints))
	}

	// Make sure endpoints are sorted - validateEndpoint() depends on it.
	sortEndpoints(endpoints)
	sortEndpoints(expected)

	for i := range endpoints {
		validateEndpoint(t, endpoints[i], expected[i])
	}
}

// TODO: copied from source/wrappers/source_test.go - unify
func validateEndpoint(t *testing.T, endpoint, expected *endpoint.Endpoint) {
	t.Helper()

	if endpoint.DNSName != expected.DNSName {
		t.Errorf("DNSName expected %q, got %q", expected.DNSName, endpoint.DNSName)
	}

	if !endpoint.Targets.Same(expected.Targets) {
		t.Errorf("Targets expected %q, got %q", expected.Targets, endpoint.Targets)
	}

	if endpoint.RecordTTL != expected.RecordTTL {
		t.Errorf("RecordTTL expected %v, got %v", expected.RecordTTL, endpoint.RecordTTL)
	}

	// if a non-empty record type is expected, check that it matches.
	if endpoint.RecordType != expected.RecordType {
		t.Errorf("RecordType expected %q, got %q", expected.RecordType, endpoint.RecordType)
	}

	// if non-empty labels are expected, check that they match.
	if expected.Labels != nil && !reflect.DeepEqual(endpoint.Labels, expected.Labels) {
		t.Errorf("Labels expected %s, got %s", expected.Labels, endpoint.Labels)
	}

	if (len(expected.ProviderSpecific) != 0 || len(endpoint.ProviderSpecific) != 0) &&
		!reflect.DeepEqual(endpoint.ProviderSpecific, expected.ProviderSpecific) {
		t.Errorf("ProviderSpecific expected %s, got %s", expected.ProviderSpecific, endpoint.ProviderSpecific)
	}

	if endpoint.SetIdentifier != expected.SetIdentifier {
		t.Errorf("SetIdentifier expected %q, got %q", expected.SetIdentifier, endpoint.SetIdentifier)
	}
}

// TODO: copied from source/wrappers/source_test.go - unify
func sortEndpoints(endpoints []*endpoint.Endpoint) {
	for _, ep := range endpoints {
		sort.Strings(ep.Targets)
	}
	sort.Slice(endpoints, func(i, k int) bool {
		// Sort by DNSName, RecordType, and Targets
		ei, ek := endpoints[i], endpoints[k]
		if ei.DNSName != ek.DNSName {
			return ei.DNSName < ek.DNSName
		}
		if ei.RecordType != ek.RecordType {
			return ei.RecordType < ek.RecordType
		}
		// Targets are sorted ahead of time.
		for j, ti := range ei.Targets {
			if j >= len(ek.Targets) {
				return true
			}
			if tk := ek.Targets[j]; ti != tk {
				return ti < tk
			}
		}
		return false
	})
}
