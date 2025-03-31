
// ref: https://book.kubebuilder.io/cronjob-tutorial/other-api-files

// Package v1alpha1 contains API Schema definitions for the externaldns.k8s.io v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=externaldns.k8s.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "externaldns.k8s.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&DNSEndpoint{}, &DNSEndpointList{})
}
