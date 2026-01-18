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

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/external-dns/endpoint"
)

func TestSetCondition_AddsNewCondition(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Status:     DNSEndpointStatus{},
	}

	setCondition(input, "TestType", metav1.ConditionTrue, "TestReason", "Test message")

	assert.Len(t, input.Status.Conditions, 1)
	assert.Equal(t, "TestType", input.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, input.Status.Conditions[0].Status)
	assert.Equal(t, "TestReason", input.Status.Conditions[0].Reason)
	assert.Equal(t, "Test message", input.Status.Conditions[0].Message)
	assert.Equal(t, int64(1), input.Status.Conditions[0].ObservedGeneration)
	assert.NotNil(t, input.Status.LastStatusChange)
}

func TestSetCondition_UpdatesExistingCondition(t *testing.T) {
	originalTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: DNSEndpointStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "TestType",
					Status:             metav1.ConditionFalse,
					Reason:             "OldReason",
					Message:            "Old message",
					ObservedGeneration: 1,
					LastTransitionTime: originalTime,
				},
			},
		},
	}

	setCondition(input, "TestType", metav1.ConditionTrue, "NewReason", "New message")

	assert.Len(t, input.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionTrue, input.Status.Conditions[0].Status)
	assert.Equal(t, "NewReason", input.Status.Conditions[0].Reason)
	assert.Equal(t, "New message", input.Status.Conditions[0].Message)
	assert.Equal(t, int64(2), input.Status.Conditions[0].ObservedGeneration)
	// LastTransitionTime should be updated since status changed
	assert.NotEqual(t, originalTime, input.Status.Conditions[0].LastTransitionTime)
}

func TestSetCondition_PreservesLastTransitionTimeWhenStatusUnchanged(t *testing.T) {
	originalTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: DNSEndpointStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "TestType",
					Status:             metav1.ConditionTrue,
					Reason:             "OldReason",
					Message:            "Old message",
					ObservedGeneration: 1,
					LastTransitionTime: originalTime,
				},
			},
		},
	}

	setCondition(input, "TestType", metav1.ConditionTrue, "NewReason", "New message")

	assert.Len(t, input.Status.Conditions, 1)
	// LastTransitionTime should be preserved since status didn't change
	assert.Equal(t, originalTime, input.Status.Conditions[0].LastTransitionTime)
}

func TestSetCondition_AcceptedSetsRecordsFromZero(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}, {}}, // 3 endpoints
		},
		Status: DNSEndpointStatus{
			Records: "0/0",
		},
	}

	setCondition(input, string(DNSEndpointAccepted), metav1.ConditionUnknown, string(ReasonAccepted), "Accepted")

	assert.Equal(t, "0/3", input.Status.Records)
	assert.Equal(t, 0, input.Status.RecordsProvisioned)
	assert.Equal(t, 3, input.Status.RecordsTotal)
}

func TestSetCondition_AcceptedPreservesRecordsProvisioned(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}, {}, {}}, // 4 endpoints
		},
		Status: DNSEndpointStatus{
			Records:            "2/3",
			RecordsProvisioned: 2,
			RecordsTotal:       3,
		},
	}

	setCondition(input, string(DNSEndpointAccepted), metav1.ConditionUnknown, string(ReasonAccepted), "Accepted")

	assert.Equal(t, "2/4", input.Status.Records)
	assert.Equal(t, 2, input.Status.RecordsProvisioned)
	assert.Equal(t, 4, input.Status.RecordsTotal)
}

func TestSetCondition_AcceptedSetsProgrammedUnknownWhenNotAllProvisioned(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}, {}}, // 3 endpoints
		},
		Status: DNSEndpointStatus{
			Records:            "0/0",
			RecordsProvisioned: 0,
		},
	}

	setCondition(input, string(DNSEndpointAccepted), metav1.ConditionUnknown, string(ReasonAccepted), "Accepted")

	// Find Programmed condition
	var programmedCondition *metav1.Condition
	for i := range input.Status.Conditions {
		if input.Status.Conditions[i].Type == string(DNSEndpointProgrammed) {
			programmedCondition = &input.Status.Conditions[i]
			break
		}
	}

	assert.NotNil(t, programmedCondition)
	assert.Equal(t, metav1.ConditionUnknown, programmedCondition.Status)
	assert.Equal(t, string(ReasonPending), programmedCondition.Reason)
}

func TestSetCondition_ProgrammedTrueSetsAllRecordsProvisioned(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}, {}, {}, {}}, // 5 endpoints
		},
		Status: DNSEndpointStatus{
			Records:            "2/5",
			RecordsProvisioned: 2,
			RecordsTotal:       5,
		},
	}

	setCondition(input, string(DNSEndpointProgrammed), metav1.ConditionTrue, string(ReasonProgrammed), "Programmed")

	assert.Equal(t, "5/5", input.Status.Records)
	assert.Equal(t, 5, input.Status.RecordsProvisioned)
	assert.Equal(t, 5, input.Status.RecordsTotal)
	assert.Equal(t, int64(1), input.Status.ObservedGeneration)
}

func TestSetCondition_LastStatusChangeAlwaysSet(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Status:     DNSEndpointStatus{},
	}

	assert.Nil(t, input.Status.LastStatusChange)

	setCondition(input, "TestType", metav1.ConditionTrue, "TestReason", "Test message")

	assert.NotNil(t, input.Status.LastStatusChange)
}

func TestSetAccepted(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}},
		},
		Status: DNSEndpointStatus{
			Records: "0/0",
		},
	}

	SetAccepted(input, "Endpoint accepted", 1)

	// Find Accepted condition
	var acceptedCondition *metav1.Condition
	for i := range input.Status.Conditions {
		if input.Status.Conditions[i].Type == string(DNSEndpointAccepted) {
			acceptedCondition = &input.Status.Conditions[i]
			break
		}
	}

	assert.NotNil(t, acceptedCondition)
	assert.Equal(t, metav1.ConditionUnknown, acceptedCondition.Status)
	assert.Equal(t, string(ReasonAccepted), acceptedCondition.Reason)
	assert.Equal(t, "Endpoint accepted", acceptedCondition.Message)
}

func TestSetProgrammed(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}},
		},
		Status: DNSEndpointStatus{},
	}

	SetProgrammed(input, "Successfully programmed", 1)

	// Find Programmed condition
	var programmedCondition *metav1.Condition
	for i := range input.Status.Conditions {
		if input.Status.Conditions[i].Type == string(DNSEndpointProgrammed) {
			programmedCondition = &input.Status.Conditions[i]
			break
		}
	}

	assert.NotNil(t, programmedCondition)
	assert.Equal(t, metav1.ConditionTrue, programmedCondition.Status)
	assert.Equal(t, string(ReasonProgrammed), programmedCondition.Reason)
	assert.Equal(t, "Successfully programmed", programmedCondition.Message)
	assert.Equal(t, "2/2", input.Status.Records)
}

func TestSetRecords(t *testing.T) {
	status := &DNSEndpointStatus{}

	setRecords(status, 3, 5)

	assert.Equal(t, 3, status.RecordsProvisioned)
	assert.Equal(t, 5, status.RecordsTotal)
	assert.Equal(t, "3/5", status.Records)
}

func TestUpdateProgrammedStatus_UpdatesExisting(t *testing.T) {
	originalTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	status := &DNSEndpointStatus{
		Conditions: []metav1.Condition{
			{
				Type:               string(DNSEndpointProgrammed),
				Status:             metav1.ConditionFalse,
				Reason:             "OldReason",
				Message:            "Original message",
				ObservedGeneration: 1,
				LastTransitionTime: originalTime,
			},
		},
	}

	updateProgrammedStatus(status, metav1.ConditionTrue, string(ReasonProgrammed), 2)

	assert.Len(t, status.Conditions, 1)
	assert.Equal(t, metav1.ConditionTrue, status.Conditions[0].Status)
	assert.Equal(t, string(ReasonProgrammed), status.Conditions[0].Reason)
	assert.Equal(t, "Original message", status.Conditions[0].Message) // Message preserved
	assert.Equal(t, int64(2), status.Conditions[0].ObservedGeneration)
	assert.NotEqual(t, originalTime, status.Conditions[0].LastTransitionTime)
}

func TestUpdateProgrammedStatus_PreservesTimeWhenStatusUnchanged(t *testing.T) {
	originalTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	status := &DNSEndpointStatus{
		Conditions: []metav1.Condition{
			{
				Type:               string(DNSEndpointProgrammed),
				Status:             metav1.ConditionTrue,
				Reason:             "OldReason",
				Message:            "Original message",
				ObservedGeneration: 1,
				LastTransitionTime: originalTime,
			},
		},
	}

	updateProgrammedStatus(status, metav1.ConditionTrue, string(ReasonProgrammed), 2)

	// Should not update anything since status is the same
	assert.Equal(t, originalTime, status.Conditions[0].LastTransitionTime)
	assert.Equal(t, int64(1), status.Conditions[0].ObservedGeneration) // Not updated
}

func TestUpdateProgrammedStatus_CreatesNewIfNotExists(t *testing.T) {
	status := &DNSEndpointStatus{
		Conditions: []metav1.Condition{},
	}

	updateProgrammedStatus(status, metav1.ConditionUnknown, string(ReasonPending), 1)

	assert.Len(t, status.Conditions, 1)
	assert.Equal(t, string(DNSEndpointProgrammed), status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionUnknown, status.Conditions[0].Status)
	assert.Equal(t, string(ReasonPending), status.Conditions[0].Reason)
	assert.Equal(t, "Waiting for controller", status.Conditions[0].Message)
}

func TestSetFailed(t *testing.T) {
	input := &DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec: DNSEndpointSpec{
			Endpoints: []*endpoint.Endpoint{{}, {}, {}},
		},
		Status: DNSEndpointStatus{
			Records:            "0/3",
			RecordsProvisioned: 0,
			RecordsTotal:       3,
		},
	}

	SetFailed(input, "Failed to create record: timeout")

	// Find Programmed condition
	var programmedCondition *metav1.Condition
	for i := range input.Status.Conditions {
		if input.Status.Conditions[i].Type == string(DNSEndpointProgrammed) {
			programmedCondition = &input.Status.Conditions[i]
			break
		}
	}

	assert.NotNil(t, programmedCondition)
	assert.Equal(t, metav1.ConditionFalse, programmedCondition.Status)
	assert.Equal(t, string(ReasonFailed), programmedCondition.Reason)
	assert.Equal(t, "Failed to create record: timeout", programmedCondition.Message)
	// Records should not be modified by SetFailed
	assert.Equal(t, "0/3", input.Status.Records)
}
