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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition adds or updates a condition in the DNSEndpointStatus.
// It handles the LastTransitionTime properly - only updating it when the status changes.
func SetCondition(status *DNSEndpointStatus, conditionType string, conditionStatus metav1.ConditionStatus, reason, message string, observedGeneration int64) {
	now := metav1.NewTime(metav1.Now().Time)

	// Find existing condition
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
		ObservedGeneration: observedGeneration,
		LastTransitionTime: now,
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
				return
			}
		}
	}

	// Add new condition
	status.Conditions = append(status.Conditions, newCondition)
}

// GetCondition retrieves a condition by type from the DNSEndpointStatus.
// Returns nil if the condition is not found.
func GetCondition(status *DNSEndpointStatus, conditionType string) *metav1.Condition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}

// IsConditionTrue checks if a condition exists and has status True.
func IsConditionTrue(status *DNSEndpointStatus, conditionType string) bool {
	condition := GetCondition(status, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// SetAccepted marks the endpoint as accepted by the controller with Unknown status.
// Use this when the endpoint is first seen and validated, but not yet processed.
func SetAccepted(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetCondition(status, string(DNSEndpointAccepted), metav1.ConditionUnknown,
		string(ReasonAccepted), message, observedGeneration)
}

// SetAcceptedTrue marks the endpoint as accepted and confirmed by the controller.
func SetAcceptedTrue(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetCondition(status, string(DNSEndpointAccepted), metav1.ConditionTrue,
		string(ReasonAccepted), message, observedGeneration)
}

// SetProgrammed marks the endpoint as successfully programmed to the DNS provider.
func SetProgrammed(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetCondition(status, string(DNSEndpointProgrammed), metav1.ConditionTrue,
		string(ReasonProgrammed), message, observedGeneration)
}

// SetProgrammedFailed marks the endpoint programming as failed.
func SetProgrammedFailed(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetCondition(status, string(DNSEndpointProgrammed), metav1.ConditionFalse,
		string(ReasonInvalid), message, observedGeneration)
}

// SetSyncSuccess is a high-level helper that marks the endpoint as successfully synced.
// It sets Accepted to True and Programmed to True with appropriate messages.
func SetSyncSuccess(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetAcceptedTrue(status, "DNSEndpoint accepted", observedGeneration)
	SetProgrammed(status, message, observedGeneration)
}

// SetSyncFailed is a high-level helper that marks the endpoint sync as failed.
// It sets Accepted to True and Programmed to False.
func SetSyncFailed(status *DNSEndpointStatus, message string, observedGeneration int64) {
	SetAcceptedTrue(status, "DNSEndpoint accepted", observedGeneration)
	SetProgrammedFailed(status, message, observedGeneration)
}
