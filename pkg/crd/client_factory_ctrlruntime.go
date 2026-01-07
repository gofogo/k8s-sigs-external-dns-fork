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

// REFACTORING NOTE: Controller-Runtime Client Factory
//
// This file provides a factory function for creating controller-runtime clients
// as an alternative to the manual REST client approach in client_factory.go.
//
// TO REMOVE THIS OPTION AND KEEP REST CLIENT:
// 1. Delete this file (pkg/crd/client_factory_ctrlruntime.go)
// 2. Delete pkg/crd/dnsendpoint_client_ctrlruntime.go
// 3. Remove CLIENT_IMPL=controller-runtime support from controller/execute.go
//
// TO REMOVE REST CLIENT AND KEEP THIS OPTION:
// 1. Delete pkg/crd/client_factory.go (REST factory)
// 2. Delete pkg/crd/dnsendpoint_client.go (REST implementation)
// 3. Remove CLIENT_IMPL=rest support from controller/execute.go
// 4. Remove environment variable checks
// 5. Rename functions to remove "ControllerRuntime" suffix

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// NewControllerRuntimeClient creates a controller-runtime client.Client for DNSEndpoint CRDs.
//
// This is a modern alternative to NewCRDClientForAPIVersionKind that uses controller-runtime
// instead of manual REST client setup. Benefits:
//   - No manual API discovery needed
//   - No manual serializer configuration
//   - Cleaner status subresource updates via client.Status().Update()
//   - Type-safe operations with client.Object interface
//
// Parameters:
//   - kubeConfig: Path to kubeconfig file (empty string uses default location)
//   - apiServerURL: Kubernetes API server URL (empty string uses kubeconfig)
//   - namespace: Namespace to scope client operations (empty string for cluster-wide)
//
// Returns:
//   - Controller-runtime client configured for DNSEndpoint CRDs
//   - Error if client creation fails
func NewControllerRuntimeClient(kubeConfig, apiServerURL, namespace string) (client.Client, error) {
	// Build REST config from kubeconfig
	config, err := buildRESTConfigForControllerRuntime(kubeConfig, apiServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	// Create scheme and register DNSEndpoint types
	scheme := runtime.NewScheme()
	if err := apiv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to add DNSEndpoint types to scheme: %w", err)
	}

	// Create controller-runtime client
	// Note: namespace parameter is not used for scoping the client itself,
	// as controller-runtime clients are cluster-wide by default.
	// Namespace filtering is done at the List/Get level.
	opts := client.Options{Scheme: scheme}

	c, err := client.New(config, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create controller-runtime client: %w", err)
	}

	return c, nil
}

// buildRESTConfigForControllerRuntime builds a REST config for controller-runtime client.
// This reuses the same logic as the REST client approach for consistency.
func buildRESTConfigForControllerRuntime(kubeConfig, apiServerURL string) (*rest.Config, error) {
	// Use default kubeconfig location if not specified
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}

	// Build REST config from kubeconfig and API server URL
	config, err := clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from flags: %w", err)
	}

	return config, nil
}
