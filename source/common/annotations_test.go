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
	"reflect"
	"testing"

	"sigs.k8s.io/external-dns/source/annotations"
)

func init() {
	// Initialize annotation keys
	annotations.SetAnnotationPrefix(annotations.DefaultAnnotationPrefix)
}

func TestGetHostnamesFromAnnotations(t *testing.T) {
	tests := []struct {
		name                     string
		annotations              map[string]string
		ignoreHostnameAnnotation bool
		expected                 []string
	}{
		{
			name: "with hostname annotation, not ignored",
			annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": "example.com,test.com",
			},
			ignoreHostnameAnnotation: false,
			expected:                 []string{"example.com", "test.com"},
		},
		{
			name: "with hostname annotation, but ignored",
			annotations: map[string]string{
				"external-dns.alpha.kubernetes.io/hostname": "example.com,test.com",
			},
			ignoreHostnameAnnotation: true,
			expected:                 nil,
		},
		{
			name:                     "without hostname annotation",
			annotations:              map[string]string{},
			ignoreHostnameAnnotation: false,
			expected:                 nil,
		},
		{
			name:                     "nil annotations",
			annotations:              nil,
			ignoreHostnameAnnotation: false,
			expected:                 nil,
		},
		{
			name:                     "ignore flag true with nil annotations",
			annotations:              nil,
			ignoreHostnameAnnotation: true,
			expected:                 nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHostnamesFromAnnotations(tt.annotations, tt.ignoreHostnameAnnotation)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetHostnamesFromAnnotations() = %v, want %v", result, tt.expected)
			}
		})
	}
}
