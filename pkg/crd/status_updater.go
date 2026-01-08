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
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// REFACTORING NOTE: This is Option 1 - StatusUpdater in pkg/crd package
//
// PROS:
// - Clean separation: reusable status updater independent of source logic
// - Natural fit in pkg/crd alongside DNSEndpointClient (repository pattern)
// - Easy to test in isolation
// - No coupling to controller package
// - Can be used by any component that needs to update DNSEndpoint status
//
// CONS:
// - Adds another abstraction layer
// - Requires creating helper function to build it in execute.go
//
// TO REMOVE THIS OPTION:
// 1. Delete this file (pkg/crd/status_updater.go)
// 2. Delete pkg/crd/status_updater_test.go (if exists)
// 3. Remove StatusUpdater usage from controller/execute.go (search for "OPTION 1")
// 4. Keep Option 2 (controller/dnsendpoint_status.go) and its usage

// StatusUpdater handles DNSEndpoint status updates.
// This is a service layer component that uses DNSEndpointClient (repository layer)
// to perform status update operations on DNSEndpoint CRDs.
type StatusUpdater interface {
	// UpdateDNSEndpointStatus updates the status of a single DNSEndpoint CRD.
	// Parameters:
	//   - ctx: Context for the operation
	//   - namespace: Kubernetes namespace of the DNSEndpoint
	//   - name: Name of the DNSEndpoint resource
	//   - success: Whether the DNS sync was successful
	//   - message: Human-readable message describing the sync result
	// Returns error if the update fails.
	UpdateDNSEndpointStatus(ctx context.Context, namespace, name string, success bool, message string) error
}

// dnsEndpointStatusUpdater implements StatusUpdater interface
type dnsEndpointStatusUpdater struct {
	client DNSEndpointClient
}

// NewDNSEndpointStatusUpdater creates a new status updater for DNSEndpoint CRDs.
// This updater can be used to update DNSEndpoint status after DNS synchronization.
//
// Parameters:
//   - client: DNSEndpointClient for accessing DNSEndpoint CRDs
//
// Returns a StatusUpdater that can update DNSEndpoint status fields.
func NewDNSEndpointStatusUpdater(client DNSEndpointClient) StatusUpdater {
	return &dnsEndpointStatusUpdater{
		client: client,
	}
}

// UpdateDNSEndpointStatus updates the status of a single DNSEndpoint.
// This method:
// 1. Fetches the current DNSEndpoint from the API server
// 2. Updates the status conditions based on success/failure
// 3. Writes the updated status back to the API server
//
// The status update uses the status subresource, which means:
// - Only the status field is modified
// - The spec and metadata remain unchanged
// - Status updates don't trigger reconciliation loops
func (u *dnsEndpointStatusUpdater) UpdateDNSEndpointStatus(ctx context.Context, namespace, name string, success bool, message string) error {
	// 1. Get current DNSEndpoint from API server
	dnsEndpoint, err := u.client.Get(ctx, namespace, name)
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
	_, err = u.client.UpdateStatus(ctx, dnsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Debugf("Updated status of DNSEndpoint %s/%s: success=%v", namespace, name, success)
	return nil
}
