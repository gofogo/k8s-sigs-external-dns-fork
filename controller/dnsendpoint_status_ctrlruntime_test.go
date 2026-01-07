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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
)

// TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_Success tests successful status update
func TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_Success(t *testing.T) {
	// Create scheme with DNSEndpoint types
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create test DNSEndpoint
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

	// Create fake client with status subresource support
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dnsEndpoint).
		WithStatusSubresource(dnsEndpoint).
		Build()

	// Create status manager
	manager := NewDNSEndpointStatusManagerCtrlRuntime(fakeClient)

	// Update status with success
	err = manager.UpdateStatus(context.Background(), "default", "test-endpoint", true, "Successfully synced")
	require.NoError(t, err)

	// Verify status was updated
	updated := &apiv1alpha1.DNSEndpoint{}
	key := client.ObjectKey{Namespace: "default", Name: "test-endpoint"}
	err = fakeClient.Get(context.Background(), key, updated)
	require.NoError(t, err)

	// Verify Programmed condition is True
	require.True(t, apiv1alpha1.IsConditionTrue(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed)))

	// Verify condition details
	programmedCond := apiv1alpha1.GetCondition(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed))
	require.NotNil(t, programmedCond)
	require.Equal(t, metav1.ConditionTrue, programmedCond.Status)
	require.Equal(t, string(apiv1alpha1.ReasonProgrammed), programmedCond.Reason)
	require.Equal(t, "Successfully synced", programmedCond.Message)
	require.Equal(t, int64(1), programmedCond.ObservedGeneration)
}

// TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_Failure tests failed status update
func TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_Failure(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			Generation: 2,
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dnsEndpoint).
		WithStatusSubresource(dnsEndpoint).
		Build()

	manager := NewDNSEndpointStatusManagerCtrlRuntime(fakeClient)

	// Update status with failure
	err = manager.UpdateStatus(context.Background(), "default", "test-endpoint", false, "Failed to sync: DNS error")
	require.NoError(t, err)

	// Verify status was updated
	updated := &apiv1alpha1.DNSEndpoint{}
	key := client.ObjectKey{Namespace: "default", Name: "test-endpoint"}
	err = fakeClient.Get(context.Background(), key, updated)
	require.NoError(t, err)

	// Verify Programmed condition is False
	require.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed)))

	// Verify condition details
	programmedCond := apiv1alpha1.GetCondition(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed))
	require.NotNil(t, programmedCond)
	require.Equal(t, metav1.ConditionFalse, programmedCond.Status)
	require.Contains(t, programmedCond.Message, "Failed to sync: DNS error")
	require.Equal(t, int64(2), programmedCond.ObservedGeneration)
}

// TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_NotFound tests error handling for missing resource
func TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	err := apiv1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	manager := NewDNSEndpointStatusManagerCtrlRuntime(fakeClient)

	// Try to update non-existent endpoint
	err = manager.UpdateStatus(context.Background(), "default", "nonexistent", true, "Test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get DNSEndpoint")
}

// TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_MultipleUpdates tests multiple status updates
func TestDNSEndpointStatusManagerCtrlRuntime_UpdateStatus_MultipleUpdates(t *testing.T) {
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

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(dnsEndpoint).
		WithStatusSubresource(dnsEndpoint).
		Build()

	manager := NewDNSEndpointStatusManagerCtrlRuntime(fakeClient)

	// First update: success
	err = manager.UpdateStatus(context.Background(), "default", "test-endpoint", true, "First sync")
	require.NoError(t, err)

	// Verify first update
	updated := &apiv1alpha1.DNSEndpoint{}
	key := client.ObjectKey{Namespace: "default", Name: "test-endpoint"}
	err = fakeClient.Get(context.Background(), key, updated)
	require.NoError(t, err)
	require.True(t, apiv1alpha1.IsConditionTrue(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed)))

	// Second update: failure
	err = manager.UpdateStatus(context.Background(), "default", "test-endpoint", false, "Second sync failed")
	require.NoError(t, err)

	// Verify second update overwrote first
	err = fakeClient.Get(context.Background(), key, updated)
	require.NoError(t, err)
	require.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed)))

	programmedCond := apiv1alpha1.GetCondition(&updated.Status, string(apiv1alpha1.DNSEndpointProgrammed))
	require.NotNil(t, programmedCond)
	require.Equal(t, "Second sync failed", programmedCond.Message)
}
