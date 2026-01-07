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

package controller

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/plan"
)

// dnsEndpointStatusUpdater is an interface for sources that support DNSEndpoint status updates
type dnsEndpointStatusUpdater interface {
	Get(ctx context.Context, namespace, name string) (*apiv1alpha1.DNSEndpoint, error)
	UpdateStatus(ctx context.Context, dnsEndpoint *apiv1alpha1.DNSEndpoint) (*apiv1alpha1.DNSEndpoint, error)
}

// updateDNSEndpointStatus updates the status of DNSEndpoint CRDs based on sync results
func (c *Controller) updateDNSEndpointStatus(ctx context.Context, changes *plan.Changes, success bool, message string) {
	fmt.Println("DEBUG: updateDNSEndpointStatus called") // Debug line
	// Quick skip if status updates are disabled
	if !c.UpdateDNSEndpointStatus {
		return
	}

	// Check if the source supports DNSEndpoint status updates
	statusUpdater, ok := c.Source.(dnsEndpointStatusUpdater)
	if !ok {
		// Source doesn't support status updates, skip
		return
	}
	fmt.Println("DEBUG: updateDNSEndpointStatus statusUpdater ok") // Debug line

	// Collect unique DNSEndpoint references from all endpoints in the plan
	dnsEndpoints := make(map[string]struct {
		namespace string
		name      string
		uid       string
	})

	// TODO: review
	// Check all endpoints in Creates, UpdateOld, UpdateNew, and Delete
	allEndpoints := append(append(append(
		changes.Create,
		changes.UpdateOld...),
		changes.UpdateNew...))

	for _, ep := range allEndpoints {
		ref := ep.RefObject()
		if ref != nil && ref.Kind == "DNSEndpoint" {
			key := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
			dnsEndpoints[key] = struct {
				namespace string
				name      string
				uid       string
			}{
				namespace: ref.Namespace,
				name:      ref.Name,
				uid:       string(ref.UID),
			}
		}
	}

	// Update status for each unique DNSEndpoint
	for _, ref := range dnsEndpoints {
		if err := updateSingleDNSEndpointStatus(ctx, statusUpdater, ref.namespace, ref.name, success, message); err != nil {
			log.Warnf("Failed to update status for DNSEndpoint %s/%s: %v",
				ref.namespace, ref.name, err)
		}
	}
}

// updateSingleDNSEndpointStatus updates status for a single DNSEndpoint
func updateSingleDNSEndpointStatus(ctx context.Context, statusUpdater dnsEndpointStatusUpdater, namespace, name string, success bool, message string) error {
	// Fetch the current DNSEndpoint
	dnsEndpoint, err := statusUpdater.Get(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get DNSEndpoint: %w", err)
	}

	// Update status fields based on success/failure
	if success {
		apiv1alpha1.SetSyncSuccess(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
	} else {
		apiv1alpha1.SetSyncFailed(&dnsEndpoint.Status, message, dnsEndpoint.Generation)
	}

	// Update the status
	_, err = statusUpdater.UpdateStatus(ctx, dnsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	log.Debugf("Updated status for DNSEndpoint %s/%s: success=%v, message=%s",
		namespace, name, success, message)

	return nil
}
