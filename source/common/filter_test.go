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
	"testing"
)

func TestShouldProcessResource(t *testing.T) {
	tests := []struct {
		name            string
		annotations     map[string]string
		controllerValue string
		resourceType    string
		namespace       string
		resourceName    string
		expected        bool
	}{
		{
			name:            "no controller annotation",
			annotations:     map[string]string{},
			controllerValue: "external-dns",
			resourceType:    "service",
			namespace:       "default",
			resourceName:    "my-service",
			expected:        true,
		},
		{
			name: "matching controller annotation",
			annotations: map[string]string{
				ControllerAnnotationKey: "external-dns",
			},
			controllerValue: "external-dns",
			resourceType:    "service",
			namespace:       "default",
			resourceName:    "my-service",
			expected:        true,
		},
		{
			name: "non-matching controller annotation",
			annotations: map[string]string{
				ControllerAnnotationKey: "other-controller",
			},
			controllerValue: "external-dns",
			resourceType:    "service",
			namespace:       "default",
			resourceName:    "my-service",
			expected:        false,
		},
		{
			name: "empty controller value with annotation",
			annotations: map[string]string{
				ControllerAnnotationKey: "",
			},
			controllerValue: "external-dns",
			resourceType:    "ingress",
			namespace:       "kube-system",
			resourceName:    "test-ingress",
			expected:        false,
		},
		{
			name:            "nil annotations",
			annotations:     nil,
			controllerValue: "external-dns",
			resourceType:    "service",
			namespace:       "default",
			resourceName:    "my-service",
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldProcessResource(tt.annotations, tt.controllerValue, tt.resourceType, tt.namespace, tt.resourceName)
			if result != tt.expected {
				t.Errorf("ShouldProcessResource() = %v, want %v", result, tt.expected)
			}
		})
	}
}
