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

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSoftErrorZones(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := SoftErrorZones(originalErr)

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to list zones")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestSoftErrorZonesWithContext(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := SoftErrorZonesWithContext(originalErr, "compartment-123")

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to list zones")
	assert.Contains(t, err.Error(), "compartment-123")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestSoftErrorRecords(t *testing.T) {
	originalErr := errors.New("timeout")
	err := SoftErrorRecords(originalErr)

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to fetch records")
	assert.Contains(t, err.Error(), "timeout")
}

func TestSoftErrorRecordsForZone(t *testing.T) {
	originalErr := errors.New("rate limited")
	err := SoftErrorRecordsForZone(originalErr, "example.com")

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to fetch records for zone example.com")
	assert.Contains(t, err.Error(), "rate limited")
}

func TestSoftErrorApplyChanges(t *testing.T) {
	originalErr := errors.New("permission denied")
	err := SoftErrorApplyChanges(originalErr)

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to apply changes")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestSoftErrorApplyChangesForZones(t *testing.T) {
	failedZones := []string{"zone1.com", "zone2.com"}
	err := SoftErrorApplyChangesForZones(failedZones)

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to submit changes for zones")
	assert.Contains(t, err.Error(), "zone1.com")
	assert.Contains(t, err.Error(), "zone2.com")
}

func TestSoftErrorTags(t *testing.T) {
	originalErr := errors.New("access denied")
	err := SoftErrorTags(originalErr)

	assert.True(t, errors.Is(err, SoftError))
	assert.Contains(t, err.Error(), "failed to list tags")
	assert.Contains(t, err.Error(), "access denied")
}

func TestSoftErrorsAreUnwrappable(t *testing.T) {
	originalErr := errors.New("original error")

	testCases := []struct {
		name string
		err  error
	}{
		{"SoftErrorZones", SoftErrorZones(originalErr)},
		{"SoftErrorRecords", SoftErrorRecords(originalErr)},
		{"SoftErrorApplyChanges", SoftErrorApplyChanges(originalErr)},
		{"SoftErrorTags", SoftErrorTags(originalErr)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// All soft errors should be identifiable as SoftError
			assert.True(t, errors.Is(tc.err, SoftError))

			// The error message should contain the original error
			assert.Contains(t, tc.err.Error(), "original error")
		})
	}
}
