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
			conditionType:      string(DNSEndpointProgrammed),
			status:             metav1.ConditionTrue,
			reason:             string(ReasonProgrammed),
			message:            "Successfully programmed",
			observedGeneration: 1,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, string(DNSEndpointProgrammed))
				assert.NotNil(t, cond, "condition should be found")
				assert.Equal(t, metav1.ConditionTrue, cond.Status)
				assert.Equal(t, string(ReasonProgrammed), cond.Reason)
				assert.Equal(t, "Successfully programmed", cond.Message)
				assert.Equal(t, int64(1), cond.ObservedGeneration)
			},
		},
		{
			name: "update existing condition with same status preserves LastTransitionTime",
			existingConditions: []metav1.Condition{
				{
					Type:               string(DNSEndpointProgrammed),
					Status:             metav1.ConditionTrue,
					Reason:             string(ReasonProgrammed),
					Message:            "Old message",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			conditionType:      string(DNSEndpointProgrammed),
			status:             metav1.ConditionTrue,
			reason:             string(ReasonProgrammed),
			message:            "New message",
			observedGeneration: 2,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, string(DNSEndpointProgrammed))
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
					Type:               string(DNSEndpointProgrammed),
					Status:             metav1.ConditionTrue,
					Reason:             string(ReasonProgrammed),
					Message:            "Was successful",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			conditionType:      string(DNSEndpointProgrammed),
			status:             metav1.ConditionFalse,
			reason:             string(ReasonInvalid),
			message:            "Now failed",
			observedGeneration: 2,
			wantConditionCount: 1,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				cond := GetCondition(status, string(DNSEndpointProgrammed))
				assert.NotNil(t, cond, "condition should be found")
				assert.Equal(t, metav1.ConditionFalse, cond.Status)
				assert.Equal(t, string(ReasonInvalid), cond.Reason)
				// LastTransitionTime should be recent (updated)
				assert.Less(t, time.Since(cond.LastTransitionTime.Time), 5*time.Second, "LastTransitionTime should be updated (recent)")
			},
		},
		{
			name: "add second condition preserves first",
			existingConditions: []metav1.Condition{
				{
					Type:               string(DNSEndpointAccepted),
					Status:             metav1.ConditionTrue,
					Reason:             string(ReasonAccepted),
					Message:            "Accepted",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.Now(),
				},
			},
			conditionType:      string(DNSEndpointProgrammed),
			status:             metav1.ConditionTrue,
			reason:             string(ReasonProgrammed),
			message:            "Programmed",
			observedGeneration: 1,
			wantConditionCount: 2,
			validateFunc: func(t *testing.T, status *DNSEndpointStatus) {
				acceptedCond := GetCondition(status, string(DNSEndpointAccepted))
				assert.NotNil(t, acceptedCond, "Accepted condition should be found")
				assert.Equal(t, string(ReasonAccepted), acceptedCond.Reason, "Accepted condition should not be modified")

				programmedCond := GetCondition(status, string(DNSEndpointProgrammed))
				assert.NotNil(t, programmedCond, "Programmed condition should be found")
				assert.Equal(t, string(ReasonProgrammed), programmedCond.Reason)
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
			conditions:    []metav1.Condition{{Type: string(DNSEndpointProgrammed), Reason: string(ReasonProgrammed)}},
			conditionType: string(DNSEndpointProgrammed),
			wantNil:       false,
			wantReason:    string(ReasonProgrammed),
		},
		{
			name:          "get non-existent condition",
			conditions:    []metav1.Condition{{Type: string(DNSEndpointAccepted), Reason: string(ReasonAccepted)}},
			conditionType: string(DNSEndpointProgrammed),
			wantNil:       true,
		},
		{
			name:          "get from empty conditions",
			conditions:    []metav1.Condition{},
			conditionType: string(DNSEndpointProgrammed),
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
				{Type: string(DNSEndpointProgrammed), Status: metav1.ConditionTrue},
			},
			conditionType: string(DNSEndpointProgrammed),
			want:          true,
		},
		{
			name: "condition exists but is false",
			conditions: []metav1.Condition{
				{Type: string(DNSEndpointProgrammed), Status: metav1.ConditionFalse},
			},
			conditionType: string(DNSEndpointProgrammed),
			want:          false,
		},
		{
			name: "condition exists but is unknown",
			conditions: []metav1.Condition{
				{Type: string(DNSEndpointProgrammed), Status: metav1.ConditionUnknown},
			},
			conditionType: string(DNSEndpointProgrammed),
			want:          false,
		},
		{
			name:          "condition does not exist",
			conditions:    []metav1.Condition{},
			conditionType: string(DNSEndpointProgrammed),
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

func TestSetAccepted(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "DNSEndpoint accepted by controller with owner ID: test"
	observedGeneration := int64(1)

	SetAccepted(status, message, observedGeneration)

	// Check Accepted condition
	acceptedCond := GetCondition(status, string(DNSEndpointAccepted))
	assert.NotNil(t, acceptedCond, "Accepted condition should be set")
	assert.Equal(t, metav1.ConditionUnknown, acceptedCond.Status, "Should be Unknown when first accepted")
	assert.Equal(t, string(ReasonAccepted), acceptedCond.Reason)
	assert.Equal(t, message, acceptedCond.Message)
	assert.Equal(t, observedGeneration, acceptedCond.ObservedGeneration)
}

func TestSetSyncSuccess(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "Successfully synced 5 DNS records"
	observedGeneration := int64(3)

	SetSyncSuccess(status, message, observedGeneration)

	// Check Accepted condition
	acceptedCond := GetCondition(status, string(DNSEndpointAccepted))
	assert.NotNil(t, acceptedCond, "Accepted condition should be set")
	assert.Equal(t, metav1.ConditionTrue, acceptedCond.Status)
	assert.Equal(t, string(ReasonAccepted), acceptedCond.Reason)
	assert.Equal(t, observedGeneration, acceptedCond.ObservedGeneration)

	// Check Programmed condition
	programmedCond := GetCondition(status, string(DNSEndpointProgrammed))
	assert.NotNil(t, programmedCond, "Programmed condition should be set")
	assert.Equal(t, metav1.ConditionTrue, programmedCond.Status)
	assert.Equal(t, string(ReasonProgrammed), programmedCond.Reason)
	assert.Equal(t, message, programmedCond.Message)
	assert.Equal(t, observedGeneration, programmedCond.ObservedGeneration)
}

func TestSetSyncFailed(t *testing.T) {
	status := &DNSEndpointStatus{}
	message := "Failed to sync: connection timeout"
	observedGeneration := int64(2)

	SetSyncFailed(status, message, observedGeneration)

	// Check Accepted condition
	acceptedCond := GetCondition(status, string(DNSEndpointAccepted))
	assert.NotNil(t, acceptedCond, "Accepted condition should be set")
	assert.Equal(t, metav1.ConditionTrue, acceptedCond.Status)
	assert.Equal(t, string(ReasonAccepted), acceptedCond.Reason)

	// Check Programmed condition
	programmedCond := GetCondition(status, string(DNSEndpointProgrammed))
	assert.NotNil(t, programmedCond, "Programmed condition should be set")
	assert.Equal(t, metav1.ConditionFalse, programmedCond.Status)
	assert.Equal(t, string(ReasonInvalid), programmedCond.Reason)
	assert.Equal(t, message, programmedCond.Message)
}
