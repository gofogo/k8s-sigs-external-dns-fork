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

package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/events"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/source"
)

// mockStatusUpdater implements both source.Source and dnsEndpointStatusUpdater interfaces
type mockStatusUpdater struct {
	endpoints      map[string]*apiv1alpha1.DNSEndpoint
	getError       error
	updateError    error
	getCalls       int
	updateCalls    int
	lastUpdatedGen int64
}

func newMockStatusUpdater() *mockStatusUpdater {
	return &mockStatusUpdater{
		endpoints: make(map[string]*apiv1alpha1.DNSEndpoint),
	}
}

func (m *mockStatusUpdater) Get(ctx context.Context, namespace, name string) (*apiv1alpha1.DNSEndpoint, error) {
	m.getCalls++
	if m.getError != nil {
		return nil, m.getError
	}
	key := namespace + "/" + name
	ep, ok := m.endpoints[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return ep, nil
}

func (m *mockStatusUpdater) UpdateStatus(ctx context.Context, dnsEndpoint *apiv1alpha1.DNSEndpoint) (*apiv1alpha1.DNSEndpoint, error) {
	m.updateCalls++
	if m.updateError != nil {
		return nil, m.updateError
	}
	key := dnsEndpoint.Namespace + "/" + dnsEndpoint.Name
	m.endpoints[key] = dnsEndpoint
	m.lastUpdatedGen = dnsEndpoint.Status.ObservedGeneration
	return dnsEndpoint, nil
}

// Implement source.Source interface
func (m *mockStatusUpdater) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

func (m *mockStatusUpdater) AddEventHandler(ctx context.Context, handler func()) {}

// Ensure mockStatusUpdater implements the interfaces
var _ dnsEndpointStatusUpdater = (*mockStatusUpdater)(nil)
var _ source.Source = (*mockStatusUpdater)(nil)

// mockSource is a basic source that doesn't implement status updates
type mockSource struct{}

func (m *mockSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	return nil, nil
}

func (m *mockSource) AddEventHandler(ctx context.Context, handler func()) {}

func TestUpdateDNSEndpointStatus_FeatureDisabled(t *testing.T) {
	mock := newMockStatusUpdater()
	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: false, // Feature disabled
	}

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{
			{
				DNSName: "test.example.com",
			},
		},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")

	// Should not call any methods when feature is disabled
	assert.Equal(t, 0, mock.getCalls, "Get should not be called when feature is disabled")
	assert.Equal(t, 0, mock.updateCalls, "UpdateStatus should not be called when feature is disabled")
}

func TestUpdateDNSEndpointStatus_SourceDoesNotImplementInterface(t *testing.T) {
	mockSrc := &mockSource{}
	ctrl := &Controller{
		Source:                  mockSrc,
		UpdateDNSEndpointStatus: true,
	}

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{
			{
				DNSName: "test.example.com",
			},
		},
	}

	// Should not panic when source doesn't implement interface
	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")
	// No assertions needed - just verifying it doesn't panic
}

func TestUpdateDNSEndpointStatus_NoRefObjects(t *testing.T) {
	mock := newMockStatusUpdater()
	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{
			{
				DNSName: "test.example.com",
				// No RefObject attached
			},
		},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")

	// Should not call any methods when no RefObjects are present
	assert.Equal(t, 0, mock.getCalls, "Get should not be called when no RefObjects")
	assert.Equal(t, 0, mock.updateCalls, "UpdateStatus should not be called when no RefObjects")
}

func TestUpdateDNSEndpointStatus_NonDNSEndpointRefObject(t *testing.T) {
	mock := newMockStatusUpdater()
	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ep := endpoint.NewEndpoint("test.example.com", endpoint.RecordTypeA, "1.2.3.4")
	// Attach a non-DNSEndpoint reference (e.g., Service)
	ep.WithRefObject(&events.ObjectReference{
		Kind:      "Service",
		Namespace: "default",
		Name:      "my-service",
		UID:       types.UID("service-uid"),
	})

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")

	// Should not call any methods when RefObject is not DNSEndpoint
	assert.Equal(t, 0, mock.getCalls, "Get should not be called for non-DNSEndpoint RefObjects")
	assert.Equal(t, 0, mock.updateCalls, "UpdateStatus should not be called for non-DNSEndpoint RefObjects")
}

func TestUpdateDNSEndpointStatus_SuccessfulSync(t *testing.T) {
	mock := newMockStatusUpdater()

	// Create a DNSEndpoint
	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 5,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ep := endpoint.NewEndpoint("test.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "Successfully synced DNS records")

	assert.Equal(t, 1, mock.getCalls, "Get should be called once")
	assert.Equal(t, 1, mock.updateCalls, "UpdateStatus should be called once")
	assert.Equal(t, int64(5), mock.lastUpdatedGen, "ObservedGeneration should be updated")

	// Check the updated status
	updated := mock.endpoints["default/test-endpoint"]
	assert.Equal(t, int64(5), updated.Status.ObservedGeneration)
	assert.NotNil(t, updated.Status.LastSyncTime)
	assert.True(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointReady))
	assert.True(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointSynced))

	readyCond := apiv1alpha1.GetCondition(&updated.Status, apiv1alpha1.DNSEndpointReady)
	assert.NotNil(t, readyCond)
	assert.Equal(t, apiv1alpha1.ReasonSyncSuccessful, readyCond.Reason)
	assert.Equal(t, "Successfully synced DNS records", readyCond.Message)
}

func TestUpdateDNSEndpointStatus_FailedSync(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 3,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ep := endpoint.NewEndpoint("test.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, false, "Failed to sync: connection timeout")

	assert.Equal(t, 1, mock.getCalls)
	assert.Equal(t, 1, mock.updateCalls)

	// Check the updated status
	updated := mock.endpoints["default/test-endpoint"]
	assert.Equal(t, int64(3), updated.Status.ObservedGeneration)
	assert.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointReady))
	assert.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointSynced))

	readyCond := apiv1alpha1.GetCondition(&updated.Status, apiv1alpha1.DNSEndpointReady)
	assert.NotNil(t, readyCond)
	assert.Equal(t, apiv1alpha1.ReasonFailed, readyCond.Reason)
	assert.Equal(t, "Failed to sync: connection timeout", readyCond.Message)

	syncedCond := apiv1alpha1.GetCondition(&updated.Status, apiv1alpha1.DNSEndpointSynced)
	assert.NotNil(t, syncedCond)
	assert.Equal(t, apiv1alpha1.ReasonSyncFailed, syncedCond.Reason)
}

func TestUpdateDNSEndpointStatus_MultipleDNSEndpoints(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint1 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "endpoint1",
			Namespace:  "default",
			UID:        types.UID("uid1"),
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	dnsEndpoint2 := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "endpoint2",
			Namespace:  "default",
			UID:        types.UID("uid2"),
			Generation: 2,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}

	mock.endpoints["default/endpoint1"] = dnsEndpoint1
	mock.endpoints["default/endpoint2"] = dnsEndpoint2

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ep1 := endpoint.NewEndpoint("test1.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep1.WithRefObject(events.NewObjectReference(dnsEndpoint1, "crd"))

	ep2 := endpoint.NewEndpoint("test2.example.com", endpoint.RecordTypeA, "1.2.3.5")
	ep2.WithRefObject(events.NewObjectReference(dnsEndpoint2, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep1, ep2},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "Successfully synced")

	// Should update both DNSEndpoints
	assert.Equal(t, 2, mock.getCalls, "Should call Get for both DNSEndpoints")
	assert.Equal(t, 2, mock.updateCalls, "Should call UpdateStatus for both DNSEndpoints")

	// Verify both were updated
	updated1 := mock.endpoints["default/endpoint1"]
	assert.True(t, apiv1alpha1.IsConditionTrue(&updated1.Status, apiv1alpha1.DNSEndpointReady))

	updated2 := mock.endpoints["default/endpoint2"]
	assert.True(t, apiv1alpha1.IsConditionTrue(&updated2.Status, apiv1alpha1.DNSEndpointReady))
}

func TestUpdateDNSEndpointStatus_DuplicateReferences(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	// Create multiple endpoints referencing the same DNSEndpoint
	ep1 := endpoint.NewEndpoint("test1.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep1.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	ep2 := endpoint.NewEndpoint("test2.example.com", endpoint.RecordTypeA, "1.2.3.5")
	ep2.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep1, ep2},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "Successfully synced")

	// Should only update once (deduplication)
	assert.Equal(t, 1, mock.getCalls, "Should call Get only once for the same DNSEndpoint")
	assert.Equal(t, 1, mock.updateCalls, "Should call UpdateStatus only once for the same DNSEndpoint")
}

func TestUpdateDNSEndpointStatus_GetError(t *testing.T) {
	mock := newMockStatusUpdater()
	mock.getError = fmt.Errorf("failed to get endpoint")

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}

	ep := endpoint.NewEndpoint("test.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	// Should not panic on error
	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")

	assert.Equal(t, 1, mock.getCalls, "Should attempt to call Get")
	assert.Equal(t, 0, mock.updateCalls, "Should not call UpdateStatus when Get fails")
}

func TestUpdateDNSEndpointStatus_UpdateError(t *testing.T) {
	mock := newMockStatusUpdater()
	mock.updateError = fmt.Errorf("failed to update status")

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ep := endpoint.NewEndpoint("test.example.com", endpoint.RecordTypeA, "1.2.3.4")
	ep.WithRefObject(events.NewObjectReference(dnsEndpoint, "crd"))

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	// Should not panic on error
	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "test message")

	assert.Equal(t, 1, mock.getCalls, "Should call Get")
	assert.Equal(t, 1, mock.updateCalls, "Should attempt to call UpdateStatus")
}

func TestUpdateDNSEndpointStatus_AllChangeTypes(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			UID:        types.UID("test-uid"),
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DNSEndpoint",
			APIVersion: "externaldns.k8s.io/v1alpha1",
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	ctrl := &Controller{
		Source:                  mock,
		UpdateDNSEndpointStatus: true,
	}

	ref := events.NewObjectReference(dnsEndpoint, "crd")

	epCreate := endpoint.NewEndpoint("create.example.com", endpoint.RecordTypeA, "1.2.3.4")
	epCreate.WithRefObject(ref)

	epUpdateOld := endpoint.NewEndpoint("update.example.com", endpoint.RecordTypeA, "1.2.3.5")
	epUpdateOld.WithRefObject(ref)

	epUpdateNew := endpoint.NewEndpoint("update.example.com", endpoint.RecordTypeA, "1.2.3.6")
	epUpdateNew.WithRefObject(ref)

	epDelete := endpoint.NewEndpoint("delete.example.com", endpoint.RecordTypeA, "1.2.3.7")
	epDelete.WithRefObject(ref)

	changes := &plan.Changes{
		Create:    []*endpoint.Endpoint{epCreate},
		UpdateOld: []*endpoint.Endpoint{epUpdateOld},
		UpdateNew: []*endpoint.Endpoint{epUpdateNew},
		Delete:    []*endpoint.Endpoint{epDelete},
	}

	ctrl.updateDNSEndpointStatus(context.Background(), changes, true, "Successfully synced")

	// Should only update once despite multiple endpoints (deduplication)
	assert.Equal(t, 1, mock.getCalls, "Should call Get once")
	assert.Equal(t, 1, mock.updateCalls, "Should call UpdateStatus once")
}

func TestUpdateSingleDNSEndpointStatus_Success(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			Generation: 10,
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	err := updateSingleDNSEndpointStatus(
		context.Background(),
		mock,
		"default",
		"test-endpoint",
		true,
		"Successfully synced",
	)

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.getCalls)
	assert.Equal(t, 1, mock.updateCalls)
	assert.Equal(t, int64(10), mock.lastUpdatedGen)
}

func TestUpdateSingleDNSEndpointStatus_Failure(t *testing.T) {
	mock := newMockStatusUpdater()

	dnsEndpoint := &apiv1alpha1.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-endpoint",
			Namespace:  "default",
			Generation: 5,
		},
	}
	mock.endpoints["default/test-endpoint"] = dnsEndpoint

	err := updateSingleDNSEndpointStatus(
		context.Background(),
		mock,
		"default",
		"test-endpoint",
		false,
		"Failed to sync",
	)

	assert.NoError(t, err)

	updated := mock.endpoints["default/test-endpoint"]
	assert.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointReady))
	assert.False(t, apiv1alpha1.IsConditionTrue(&updated.Status, apiv1alpha1.DNSEndpointSynced))
}
