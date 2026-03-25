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

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/internal/testutils"
	"sigs.k8s.io/external-dns/source/types"
)

func TestParseIngress(t *testing.T) {
	tests := []struct {
		name      string
		ingress   string
		wantNS    string
		wantName  string
		wantError bool
	}{
		{
			name:      "valid namespace and name",
			ingress:   "default/test-ingress",
			wantNS:    "default",
			wantName:  "test-ingress",
			wantError: false,
		},
		{
			name:      "only name provided",
			ingress:   "test-ingress",
			wantNS:    "",
			wantName:  "test-ingress",
			wantError: false,
		},
		{
			name:      "invalid format",
			ingress:   "default/test/ingress",
			wantNS:    "",
			wantName:  "",
			wantError: true,
		},
		{
			name:      "empty string",
			ingress:   "",
			wantNS:    "",
			wantName:  "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNS, gotName, err := ParseIngress(tt.ingress)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantNS, gotNS)
			assert.Equal(t, tt.wantName, gotName)
		})
	}
}

func TestSelectorMatchesService(t *testing.T) {
	tests := []struct {
		name        string
		selector    map[string]string
		svcSelector map[string]string
		expected    bool
	}{
		{
			name:        "all key-value pairs match",
			selector:    map[string]string{"app": "nginx", "env": "prod"},
			svcSelector: map[string]string{"app": "nginx", "env": "prod"},
			expected:    true,
		},
		{
			name:        "one key-value pair does not match",
			selector:    map[string]string{"app": "nginx", "env": "prod"},
			svcSelector: map[string]string{"app": "nginx", "env": "dev"},
			expected:    false,
		},
		{
			name:        "key not present in svcSelector",
			selector:    map[string]string{"app": "nginx", "env": "prod"},
			svcSelector: map[string]string{"app": "nginx"},
			expected:    false,
		},
		{
			name:        "empty selector",
			selector:    map[string]string{},
			svcSelector: map[string]string{"app": "nginx", "env": "prod"},
			expected:    true,
		},
		{
			name:        "empty svcSelector",
			selector:    map[string]string{"app": "nginx", "env": "prod"},
			svcSelector: map[string]string{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesServiceSelector(tt.selector, tt.svcSelector)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeEndpoints_RefObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    func() []*endpoint.Endpoint
		expected func(*testing.T, []*endpoint.Endpoint)
	}{
		{
			name:  "empty input",
			input: func() []*endpoint.Endpoint { return []*endpoint.Endpoint{} },
			expected: func(t *testing.T, ep []*endpoint.Endpoint) {
				assert.Empty(t, ep)
			},
		},
		{
			name: "single endpoint",
			input: func() []*endpoint.Endpoint {
				return []*endpoint.Endpoint{
					testutils.NewEndpointWithRef("example.com", "1.2.3.4", &v1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "123"},
					}, types.Service),
				}
			},
			expected: func(t *testing.T, ep []*endpoint.Endpoint) {
				assert.Len(t, ep, 1)
				assert.Equal(t, types.Service, ep[0].RefObject().Source)
				assert.Equal(t, "foo", ep[0].RefObject().Name)
				assert.Equal(t, "123", string(ep[0].RefObject().UID))
			},
		},
		{
			name: "two endpoints merged and only single refObject preserved",
			input: func() []*endpoint.Endpoint {
				return []*endpoint.Endpoint{
					testutils.NewEndpointWithRef("a.example.com", "1.1.1.1", &v1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "123"},
					}, types.Service),
					testutils.NewEndpointWithRef("a.example.com", "1.1.1.1", &v1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "ns", UID: "345"},
					}, types.Service),
				}
			},
			expected: func(t *testing.T, ep []*endpoint.Endpoint) {
				assert.Len(t, ep, 1)
				assert.Equal(t, types.Service, ep[0].RefObject().Source)
				assert.Equal(t, "foo", ep[0].RefObject().Name)
				assert.Equal(t, "123", string(ep[0].RefObject().UID))
				assert.NotEqual(t, "345", string(ep[0].RefObject().UID))
			},
		},
		{
			name: "two endpoints not merged and two refObject preserved",
			input: func() []*endpoint.Endpoint {
				return []*endpoint.Endpoint{
					testutils.NewEndpointWithRef("a.example.com", "1.1.1.1", &v1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default", UID: "123"},
					}, types.Service),
					testutils.NewEndpointWithRef("b.example.com", "1.1.1.2", &v1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "bar", Namespace: "ns", UID: "345"},
					}, types.Service),
				}
			},
			expected: func(t *testing.T, ep []*endpoint.Endpoint) {
				assert.Len(t, ep, 2)
				assert.NotEqual(t, ep[0], ep[1])
				for _, el := range ep {
					assert.Equal(t, types.Service, el.RefObject().Source)
					assert.Contains(t, []string{"foo", "bar"}, el.RefObject().Name)
					assert.Contains(t, []string{"123", "345"}, string(el.RefObject().UID))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := endpoint.MergeEndpoints(tt.input())
			tt.expected(t, result)
		})
	}
}
