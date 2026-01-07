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
	"testing"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/external-dns/plan"
)

// This file previously contained tests for the old dnsEndpointStatusUpdater interface.
//
// The status update mechanism has been replaced with a callback-based approach.
// See dnsendpoint_status.go for details.
//
// TODO: Add tests for the new callback-based status update mechanism:
// - Test RegisterStatusUpdateCallback()
// - Test invokeStatusUpdateCallbacks()
// - Test crdSource.UpdateDNSEndpointStatuses()

// TestCallbackRegistration tests that callbacks can be registered and invoked
func TestCallbackRegistration(t *testing.T) {
	ctrl := &Controller{
		UpdateDNSEndpointStatus: true,
	}

	callbackInvoked := false
	var receivedChanges *plan.Changes
	var receivedSuccess bool
	var receivedMessage string

	// Register a test callback
	ctrl.RegisterStatusUpdateCallback(func(ctx context.Context, changes *plan.Changes, success bool, message string) {
		callbackInvoked = true
		receivedChanges = changes
		receivedSuccess = success
		receivedMessage = message
	})

	// Invoke callbacks
	testChanges := &plan.Changes{}
	ctrl.invokeStatusUpdateCallbacks(context.Background(), testChanges, true, "test message")

	// Verify callback was invoked with correct parameters
	assert.True(t, callbackInvoked, "Callback should have been invoked")
	assert.Equal(t, testChanges, receivedChanges, "Should receive correct changes")
	assert.True(t, receivedSuccess, "Should receive correct success flag")
	assert.Equal(t, "test message", receivedMessage, "Should receive correct message")
}

// TestMultipleCallbacks tests that multiple callbacks can be registered and all are invoked
func TestMultipleCallbacks(t *testing.T) {
	ctrl := &Controller{
		UpdateDNSEndpointStatus: true,
	}

	invocationCount := 0

	// Register multiple callbacks
	ctrl.RegisterStatusUpdateCallback(func(ctx context.Context, changes *plan.Changes, success bool, message string) {
		invocationCount++
	})
	ctrl.RegisterStatusUpdateCallback(func(ctx context.Context, changes *plan.Changes, success bool, message string) {
		invocationCount++
	})
	ctrl.RegisterStatusUpdateCallback(func(ctx context.Context, changes *plan.Changes, success bool, message string) {
		invocationCount++
	})

	// Invoke callbacks
	ctrl.invokeStatusUpdateCallbacks(context.Background(), &plan.Changes{}, true, "test")

	// Verify all callbacks were invoked
	assert.Equal(t, 3, invocationCount, "All 3 callbacks should have been invoked")
}
