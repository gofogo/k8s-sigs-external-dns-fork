/*
Copyright 2026 The Kubernetes Authors.

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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/external-dns/source/annotations"
)

func TestParseResources(t *testing.T) {
	scenariosPath := filepath.Join(GetScenariosPath(), "integration_test.yaml")
	scenarios, err := LoadScenarios(scenariosPath)
	require.NoError(t, err, "failed to load scenarios")
	require.NotEmpty(t, scenarios.Scenarios, "no scenarios found")

	// Test first scenario
	scenario := scenarios.Scenarios[0]
	t.Logf("Scenario: %s", scenario.Name)
	t.Logf("Sources: %v", scenario.Config.Sources)
	t.Logf("Resources count: %d", len(scenario.Resources))

	resources, err := ParseResources(scenario.Resources)
	require.NoError(t, err, "failed to parse resources")

	t.Logf("Parsed ingresses: %d", len(resources.Ingresses))
	for _, ing := range resources.Ingresses {
		t.Logf("  Ingress: %s/%s", ing.Namespace, ing.Name)
		t.Logf("  Annotations: %v", ing.Annotations)
		t.Logf("  Status: %+v", ing.Status)
	}
}

func TestSourceDirect(t *testing.T) {
	ctx := context.Background()

	// Create fake client
	client := CreateFakeClient()

	// Create ingress directly using API
	ing, err := client.NetworkingV1().Ingresses("default").Create(ctx, &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": "test.example.com",
			},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{IP: "1.2.3.4"},
				},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Logf("Created ingress: %s/%s", ing.Namespace, ing.Name)
	t.Logf("Ingress status after create: %+v", ing.Status)

	// Update status
	ing.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}}
	updated, err := client.NetworkingV1().Ingresses("default").UpdateStatus(ctx, ing, metav1.UpdateOptions{})
	require.NoError(t, err)
	t.Logf("Ingress status after update: %+v", updated.Status)

	// List to verify
	list, err := client.NetworkingV1().Ingresses("default").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	t.Logf("Listed %d ingresses", len(list.Items))
	for _, i := range list.Items {
		t.Logf("  - %s/%s annotations=%v status=%+v", i.Namespace, i.Name, i.Annotations, i.Status)
	}

	// Now create wrapped source
	wrappedSource, err := CreateWrappedSource(ctx, client, ScenarioConfig{Sources: []string{"ingress"}})
	require.NoError(t, err)
	t.Logf("Created wrapped source")

	// Get endpoints
	endpoints, err := wrappedSource.Endpoints(ctx)
	require.NoError(t, err)
	t.Logf("Got %d endpoints", len(endpoints))
	for _, ep := range endpoints {
		t.Logf("  - %s %s -> %v", ep.DNSName, ep.RecordType, ep.Targets)
	}
}

func TestSourceIntegration(t *testing.T) {
	// TODO: this is required to ensure annotation parsing works as expected. Ideally, should be set differently.
	annotations.SetAnnotationPrefix(annotations.DefaultAnnotationPrefix)
	scenarios, err := LoadScenarios(filepath.Join(GetScenariosPath(), "integration_test.yaml"))
	require.NoError(t, err, "failed to load scenarios")
	require.NotEmpty(t, scenarios.Scenarios, "no scenarios found")

	for _, scenario := range scenarios.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			client, err := LoadResources(ctx, scenario)
			require.NoError(t, err, "failed to populate resources")

			// Create wrapped source
			wrappedSource, err := CreateWrappedSource(ctx, client, scenario.Config)
			require.NoError(t, err, "failed to create wrapped source")

			// Get endpoints
			endpoints, err := wrappedSource.Endpoints(ctx)
			require.NoError(t, err)
			validateEndpoints(t, endpoints, scenario.Expected)
		})
	}
}
