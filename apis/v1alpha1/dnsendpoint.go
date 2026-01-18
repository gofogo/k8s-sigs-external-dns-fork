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
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSEndpointSpec   `json:"spec,omitempty"`
	Status DNSEndpointStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DNSEndpointList is a list of DNSEndpoint objects
type DNSEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSEndpoint `json:"items"`
}

// DNSEndpointSpec defines the desired state of DNSEndpoint
type DNSEndpointSpec struct {
	Endpoints []*endpoint.Endpoint `json:"endpoints,omitempty"`
}

// DNSEndpointStatus defines the observed state of DNSEndpoint
type DNSEndpointStatus struct {
	// The generation observed by the external-dns controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions describe the current conditions of the DNSEndpoint.
	//
	// Known condition types are:
	//
	// * "Accepted"
	// * "Programmed"
	// * "Ready"
	//
	// <externaldns:util:excludeFromCRD>
	// Notes for implementors:
	//
	// Conditions are a listType `map`, which means that they function like a
	// map with a key of the `type` field _in the k8s apiserver_.
	//
	// This means that implementations must obey some rules when updating this
	// section.
	//
	// * Implementations MUST perform a read-modify-write cycle on this field
	//   before modifying it. That is, when modifying this field, implementations
	//   must be confident they have fetched the most recent version of this field,
	//   and ensure that changes they make are on that recent version.
	// * Implementations MUST NOT remove or reorder Conditions that they are not
	//   directly responsible for. For example, if an implementation sees a Condition
	//   with type `special.io/SomeField`, it MUST NOT remove, change or update that
	//   Condition.
	// * If the `observedGeneration` of a Condition is _greater than_ the value the
	//   implementation knows about, then it MUST NOT perform the update on that Condition,
	//   but must wait for a future reconciliation and status update. (The assumption is that
	//   the implementation's copy of the object is stale and an update will be re-triggered
	//   if relevant.)
	//
	// </externaldns:util:excludeFromCRD>
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:default={{type: "Accepted", status: "Unknown", reason:"Pending", message:"Waiting for controller", lastTransitionTime: "1970-01-01T00:00:00Z"},{type: "Programmed", status: "Unknown", reason:"Pending", message:"Waiting for controller", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
