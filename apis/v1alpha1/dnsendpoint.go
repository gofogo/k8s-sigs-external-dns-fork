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

	"sigs.k8s.io/external-dns/endpoint"
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
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNSEndpoint is a contract that a user-specified CRD must implement to be used as a source for external-dns.
// The user-specified CRD should also have the status sub-resource.
// +k8s:openapi-gen=true
// +groupName=externaldns.k8s.io
// +kubebuilder:resource:path=dnsendpoints
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=https://github.com/kubernetes-sigs/external-dns/pull/2007"
// +versionName=v1alpha1
type DNSEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DNSEndpointSpec   `json:"spec"`
	Status DNSEndpointStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DNSEndpointList is a list of DNSEndpoint objects
type DNSEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DNSEndpoint `json:"items"`
}

// DNSEndpointSpec defines the desired state of DNSEndpoint
type DNSEndpointSpec struct {
	Endpoints []*endpoint.Endpoint `json:"endpoints,omitempty"`
}

// DNSEndpointStatus defines the observed state of DNSEndpoint
type DNSEndpointStatus struct {
	// Conditions describe the current conditions of the DNSEndpoint.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:default={{type: "Accepted", status: "Unknown", reason:"Pending", message:"Waiting for controller", lastTransitionTime: "1970-01-01T00:00:00Z"},{type: "Programmed", status: "Unknown", reason:"Pending", message:"Waiting for controller", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
