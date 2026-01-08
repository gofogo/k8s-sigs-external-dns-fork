/*
Copyright 2025 The Kubernetes Authors.

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

package crd

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// NewCRDClientForAPIVersionKind creates a REST client for the specified CRD API version and kind.
// This function:
//   - Discovers the CRD resource in the cluster
//   - Creates a runtime scheme with the necessary type registrations
//   - Configures and returns a REST client for accessing the CRD
//
// Parameters:
//   - client: Kubernetes clientset for API discovery
//   - kubeConfig: Path to kubeconfig file (empty string to use default)
//   - apiServerURL: Kubernetes API server URL (empty string to use kubeconfig)
//   - apiVersion: Full API version (e.g., "externaldns.k8s.io/v1alpha1")
//   - kind: Resource kind (e.g., "DNSEndpoint")
//
// Returns:
//   - REST client configured for the CRD
//   - Runtime scheme with registered types
//   - Error if client creation fails
func NewCRDClientForAPIVersionKind(client kubernetes.Interface, kubeConfig, apiServerURL, apiVersion, kind string) (*rest.RESTClient, *runtime.Scheme, error) {
	// Use default kubeconfig location if not specified
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}

	// Build REST config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	// Parse and validate the API group version
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, nil, err
	}

	// Discover the CRD resource in the cluster
	apiResourceList, err := client.Discovery().ServerResourcesForGroupVersion(groupVersion.String())
	if err != nil {
		return nil, nil, fmt.Errorf("error listing resources in GroupVersion %q: %w", groupVersion.String(), err)
	}

	// Find the specific resource kind
	var crdAPIResource *metav1.APIResource
	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Kind == kind {
			crdAPIResource = &apiResource
			break
		}
	}
	if crdAPIResource == nil {
		return nil, nil, fmt.Errorf("unable to find Resource Kind %q in GroupVersion %q", kind, apiVersion)
	}

	// Create scheme and register DNSEndpoint types
	scheme := runtime.NewScheme()
	if err := apiv1alpha1.AddToScheme(scheme); err != nil {
		return nil, nil, fmt.Errorf("failed to add types to scheme: %w", err)
	}
	// Add metav1 types for ListOptions, etc.
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, nil, fmt.Errorf("failed to add meta types to scheme: %w", err)
	}

	// Configure REST client for CRD access
	config.GroupVersion = &groupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	// Create and return the REST client
	crdClient, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, nil, err
	}

	return crdClient, scheme, nil
}
