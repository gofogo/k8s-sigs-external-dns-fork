package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSEndpointConditionType is a type of condition for a DNSEndpoint.
type DNSEndpointConditionType string

// ConditionReason is a reason for a DNSEndpoint condition.
type ConditionReason string

// Condition types for DNSEndpoint status
const (
	// DNSEndpointAccepted indicates the endpoint has been accepted by the controller
	DNSEndpointAccepted DNSEndpointConditionType = "Accepted"

	// DNSEndpointProgrammed indicates the endpoint has been successfully programmed to the DNS provider
	DNSEndpointProgrammed DNSEndpointConditionType = "Programmed"
)

// Condition reasons for DNSEndpoint status
const (
	// ReasonAccepted indicates the endpoint has been accepted
	ReasonAccepted ConditionReason = "Accepted"

	// ReasonProgrammed indicates successful programming to DNS provider
	ReasonProgrammed ConditionReason = "Programmed"

	// ReasonInvalid indicates the endpoint is invalid
	ReasonInvalid ConditionReason = "Invalid"

	// ReasonPending indicates the endpoint is pending processing
	ReasonPending ConditionReason = "Pending"

	// ReasonFailed indicates one or more records failed to provision
	ReasonFailed ConditionReason = "Failed"
)

// setCondition adds or updates a condition in the DNSEndpointStatus.
//
// Behavior:
//   - LastTransitionTime is only updated when the status actually changes
//   - LastStatusChange is always updated to track the most recent status modification
//   - ObservedGeneration is set from input.Generation
//
// Side effects for specific condition types:
//   - DNSEndpointAccepted: Updates Records count and syncs Programmed condition status.
//     If not all records are provisioned, sets Programmed=Unknown.
//   - DNSEndpointProgrammed (with ConditionTrue): Sets all records as provisioned
//     and updates ObservedGeneration.
func setCondition(
	input *DNSEndpoint,
	conditionType string,
	conditionStatus metav1.ConditionStatus,
	reason, message string) {
	status := &input.Status
	var existingCondition *metav1.Condition
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			existingCondition = &status.Conditions[i]
			break
		}
	}
	// Create new condition
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: input.Generation,
		LastTransitionTime: metav1.NewTime(metav1.Now().Time),
	}

	if conditionType == string(DNSEndpointAccepted) {
		if status.Records == "0/0" {
			setRecords(status, 0, len(input.Spec.Endpoints))
		} else {
			setRecords(status, status.RecordsProvisioned, len(input.Spec.Endpoints))
		}
		// Update Programmed condition status based on whether all records are provisioned
		if status.RecordsProvisioned != len(input.Spec.Endpoints) {
			updateProgrammedStatus(status, metav1.ConditionUnknown, string(ReasonPending), input.Generation)
		}
	} else if conditionType == string(DNSEndpointProgrammed) && conditionStatus == metav1.ConditionTrue {
		setRecords(status, len(input.Spec.Endpoints), len(input.Spec.Endpoints))
		status.ObservedGeneration = input.Generation
	}

	// If condition exists, preserve LastTransitionTime if status hasn't changed
	if existingCondition != nil {
		if existingCondition.Status == conditionStatus {
			newCondition.LastTransitionTime = existingCondition.LastTransitionTime
		}
		// Replace existing condition
		for i := range status.Conditions {
			if status.Conditions[i].Type == conditionType {
				status.Conditions[i] = newCondition
				break
			}
		}
	} else {
		// Add new condition
		status.Conditions = append(status.Conditions, newCondition)
	}
	status.LastStatusChange = &newCondition.LastTransitionTime
}

// SetAccepted marks the endpoint as accepted by the controller with Unknown status.
// Use this when the endpoint is first seen and validated, but not yet processed.
func SetAccepted(input *DNSEndpoint, message string, observedGeneration int64) {
	setCondition(input, string(DNSEndpointAccepted), metav1.ConditionUnknown,
		string(ReasonAccepted), message)
}

// SetProgrammed marks the endpoint as successfully programmed to the DNS provider.
func SetProgrammed(input *DNSEndpoint, message string, observedGeneration int64) {
	setCondition(input, string(DNSEndpointProgrammed), metav1.ConditionTrue,
		string(ReasonProgrammed), message)
}

// SetFailed marks the endpoint as failed to program to the DNS provider.
func SetFailed(input *DNSEndpoint, message string) {
	setCondition(input, string(DNSEndpointProgrammed), metav1.ConditionFalse,
		string(ReasonFailed), message)
}

// setRecords updates the records count fields in the status.
// It sets RecordsProvisioned, RecordsTotal, and the display string Records (e.g., "3/5").
func setRecords(status *DNSEndpointStatus, provisioned, total int) {
	status.RecordsProvisioned = provisioned
	status.RecordsTotal = total
	status.Records = fmt.Sprintf("%d/%d", provisioned, total)
}

// updateProgrammedStatus updates the status field of the Programmed condition,
// preserving the existing message if present.
func updateProgrammedStatus(status *DNSEndpointStatus, conditionStatus metav1.ConditionStatus, reason string, generation int64) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == string(DNSEndpointProgrammed) {
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i].Status = conditionStatus
				status.Conditions[i].Reason = reason
				status.Conditions[i].ObservedGeneration = generation
				status.Conditions[i].LastTransitionTime = metav1.NewTime(metav1.Now().Time)
			}
			return
		}
	}
	// No existing Programmed condition, create one
	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               string(DNSEndpointProgrammed),
		Status:             conditionStatus,
		Reason:             reason,
		Message:            "Waiting for controller",
		ObservedGeneration: generation,
		LastTransitionTime: metav1.NewTime(metav1.Now().Time),
	})
}
