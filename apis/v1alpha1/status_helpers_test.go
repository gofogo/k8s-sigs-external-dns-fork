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

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name               string
		existingConditions []metav1.Condition
		conditionType      string
		status             metav1.ConditionStatus
		reason             string
		message            string
		observedGeneration int64
		wantConditionCount int
		validateFunc       func(*testing.T, *DNSEndpointStatus)
	}{
		{
			name:               "add new condition to empty list",
			existingConditions: []metav1.Condition{},
			conditionType:      DNSEndpointReady,
			status:             metav1.ConditionTrue,
			reason:             ReasonSyncSuccessful,
			message:            "Successfully synced",
			observedGeneration: 1,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, DNSEndpointReady)
				assert.NotNil(t, cond, "condition should be found")
				assert.Equal(t, metav1.ConditionTrue, cond.Status)
				assert.Equal(t, ReasonSyncSuccessful, cond.Reason)
				assert.Equal(t, "Successfully synced", cond.Message)
				assert.Equal(t, int64(1), cond.ObservedGeneration)
			},
		},
		{
			name: "update existing condition with same status preserves LastTransitionTime",
			existingConditions: []metav1.Condition{
				{
					Type:               DNSEndpointReady,
					Status:             metav1.ConditionTrue,
					Reason:             ReasonSyncSuccessful,
					Message:            "Old message",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			conditionType:      DNSEndpointReady,
			status:             metav1.ConditionTrue,
			reason:             ReasonSyncSuccessful,
			message:            "New message",
			observedGeneration: 2,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, DNSEndpointReady)
				assert.NotNil(t, cond, "condition should be found")
				assert.Equal(t, "New message", cond.Message, "message should be updated")
				assert.Equal(t, int64(2), cond.ObservedGeneration, "observedGeneration should be updated")
				// LastTransitionTime should be old (preserved)
				assert.Greater(t, time.Since(cond.LastTransitionTime.Time), 30*time.Minute, "LastTransitionTime should be preserved (old)")
			},
		},
		{
			name: "update existing condition with different status updates LastTransitionTime",
			existingConditions: []metav1.Condition{
				{
					Type:               DNSEndpointReady,
					Status:             metav1.ConditionTrue,
					Reason:             ReasonSyncSuccessful,
					Message:            "Was successful",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			conditionType:      DNSEndpointReady,
			status:             metav1.ConditionFalse,
			reason:             ReasonFailed,
			message:            "Now failed",
			observedGeneration: 2,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, DNSEndpointReady)
				assert.NotNil(t, cond, "condition should be found")
				assert.Equal(t, metav1.ConditionFalse, cond.Status)
				assert.Equal(t, ReasonFailed, cond.Reason)
				// LastTransitionTime should be recent (updated)
				assert.Less(t, time.Since(cond.LastTransitionTime.Time), 5*time.Second, "LastTransitionTime should be updated (recent)")
			},
		},
		{
			name: "add second condition preserves first",
			existingConditions: []metav1.Condition{
				{
					Type:               DNSEndpointSynced,
					Status:             metav1.ConditionTrue,
					Reason:             ReasonReconciling,
					Message:            "Reconciling",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.Now(),
				},
			},
			conditionType:      DNSEndpointReady,
			status:             metav1.ConditionTrue,
			reason:             ReasonSyncSuccessful,
			message:            "Ready",
			observedGeneration: 1,
			wantConditionCount: 2,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				syncedCond := GetCondition(status, DNSEndpointSynced)
				assert.NotNil(t, syncedCond, "Synced condition should be found")
				assert.Equal(t, ReasonReconciling, syncedCond.Reason, "Synced condition should not be modified")

				readyCond := GetCondition(status, DNSEndpointReady)
				assert.NotNil(t, readyCond, "Ready condition should be found")
				assert.Equal(t, ReasonSyncSuccessful, readyCond.Reason)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &DNSEndpointStatus{
				Conditions: tt.existingConditions,
			}

			SetCondition(status, tt.conditionType, tt.status, tt.reason, tt.message, tt.observedGeneration)

			assert.Len(t, status.Conditions, tt.wantConditionCount)

			if tt.validateFunc != nil {
				tt.validateFunc(t, status)
			}
		})
	}
}

func TestGetCondition(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		conditionType string
		wantNil       bool
		wantReason    string
	}{
		{
			name:          "get existing condition",
			conditions:    []metav1.Condition{{Type: DNSEndpointReady, Reason: ReasonSyncSuccessful}},
			conditionType: DNSEndpointReady,
			wantNil:       false,
			wantReason:    ReasonSyncSuccessful,
		},
		{
			name:          "get non-existent condition",
			conditions:    []metav1.Condition{{Type: DNSEndpointSynced, Reason: ReasonReconciling}},
			conditionType: DNSEndpointReady,
			wantNil:       true,
		},
		{
			name:          "get from empty conditions",
			conditions:    []metav1.Condition{},
			conditionType: DNSEndpointReady,
			wantNil:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &DNSEndpointStatus{
				Conditions: tt.conditions,
			}

			cond := GetCondition(status, tt.conditionType)

			if tt.wantNil {
				assert.Nil(t, cond)
			} else {
				assert.NotNil(t, cond)
				assert.Equal(t, tt.wantReason, cond.Reason)
			}
		})
	}
}

func TestIsConditionTrue(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		conditionType string
		want          bool
	}{
		{
			name: "condition exists and is true",
			conditions: []metav1.Condition{
				{Type: DNSEndpointReady, Status: metav1.ConditionTrue},
			},
			conditionType: DNSEndpointReady,
			want:          true,
		},
		{
			name: "condition exists but is false",
			conditions: []metav1.Condition{
				{Type: DNSEndpointReady, Status: metav1.ConditionFalse},
			},
			conditionType: DNSEndpointReady,
			want:          false,
		},
		{
			name: "condition exists but is unknown",
			conditions: []metav1.Condition{
				{Type: DNSEndpointReady, Status: metav1.ConditionUnknown},
			},
			conditionType: DNSEndpointReady,
			want:          false,
		},
		{
			name:          "condition does not exist",
			conditions:    []metav1.Condition{},
			conditionType: DNSEndpointReady,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &DNSEndpointStatus{
				Conditions: tt.conditions,
			}

			got := IsConditionTrue(status, tt.conditionType)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetSyncSuccess(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "Successfully synced 5 DNS records"
	observedGeneration := int64(3)

	SetSyncSuccess(status, message, observedGeneration)

	// Check LastSyncTime was set
	assert.NotNil(t, status.LastSyncTime, "LastSyncTime should be set")
	assert.Less(t, time.Since(status.LastSyncTime.Time), 5*time.Second, "LastSyncTime should be recent")

	// Check Synced condition
	syncedCond := GetCondition(status, DNSEndpointSynced)
	assert.NotNil(t, syncedCond, "Synced condition should be set")
	assert.Equal(t, metav1.ConditionTrue, syncedCond.Status)
	assert.Equal(t, ReasonSyncSuccessful, syncedCond.Reason)
	assert.Equal(t, message, syncedCond.Message)
	assert.Equal(t, observedGeneration, syncedCond.ObservedGeneration)

	// Check Ready condition
	readyCond := GetCondition(status, DNSEndpointReady)
	assert.NotNil(t, readyCond, "Ready condition should be set")
	assert.Equal(t, metav1.ConditionTrue, readyCond.Status)
	assert.Equal(t, ReasonSyncSuccessful, readyCond.Reason)
}

func TestSetSyncFailed(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "Failed to sync: connection timeout"
	observedGeneration := int64(2)

	SetSyncFailed(status, message, observedGeneration)

	// Check Synced condition
	syncedCond := GetCondition(status, DNSEndpointSynced)
	assert.NotNil(t, syncedCond, "Synced condition should be set")
	assert.Equal(t, metav1.ConditionFalse, syncedCond.Status)
	assert.Equal(t, ReasonSyncFailed, syncedCond.Reason)

	// Check Ready condition
	readyCond := GetCondition(status, DNSEndpointReady)
	assert.NotNil(t, readyCond, "Ready condition should be set")
	assert.Equal(t, metav1.ConditionFalse, readyCond.Status)
	assert.Equal(t, ReasonFailed, readyCond.Reason)
}

func TestSetReconciling(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "Processing 3 endpoints"
	observedGeneration := int64(1)

	SetReconciling(status, message, observedGeneration)

	// Check Synced condition
	syncedCond := GetCondition(status, DNSEndpointSynced)
	assert.NotNil(t, syncedCond, "Synced condition should be set")
	assert.Equal(t, metav1.ConditionTrue, syncedCond.Status)
	assert.Equal(t, ReasonReconciling, syncedCond.Reason)
	assert.Equal(t, message, syncedCond.Message)

	// Ready condition should not be set by SetReconciling
	readyCond := GetCondition(status, DNSEndpointReady)
	assert.Nil(t, readyCond, "Ready condition should not be set by SetReconciling")
}
