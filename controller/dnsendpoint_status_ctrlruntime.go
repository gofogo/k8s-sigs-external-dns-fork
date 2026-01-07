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

package controller

// REFACTORING NOTE: Direct Controller-Runtime Status Manager (Option 2)
//
// This file provides a status manager that uses controller-runtime client.Client directly,
// bypassing the DNSEndpointClient interface. This is an alternative to the interface-based
// approach in dnsendpoint_status.go.
//
// PROS:
// - More idiomatic controller-runtime usage (no interface abstraction)
// - Simpler: Direct client.Client usage
// - Modern: Follows controller-runtime patterns
// - No interface overhead
//
// CONS:
// - Less flexible: Tightly coupled to controller-runtime
// - Can't easily swap client implementations
// - Controller package has direct dependency on client type
//
// TO REMOVE THIS OPTION AND KEEP INTERFACE-BASED (dnsendpoint_status.go):
// 1. Delete this file (controller/dnsendpoint_status_ctrlruntime.go)
// 2. Delete controller/dnsendpoint_status_ctrlruntime_test.go
// 3. Remove CLIENT_IMPL=controller-runtime support from execute.go:registerStatusUpdateCallbacksOption2
//
// TO REMOVE INTERFACE-BASED AND KEEP THIS OPTION:
// 1. Delete controller/dnsendpoint_status.go
// 2. Remove CLIENT_IMPL=rest support from execute.go:registerStatusUpdateCallbacksOption2
// 3. Rename this file to dnsendpoint_status.go
// 4. Rename NewDNSEndpointStatusManagerCtrlRuntime to NewDNSEndpointStatusManager
// 5. Remove environment variable checks

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// DNSEndpointStatusManagerCtrlRuntime manages DNSEndpoint status using controller-runtime client directly.
//
// Unlike DNSEndpointStatusManager which uses the DNSEndpointClient interface,
// this implementation uses client.Client directly for a more idiomatic controller-runtime approach.
//
// Design Decision: Status updates are owned by the controller because:
// - Controller orchestrates the sync loop
// - Controller knows when sync succeeds or fails
// - Controller has the context (plan.Changes) needed to identify which CRDs to update
type DNSEndpointStatusManagerCtrlRuntime struct {
	// client is the controller-runtime client for accessing DNSEndpoint CRDs
	client client.Client
}

// NewDNSEndpointStatusManagerCtrlRuntime creates a status manager using controller-runtime client directly.
//
// This is an alternative to NewDNSEndpointStatusManager that bypasses the DNSEndpointClient interface
// and uses controller-runtime's client.Client directly.
//
// Parameters:
//   - c: Controller-runtime client (from crd.NewControllerRuntimeClient)
//
// Returns a status manager that can update DNSEndpoint status using controller-runtime.
func NewDNSEndpointStatusManagerCtrlRuntime(c client.Client) *DNSEndpointStatusManagerCtrlRuntime {
	return &DNSEndpointStatusManagerCtrlRuntime{
		client: c,
	}
}

// UpdateStatus updates the status of a single DNSEndpoint using controller-runtime client.
//
// This method is called by the controller callback after DNS changes are applied.
//
// Parameters:
//   - ctx: Context for the operation
//   - namespace: Kubernetes namespace of the DNSEndpoint
//   - name: Name of the DNSEndpoint resource
//   - success: Whether the DNS sync was successful
//   - message: Human-readable message describing the sync result
//
// Returns error if the update fails.
//
// Implementation:
// 1. Fetches the current DNSEndpoint from the API server using client.Get()
// 2. Updates the status conditions based on success/failure
// 3. Writes the updated status back using client.Status().Update()
func (m *DNSEndpointStatusManagerCtrlRuntime) UpdateStatus(ctx context.Context, namespace, name string, success bool, message string) error {
	// 1. Get current DNSEndpoint from API server using controller-runtime client
	dnsEndpoint := &apiv1alpha1.DNSEndpoint{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	err := m.client.Get(ctx, key, dnsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to get DNSEndpoint: %w", err)
	}

	// 2. Update status conditions based on sync result
	// Uses helper functions from apis/v1alpha1/status_helpers.go
	if success {
		// SetSyncSuccess updates:
		// - Programmed condition to True
		// - ObservedGeneration to current generation
		// - LastTransitionTime to now
		apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
	} else {
		// SetSyncFailed updates:
		// - Programmed condition to False
		// - Adds error message
		// - ObservedGeneration to current generation
		// - LastTransitionTime to now
		apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
	}

	// 3. Update status subresource using controller-runtime
	// client.Status().Update() automatically uses the status subresource
	// This is cleaner than REST client which requires explicit .SubResource("status")
	err = m.client.Status().Update(ctx, dnsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v (controller-runtime direct)", namespace, name, success)
	return nil
}
