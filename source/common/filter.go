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

package common

import (
	log "github.com/sirupsen/logrus"
)

const (
	// ControllerAnnotationKey is the annotation key used to identify which controller should process a resource
	ControllerAnnotationKey = "external-dns.alpha.kubernetes.io/controller"
)

// ShouldProcessResource checks if a resource should be processed based on its controller annotation.
// Returns false if the controller annotation exists but doesn't match the expected value.
// This ensures that only resources intended for this controller are processed.
func ShouldProcessResource(
	resourceAnnotations map[string]string,
	controllerValue string,
	resourceType, namespace, name string,
) bool {
	controller, ok := resourceAnnotations[ControllerAnnotationKey]
	if ok && controller != controllerValue {
		log.Debugf(
			"Skipping %s %s/%s because controller value does not match, found: %s, required: %s",
			resourceType, namespace, name, controller, controllerValue,
		)
		return false
	}
	return true
}
