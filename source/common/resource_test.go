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

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source/annotations"
)

func init() {
	// Initialize annotation keys
	annotations.SetAnnotationPrefix(annotations.DefaultAnnotationPrefix)
}

func TestBuildResourceIdentifier(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		namespace    string
		resourceName string
		expected     string
	}{
		{
			name:         "service resource",
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expected:     "service/default/my-service",
		},
		{
			name:         "ingress resource",
			resourceType: "ingress",
			namespace:    "kube-system",
			resourceName: "test-ingress",
			expected:     "ingress/kube-system/test-ingress",
		},
		{
			name:         "empty namespace",
			resourceType: "node",
			namespace:    "",
			resourceName: "node-1",
			expected:     "node//node-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildResourceIdentifier(tt.resourceType, tt.namespace, tt.resourceName)
			if result != tt.expected {
				t.Errorf("BuildResourceIdentifier() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetTTLForResource(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		resourceType string
		namespace    string
		resourceName string
		expectedTTL  endpoint.TTL
	}{
		{
			name: "with TTL annotation",
			annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/ttl": "300",
			},
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expectedTTL:  endpoint.TTL(300),
		},
		{
			name:         "without TTL annotation",
			annotations:  map[string]string{},
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expectedTTL:  endpoint.TTL(0),
		},
		{
			name:         "nil annotations",
			annotations:  nil,
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expectedTTL:  endpoint.TTL(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTTLForResource(tt.annotations, tt.resourceType, tt.namespace, tt.resourceName)
			if result != tt.expectedTTL {
				t.Errorf("GetTTLForResource() = %v, want %v", result, tt.expectedTTL)
			}
		})
	}
}
