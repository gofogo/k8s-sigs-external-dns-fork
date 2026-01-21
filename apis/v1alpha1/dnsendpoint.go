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
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Programmed')].status"
// +kubebuilder:printcolumn:name="RECORDS",type="string",JSONPath=".status.records"
// +kubebuilder:printcolumn:name="LAST CHANGE",type="date",JSONPath=".status.lastStatusChange"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
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
	// +kubebuilder:default=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions describe the current conditions of the DNSEndpoint.
	//
	// Known condition types are:
	//
	// * "Accepted"
	// * "Programmed"
	// * "Degraded"
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:default={{type: "Programmed", status: "Unknown", reason:"Pending", message:"Waiting for controller", lastTransitionTime: "1970-01-01T00:00:00Z", observedGeneration: 0}}
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Records shows provisioned/total records (e.g., "3/5").
	// This field is for display purposes in kubectl output.
	// +optional
	// +kubebuilder:default="0/0"
	Records string `json:"records,omitempty"`

	// RecordsTotal is the total number of records defined in spec.
	// +optional
	// +kubebuilder:default=0
	RecordsTotal int `json:"recordsTotal,omitempty"`

	// RecordsProvisioned is the number of records successfully provisioned.
	// +optional
	// +kubebuilder:default=0
	RecordsProvisioned int `json:"recordsProvisioned,omitempty"`

	// LastStatusChange is the timestamp of the most recent status update.
	// +optional
	// +kubebuilder:default="1970-01-01T00:00:00Z"
	LastStatusChange *metav1.Time `json:"lastStatusChange,omitempty"`
}
