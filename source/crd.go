/*
Copyright 2018 The Kubernetes Authors.

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
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/external-dns/source/types"

	"sigs.k8s.io/external-dns/source/annotations"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/crd"
	"sigs.k8s.io/external-dns/pkg/events"
)

// crdSource is an implementation of Source that provides endpoints by listing
// specified CRD and fetching Endpoints embedded in Spec.
type crdSource struct {
	client           crd.DNSEndpointClient
	annotationFilter string
	labelSelector    labels.Selector
	informer         cache.SharedInformer
}

// NewCRDSource creates a new crdSource with the given config.
// Parameters:
//   - client: DNSEndpointClient for accessing DNSEndpoint CRDs
//   - annotationFilter: Filter for annotation-based selection
//   - labelSelector: Selector for label-based filtering
//   - startInformer: Whether to start an informer for watching CRD changes
func NewCRDSource(
	client crd.DNSEndpointClient,
	annotationFilter string,
	labelSelector labels.Selector,
	startInformer bool) (Source, error) {
	sourceCrd := crdSource{
		client:           client,
		annotationFilter: annotationFilter,
		labelSelector:    labelSelector,
	}
	if startInformer {
		// external-dns already runs its sync-handler periodically (controlled by `--interval` flag) to ensure any
		// missed or dropped events are handled. specify resync period 0 to avoid unnecessary sync handler invocations.
		sourceCrd.informer = cache.NewSharedInformer(
			&cache.ListWatch{
				ListWithContextFunc: func(ctx context.Context, lo metav1.ListOptions) (runtime.Object, error) {
					return client.List(ctx, &lo)
				},
				WatchFuncWithContext: func(ctx context.Context, lo metav1.ListOptions) (watch.Interface, error) {
					return client.Watch(ctx, &lo)
				},
			},
			&apiv1alpha1.DNSEndpoint{},
			0)
		go sourceCrd.informer.Run(wait.NeverStop)
	}
	return &sourceCrd, nil
}

func (cs *crdSource) AddEventHandler(_ context.Context, handler func()) {
	if cs.informer != nil {
		log.Debug("Adding event handler for CRD")
		// Right now there is no way to remove event handler from informer, see:
		// https://github.com/kubernetes/kubernetes/issues/79610
		_, _ = cs.informer.AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					handler()
				},
				UpdateFunc: func(old any, newI any) {
					handler()
				},
				DeleteFunc: func(obj any) {
					handler()
				},
			},
		)
	}
}

// Endpoints returns endpoint objects.
func (cs *crdSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	endpoints := []*endpoint.Endpoint{}

	var (
		result *apiv1alpha1.DNSEndpointList
		err    error
	)

	result, err = cs.client.List(ctx, &metav1.ListOptions{LabelSelector: cs.labelSelector.String()})
	if err != nil {
		return nil, err
	}

	itemPtrs := make([]*apiv1alpha1.DNSEndpoint, len(result.Items))
	for i := range result.Items {
		itemPtrs[i] = &result.Items[i]
	}

	filtered, err := annotations.Filter(itemPtrs, cs.annotationFilter)
	if err != nil {
		return nil, err
	}

	for _, dnsEndpoint := range filtered {
		var crdEndpoints []*endpoint.Endpoint
		for _, ep := range dnsEndpoint.Spec.Endpoints {
			if (ep.RecordType == endpoint.RecordTypeCNAME || ep.RecordType == endpoint.RecordTypeA || ep.RecordType == endpoint.RecordTypeAAAA) && len(ep.Targets) < 1 {
				log.Debugf("Endpoint %s with DNSName %s has an empty list of targets, allowing it to pass through for default-targets processing", dnsEndpoint.Name, ep.DNSName)
			}
			isNAPTR := ep.RecordType == endpoint.RecordTypeNAPTR
			isTXT := ep.RecordType == endpoint.RecordTypeTXT
			illegalTarget := false
			for _, target := range ep.Targets {
				hasDot := strings.HasSuffix(target, ".")
				// Skip dot validation for TXT records as they can contain arbitrary text
				if !isTXT && ((isNAPTR && !hasDot) || (!isNAPTR && hasDot)) {
					illegalTarget = true
					break
				}
			}
			if illegalTarget {
				log.Warnf("Endpoint %s/%s with DNSName %s has an illegal target format.", dnsEndpoint.Namespace, dnsEndpoint.Name, ep.DNSName)
				continue
			}

			ep.WithLabel(endpoint.ResourceLabelKey, fmt.Sprintf("crd/%s/%s", dnsEndpoint.Namespace, dnsEndpoint.Name))
			// TODO: should add tests for this
			ep.WithRefObject(events.NewObjectReference(dnsEndpoint, types.CRD))

			crdEndpoints = append(crdEndpoints, ep)
		}

		endpoints = append(endpoints, crdEndpoints...)

		// Update status to mark endpoint as accepted
		apiv1alpha1.SetAccepted(&dnsEndpoint.Status, "DNSEndpoint accepted by controller", dnsEndpoint.Generation)
		_, err = cs.client.UpdateStatus(ctx, dnsEndpoint)
		if err != nil {
			log.Warnf("Could not update Accepted condition of DNSEndpoint %s/%s: %v", dnsEndpoint.Namespace, dnsEndpoint.Name, err)
		}
	}

	return endpoints, nil
}

// NOTE: UpdateDNSEndpointStatus method removed - it violated Single Responsibility Principle
//
// Status updates are now handled by dedicated components:
// - Option 1: pkg/crd.StatusUpdater (recommended)
// - Option 2: controller.DNSEndpointStatusManager
//
// This keeps crdSource focused on its single responsibility: implementing the Source interface
// for discovering and providing DNS endpoints from DNSEndpoint CRDs.
