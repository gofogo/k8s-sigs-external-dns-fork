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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestSortEndpointTargets(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []*endpoint.Endpoint
		expected  []*endpoint.Endpoint
	}{
		{
			name: "unsorted targets",
			endpoints: []*endpoint.Endpoint{
				{
					DNSName: "example.com",
					Targets: endpoint.Targets{"c.example.com", "a.example.com", "b.example.com"},
				},
			},
			expected: []*endpoint.Endpoint{
				{
					DNSName: "example.com",
					Targets: endpoint.Targets{"a.example.com", "b.example.com", "c.example.com"},
				},
			},
		},
		{
			name: "multiple endpoints",
			endpoints: []*endpoint.Endpoint{
				{
					DNSName: "example.com",
					Targets: endpoint.Targets{"192.168.2.1", "192.168.1.1"},
				},
				{
					DNSName: "test.com",
					Targets: endpoint.Targets{"z.test.com", "a.test.com"},
				},
			},
			expected: []*endpoint.Endpoint{
				{
					DNSName: "example.com",
					Targets: endpoint.Targets{"192.168.1.1", "192.168.2.1"},
				},
				{
					DNSName: "test.com",
					Targets: endpoint.Targets{"a.test.com", "z.test.com"},
				},
			},
		},
		{
			name:      "empty endpoints",
			endpoints: []*endpoint.Endpoint{},
			expected:  []*endpoint.Endpoint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortEndpointTargets(tt.endpoints)
			if !reflect.DeepEqual(tt.endpoints, tt.expected) {
				t.Errorf("SortEndpointTargets() = %v, want %v", tt.endpoints, tt.expected)
			}
		})
	}
}

func TestCheckAndLogEmptyEndpoints(t *testing.T) {
	tests := []struct {
		name         string
		endpoints    []*endpoint.Endpoint
		resourceType string
		namespace    string
		resourceName string
		expected     bool
	}{
		{
			name:         "empty endpoints",
			endpoints:    []*endpoint.Endpoint{},
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expected:     true,
		},
		{
			name: "non-empty endpoints",
			endpoints: []*endpoint.Endpoint{
				{DNSName: "example.com"},
			},
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expected:     false,
		},
		{
			name:         "nil endpoints",
			endpoints:    nil,
			resourceType: "service",
			namespace:    "default",
			resourceName: "my-service",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckAndLogEmptyEndpoints(tt.endpoints, tt.resourceType, tt.namespace, tt.resourceName)
			if result != tt.expected {
				t.Errorf("CheckAndLogEmptyEndpoints() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractTargetsFromLoadBalancerIngress(t *testing.T) {
	tests := []struct {
		name      string
		ingresses []corev1.LoadBalancerIngress
		expected  endpoint.Targets
	}{
		{
			name: "IPs and hostnames",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "192.168.1.1"},
				{Hostname: "lb.example.com"},
				{IP: "192.168.1.2", Hostname: "lb2.example.com"},
			},
			expected: endpoint.Targets{"192.168.1.1", "lb.example.com", "192.168.1.2", "lb2.example.com"},
		},
		{
			name: "only IPs",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "192.168.1.1"},
				{IP: "192.168.1.2"},
			},
			expected: endpoint.Targets{"192.168.1.1", "192.168.1.2"},
		},
		{
			name: "only hostnames",
			ingresses: []corev1.LoadBalancerIngress{
				{Hostname: "lb1.example.com"},
				{Hostname: "lb2.example.com"},
			},
			expected: endpoint.Targets{"lb1.example.com", "lb2.example.com"},
		},
		{
			name:      "empty ingresses",
			ingresses: []corev1.LoadBalancerIngress{},
			expected:  nil,
		},
		{
			name: "empty IP and hostname fields",
			ingresses: []corev1.LoadBalancerIngress{
				{IP: "", Hostname: ""},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTargetsFromLoadBalancerIngress(tt.ingresses)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractTargetsFromLoadBalancerIngress() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewEndpointWithMetadata(t *testing.T) {
	tests := []struct {
		name             string
		hostname         string
		recordType       string
		ttl              endpoint.TTL
		providerSpecific endpoint.ProviderSpecific
		setIdentifier    string
	}{
		{
			name:       "basic endpoint",
			hostname:   "example.com",
			recordType: "A",
			ttl:        endpoint.TTL(300),
			providerSpecific: endpoint.ProviderSpecific{
				{Name: "aws/weight", Value: "10"},
			},
			setIdentifier: "test-id",
		},
		{
			name:             "endpoint without provider specific",
			hostname:         "test.com",
			recordType:       "CNAME",
			ttl:              endpoint.TTL(600),
			providerSpecific: nil,
			setIdentifier:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewEndpointWithMetadata(tt.hostname, tt.recordType, tt.ttl, tt.providerSpecific, tt.setIdentifier)

			if result.DNSName != tt.hostname {
				t.Errorf("DNSName = %v, want %v", result.DNSName, tt.hostname)
			}
			if result.RecordType != tt.recordType {
				t.Errorf("RecordType = %v, want %v", result.RecordType, tt.recordType)
			}
			if result.RecordTTL != tt.ttl {
				t.Errorf("RecordTTL = %v, want %v", result.RecordTTL, tt.ttl)
			}
			if !reflect.DeepEqual(result.ProviderSpecific, tt.providerSpecific) {
				t.Errorf("ProviderSpecific = %v, want %v", result.ProviderSpecific, tt.providerSpecific)
			}
			if result.SetIdentifier != tt.setIdentifier {
				t.Errorf("SetIdentifier = %v, want %v", result.SetIdentifier, tt.setIdentifier)
			}
		})
	}
}
