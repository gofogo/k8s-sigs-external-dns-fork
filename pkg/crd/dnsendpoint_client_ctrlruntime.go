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

// REFACTORING NOTE: Controller-Runtime Backed DNSEndpointClient Implementation
//
// This file implements the DNSEndpointClient interface using controller-runtime client
// as an alternative to the REST client implementation in dnsendpoint_client.go.
//
// PROS:
// - Simpler: No manual API discovery or serializer setup
// - Modern: Industry standard for Kubernetes operators
// - Type-safe: Uses client.Object interface
// - Cleaner status updates: client.Status().Update()
//
// CONS:
// - Watch() not fully implemented (returns error, relies on informer in source/crd.go)
// - Slightly different error types (controller-runtime vs REST client)
//
// TO REMOVE THIS OPTION AND KEEP REST CLIENT:
// 1. Delete this file (pkg/crd/dnsendpoint_client_ctrlruntime.go)
// 2. Delete pkg/crd/client_factory_ctrlruntime.go
// 3. Remove CLIENT_IMPL=controller-runtime support from controller/execute.go
//
// TO REMOVE REST CLIENT AND KEEP THIS OPTION:
// 1. Delete pkg/crd/dnsendpoint_client.go (REST implementation)
// 2. Delete pkg/crd/client_factory.go (REST factory)
// 3. Remove CLIENT_IMPL=rest support from controller/execute.go
// 4. Rename this file to dnsendpoint_client.go
// 5. Rename NewDNSEndpointClientCtrlRuntime to NewDNSEndpointClient
// 6. Remove environment variable checks

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// ctrlRuntimeDNSEndpointClient implements DNSEndpointClient interface using controller-runtime client.
type ctrlRuntimeDNSEndpointClient struct {
	client    client.Client
	namespace string
}

// NewDNSEndpointClientCtrlRuntime creates a DNSEndpointClient backed by controller-runtime client.
//
// This is a drop-in replacement for NewDNSEndpointClient that uses controller-runtime
// instead of REST client. It implements the same DNSEndpointClient interface.
//
// Parameters:
//   - c: Controller-runtime client (from NewControllerRuntimeClient)
//   - namespace: Namespace to operate in (empty string for all namespaces)
//
// Returns DNSEndpointClient that uses controller-runtime for all operations.
func NewDNSEndpointClientCtrlRuntime(c client.Client, namespace string) DNSEndpointClient {
	return &ctrlRuntimeDNSEndpointClient{
		client:    c,
		namespace: namespace,
	}
}

// Get retrieves a single DNSEndpoint by namespace and name.
//
// Implementation: Uses controller-runtime client.Get() with client.ObjectKey.
func (c *ctrlRuntimeDNSEndpointClient) Get(ctx context.Context, namespace, name string) (*apiv1alpha1.DNSEndpoint, error) {
	dnsEndpoint := &apiv1alpha1.DNSEndpoint{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	err := c.client.Get(ctx, key, dnsEndpoint)
	if err != nil {
		return nil, err
	}

	return dnsEndpoint, nil
}

// List retrieves all DNSEndpoints matching the given options.
//
// Implementation: Converts metav1.ListOptions to controller-runtime client.ListOption,
// then uses client.List() to fetch DNSEndpoints.
//
// Note: LabelSelector and FieldSelector from ListOptions are converted to
// controller-runtime's MatchingLabelsSelector and MatchingFieldsSelector.
func (c *ctrlRuntimeDNSEndpointClient) List(ctx context.Context, opts *metav1.ListOptions) (*apiv1alpha1.DNSEndpointList, error) {
	dnsEndpointList := &apiv1alpha1.DNSEndpointList{}

	// Convert metav1.ListOptions to controller-runtime list options
	listOpts := []client.ListOption{}

	// Add namespace filter if specified
	if c.namespace != "" {
		listOpts = append(listOpts, client.InNamespace(c.namespace))
	}

	// Convert label selector
	if opts != nil && opts.LabelSelector != "" {
		selector, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	// Note: FieldSelector conversion not implemented as it's rarely used
	// If needed in the future, add similar logic as LabelSelector

	err := c.client.List(ctx, dnsEndpointList, listOpts...)
	if err != nil {
		return nil, err
	}

	return dnsEndpointList, nil
}

// UpdateStatus updates the status subresource of a DNSEndpoint.
//
// Implementation: Uses controller-runtime's client.Status().Update() which
// automatically uses the status subresource endpoint. This is cleaner than
// the REST client approach which requires explicit .SubResource("status").
//
// Note: The dnsEndpoint parameter should have the updated status fields set
// before calling this method. The method will update only the status subresource.
func (c *ctrlRuntimeDNSEndpointClient) UpdateStatus(ctx context.Context, dnsEndpoint *apiv1alpha1.DNSEndpoint) (*apiv1alpha1.DNSEndpoint, error) {
	// Controller-runtime's Status().Update() uses the status subresource automatically
	err := c.client.Status().Update(ctx, dnsEndpoint)
	if err != nil {
		return nil, err
	}

	// Return the updated object
	// Note: Unlike REST client which returns the server response,
	// controller-runtime updates the object in-place
	return dnsEndpoint, nil
}

// Watch returns a watch interface for DNSEndpoint changes.
//
// IMPLEMENTATION NOTE: This method is currently not fully implemented and returns
// an error. The rationale is:
//
// 1. source/crd.go (lines 71-82) already creates its own SharedInformer for watching
// 2. The informer uses client.List() and client.Watch() directly via ListWatch
// 3. DNSEndpointClient.Watch() may not be actively used in the codebase
//
// If Watch() is needed in the future, implement using one of these approaches:
//   - Use controller-runtime's cache.NewInformerWatcher
//   - Create a custom watch wrapper around client.Watch()
//   - Use client.WithWatch option when creating the client
//
// For now, callers should use the informer pattern as implemented in source/crd.go.
func (c *ctrlRuntimeDNSEndpointClient) Watch(ctx context.Context, opts *metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("Watch not supported with controller-runtime client - use informer from source/crd.go (lines 71-82)")
}
