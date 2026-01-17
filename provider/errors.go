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

package provider

// This file contains standardized error utilities for DNS providers.
// Using these functions ensures consistent error messages and wrapping
// across all provider implementations.

// SoftErrorZones creates a soft error for zone listing failures.
// Use this when zone retrieval fails due to transient issues.
func SoftErrorZones(err error) error {
	return NewSoftErrorf("failed to list zones: %w", err)
}

// SoftErrorZonesWithContext creates a soft error for zone listing failures with additional context.
func SoftErrorZonesWithContext(err error, context string) error {
	return NewSoftErrorf("failed to list zones (%s): %w", context, err)
}

// SoftErrorRecords creates a soft error for record retrieval failures.
// Use this when fetching DNS records fails due to transient issues.
func SoftErrorRecords(err error) error {
	return NewSoftErrorf("failed to fetch records: %w", err)
}

// SoftErrorRecordsForZone creates a soft error for record retrieval failures for a specific zone.
func SoftErrorRecordsForZone(err error, zone string) error {
	return NewSoftErrorf("failed to fetch records for zone %s: %w", zone, err)
}

// SoftErrorApplyChanges creates a soft error for apply changes failures.
// Use this when applying DNS changes fails due to transient issues.
func SoftErrorApplyChanges(err error) error {
	return NewSoftErrorf("failed to apply changes: %w", err)
}

// SoftErrorApplyChangesForZones creates a soft error when changes fail for specific zones.
func SoftErrorApplyChangesForZones(failedZones []string) error {
	return NewSoftErrorf("failed to submit changes for zones: %v", failedZones)
}

// SoftErrorTags creates a soft error for tag listing failures.
// Use this when fetching zone tags fails due to transient issues.
func SoftErrorTags(err error) error {
	return NewSoftErrorf("failed to list tags: %w", err)
}
