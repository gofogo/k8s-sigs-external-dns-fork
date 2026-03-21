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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source/informers"
)

// EndpointTargetsFromServices retrieves endpoint targets from services in a given namespace
// that match the specified selector. It returns external IPs or load balancer addresses.
//
// TODO: add support for service.Spec.Ports (type NodePort) and service.Spec.ClusterIPs (type ClusterIP)
func EndpointTargetsFromServices(svcInformer coreinformers.ServiceInformer, namespace string, selector map[string]string) (endpoint.Targets, error) {
	targets := endpoint.Targets{}

	var services []*corev1.Service

	if len(selector) == 0 {
		// Empty selector matches all services; use the lister (no index key to query).
		all, err := svcInformer.Lister().Services(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list services in namespace %q: %w", namespace, err)
		}
		services = all
	} else {
		// Pick any single k=v entry from the selector and use the per-entry index to
		// retrieve candidate services. The index stores one entry per k=v pair in each
		// service's spec.selector, so this returns all services that contain the queried
		// pair — a strict superset of the final match set. MatchesServiceSelector then
		// handles multi-pair selectors by verifying the remaining pairs.
		var firstEntry string
		for k, v := range selector {
			firstEntry = k + "=" + v
			break
		}
		objs, err := svcInformer.Informer().GetIndexer().ByIndex(informers.IndexWithSpecSelectorEntry, firstEntry)
		if err != nil {
			return nil, fmt.Errorf("failed to look up services by selector entry %q in namespace %q: %w", firstEntry, namespace, err)
		}
		services = make([]*corev1.Service, 0, len(objs))
		for _, obj := range objs {
			svc, ok := obj.(*corev1.Service)
			if !ok {
				continue
			}
			services = append(services, svc)
		}
	}

	for _, service := range services {
		if !MatchesServiceSelector(selector, service.Spec.Selector) {
			continue
		}

		if len(service.Spec.ExternalIPs) > 0 {
			targets = append(targets, service.Spec.ExternalIPs...)
			continue
		}

		for _, lb := range service.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				targets = append(targets, lb.IP)
			} else if lb.Hostname != "" {
				targets = append(targets, lb.Hostname)
			}
		}
	}
	return endpoint.NewTargets(targets...), nil
}
