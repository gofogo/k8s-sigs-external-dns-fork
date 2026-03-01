/*
Copyright 2026 The Kubernetes Authors.

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

package endpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockObjectMetaAccessor struct {
	namespace string
	name      string
}

func (m *mockObjectMetaAccessor) GetObjectMeta() metav1.Object {
	return &metav1.ObjectMeta{
		Namespace: m.namespace,
		Name:      m.name,
	}
}

func TestSuitableType(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{
			name:     "valid IPv4 address",
			target:   "192.168.1.1",
			expected: RecordTypeA,
		},
		{
			name:     "valid IPv6 address",
			target:   "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: RecordTypeAAAA,
		},
		{
			name:     "invalid IP address, should return CNAME",
			target:   "example.com",
			expected: RecordTypeCNAME,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuitableType(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasEmptyEndpoints(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []*Endpoint
		rType     string
		entity    metav1.ObjectMetaAccessor
		expected  bool
	}{
		{
			name:      "nil endpoints returns true",
			endpoints: nil,
			rType:     "Service",
			entity:    &mockObjectMetaAccessor{namespace: "default", name: "my-service"},
			expected:  true,
		},
		{
			name:      "empty slice returns true",
			endpoints: []*Endpoint{},
			rType:     "Ingress",
			entity:    &mockObjectMetaAccessor{namespace: "kube-system", name: "my-ingress"},
			expected:  true,
		},
		{
			name: "single endpoint returns false",
			endpoints: []*Endpoint{
				NewEndpoint("example.org", "A", "1.2.3.4"),
			},
			rType:    "Service",
			entity:   &mockObjectMetaAccessor{namespace: "default", name: "my-service"},
			expected: false,
		},
		{
			name: "multiple endpoints returns false",
			endpoints: []*Endpoint{
				NewEndpoint("example.org", "A", "1.2.3.4"),
				NewEndpoint("test.example.org", "CNAME", "example.org"),
			},
			rType:    "Ingress",
			entity:   &mockObjectMetaAccessor{namespace: "production", name: "frontend"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := HasNoEmptyEndpoints(tc.endpoints, tc.rType, tc.entity)
			assert.Equal(t, tc.expected, result)
			// TODO: Add log capture and verification
		})
	}
}
