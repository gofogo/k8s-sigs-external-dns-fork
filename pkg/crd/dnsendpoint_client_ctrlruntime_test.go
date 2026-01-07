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

package crd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

// TestCtrlRuntimeDNSEndpointClient_Get tests the Get method
func TestCtrlRuntimeDNSEndpointClient_Get(t *testing.T) {
	// Create scheme with DNSEndpoint types
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create test DNSEndpoint
	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-endpoint",
			Namespace: "default",
		},
		Spec: apiv1alpha1.DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{
				{
					DNSName:    "test.example.com",
					RecordType: "A",
					Targets:    []string{"1.2.3.4"},
				},
			},
		},
	}

	// Create fake client with test object
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dnsEndpoint).
		Build()

	// Create DNSEndpointClient
	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "default")

	// Test Get
	result, err := client.Get(context.Background(), "default", "test-endpoint")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "test-endpoint", result.Name)
	require.Equal(t, "default", result.Namespace)
	require.Len(t, result.Spec.Endpoints, 1)
	require.Equal(t, "test.example.com", result.Spec.Endpoints[0].DNSName)
}

// TestCtrlRuntimeDNSEndpointClient_Get_NotFound tests Get with non-existent resource
func TestCtrlRuntimeDNSEndpointClient_Get_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "default")

	// Get non-existent endpoint
	_, err = client.Get(context.Background(), "default", "nonexistent")
	require.Error(t, err)
}

// TestCtrlRuntimeDNSEndpointClient_List tests the List method
func TestCtrlRuntimeDNSEndpointClient_List(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create multiple test DNSEndpoints
	endpoint1 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint-1",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}
	endpoint2 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint-2",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}
	endpoint3 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint-3",
			Namespace: "other",
			Labels: map[string]string{
				"app": "other",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(endpoint1, endpoint2, endpoint3).
		Build()

	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "default")

	// Test List without filters
	result, err := client.List(context.Background(), &metav1.ListOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Should return only endpoints from "default" namespace
	require.Len(t, result.Items, 2)

	// Test List with label selector
	result, err = client.List(context.Background(), &metav1.ListOptions{
		LabelSelector: "app=test",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 2)
}

// TestCtrlRuntimeDNSEndpointClient_List_AllNamespaces tests List across all namespaces
func TestCtrlRuntimeDNSEndpointClient_List_AllNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	endpoint1 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint-1",
			Namespace: "default",
		},
	}
	endpoint2 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint-2",
			Namespace: "other",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(endpoint1, endpoint2).
		Build()

	// Create client with empty namespace (all namespaces)
	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "")

	result, err := client.List(context.Background(), &metav1.ListOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Should return endpoints from all namespaces
	require.Len(t, result.Items, 2)
}

// TestCtrlRuntimeDNSEndpointClient_UpdateStatus tests the UpdateStatus method
func TestCtrlRuntimeDNSEndpointClient_UpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: apiv1alpha1.DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{
				{
					DNSName:    "test.example.com",
					RecordType: "A",
					Targets:    []string{"1.2.3.4"},
				},
			},
		},
	}

	// Important: WithStatusSubresource is needed to enable status updates in fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dnsEndpoint).
		WithStatusSubresource(dnsEndpoint).
		Build()

	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "default")

	// Get the endpoint
	endpoint, err := client.Get(context.Background(), "default", "test-endpoint")
	require.NoError(t, err)

	// Update status using helper function
	apiv1alpha1.SetSyncSuccess(&endpoint.Status, "Test success", endpoint.Generation)

	// Update status via client
	updated, err := client.UpdateStatus(context.Background(), endpoint)
	require.NoError(t, err)
	require.NotNil(t, updated)

	// Verify status was updated
	result, err := client.Get(context.Background(), "default", "test-endpoint")
	require.NoError(t, err)
	require.True(t, apiv1alpha1.IsConditionTrue(&result.Status, string(apiv1alpha1.DNSEndpointProgrammed)))

	// Verify ObservedGeneration is set on the condition
	programmedCond := apiv1alpha1.GetCondition(&result.Status, string(apiv1alpha1.DNSEndpointProgrammed))
	require.NotNil(t, programmedCond)
	require.Equal(t, int64(1), programmedCond.ObservedGeneration)
}

// TestCtrlRuntimeDNSEndpointClient_Watch tests the Watch method
func TestCtrlRuntimeDNSEndpointClient_Watch(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	client := NewDNSEndpointClientCtrlRuntime(fakeClient, "default")

	// Watch should return error (not implemented)
	_, err = client.Watch(context.Background(), &metav1.ListOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Watch not supported")
}
