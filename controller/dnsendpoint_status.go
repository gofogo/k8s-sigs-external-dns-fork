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

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/pkg/crd"
)

// REFACTORING NOTE: This is Option 2 - Status manager in controller package
//
// PROS:
// - Status update logic lives where it's used (controller owns orchestration)
// - Clear that status updates are controller-specific concern
// - No need to expose StatusUpdater interface in pkg/crd
// - Simpler: one less abstraction to understand
//
// CONS:
// - Controller package now depends on pkg/crd package
// - Less reusable (tied to controller context)
// - Mixes controller orchestration with CRD-specific update logic
// - Can't use this status updater from other packages without importing controller
//
// TO REMOVE THIS OPTION:
// 1. Delete this file (controller/dnsendpoint_status.go)
// 2. Delete controller/dnsendpoint_status_test.go (if exists)
// 3. Remove DNSEndpointStatusManager usage from controller/execute.go (search for "OPTION 2")
// 4. Keep Option 1 (pkg/crd/status_updater.go) and its usage

// DNSEndpointStatusManager manages status updates for DNSEndpoint CRDs.
// This is a controller-owned component that updates DNSEndpoint status
// after DNS synchronization completes.
//
// Design Decision: Status updates are owned by the controller because:
// - Controller orchestrates the sync loop
// - Controller knows when sync succeeds or fails
// - Controller has the context (plan.Changes) needed to identify which CRDs to update
type DNSEndpointStatusManager struct {
	// client is the repository layer for accessing DNSEndpoint CRDs
	client crd.DNSEndpointClient
}

// NewDNSEndpointStatusManager creates a new status manager for DNSEndpoint CRDs.
//
// Parameters:
//   - client: DNSEndpointClient for accessing DNSEndpoint CRDs
//
// Returns a status manager that can update DNSEndpoint status.
func NewDNSEndpointStatusManager(client crd.DNSEndpointClient) *DNSEndpointStatusManager {
	return &DNSEndpointStatusManager{
		client: client,
	}
}

// UpdateStatus updates the status of a single DNSEndpoint.
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
// 1. Fetches the current DNSEndpoint from the API server
// 2. Updates the status conditions based on success/failure
// 3. Writes the updated status back to the API server (status subresource)
func (m *DNSEndpointStatusManager) UpdateStatus(ctx context.Context, namespace, name string, success bool, message string) error {
	// 1. Get current DNSEndpoint from API server
	dnsEndpoint, err := m.client.Get(ctx, namespace, name)
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

	// 3. Update status subresource via API server
	// This uses PUT /apis/externaldns.k8s.io/v1alpha1/namespaces/{ns}/dnsendpoints/{name}/status
	_, err = m.client.UpdateStatus(ctx, dnsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v", namespace, name, success)
	return nil
}
