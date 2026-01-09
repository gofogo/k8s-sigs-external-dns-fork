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
	"sort"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/external-dns/endpoint"
)

// SortEndpointTargets sorts the targets of all endpoints in-place for consistent output.
func SortEndpointTargets(endpoints []*endpoint.Endpoint) {
	for _, ep := range endpoints {
		sort.Sort(ep.Targets)
	}
}

// CheckAndLogEmptyEndpoints checks if the endpoint list is empty and logs
// a debug message if so. Returns true if empty, false otherwise.
func CheckAndLogEmptyEndpoints(
	endpoints []*endpoint.Endpoint,
	resourceType, namespace, name string,
) bool {
	if len(endpoints) == 0 {
		log.Debugf(
			"No endpoints could be generated from %s %s/%s",
			resourceType, namespace, name,
		)
		return true
	}
	return false
}

// ExtractTargetsFromLoadBalancerIngress extracts all IPs and hostnames from
// LoadBalancer ingress status entries.
func ExtractTargetsFromLoadBalancerIngress(
	ingresses []corev1.LoadBalancerIngress,
) endpoint.Targets {
	var targets endpoint.Targets
	for _, lb := range ingresses {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		}
		if lb.Hostname != "" {
			targets = append(targets, lb.Hostname)
		}
	}
	return targets
}

// NewEndpointWithMetadata creates an endpoint with TTL, provider-specific annotations,
// and set identifier all applied. This is a convenience wrapper for the common pattern
// of creating an endpoint and setting these properties.
func NewEndpointWithMetadata(
	hostname, recordType string,
	ttl endpoint.TTL,
	providerSpecific endpoint.ProviderSpecific,
	setIdentifier string,
) *endpoint.Endpoint {
	ep := endpoint.NewEndpointWithTTL(hostname, recordType, ttl)
	ep.ProviderSpecific = providerSpecific
	ep.SetIdentifier = setIdentifier
	return ep
}
