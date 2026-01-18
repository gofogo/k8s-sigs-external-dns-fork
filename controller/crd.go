package controller

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/crd"
	"sigs.k8s.io/external-dns/pkg/events"
	"sigs.k8s.io/external-dns/plan"
)

func syncFailedStatus(cl crd.DNSEndpointClient, ch plan.Changes, reason apiv1alpha1.ConditionReason) {
	if cl == nil {
		return
	}
	for _, ep := range ch.Create {
		obj := ep.RefObject()
		if obj.Kind == "DNSEndpoint" {
			fmt.Print("updating status for ", obj.Name)
		}
	}
	for _, ep := range ch.UpdateNew {
		obj := ep.RefObject()
		if obj.Kind == "DNSEndpoint" {
			fmt.Print("updating status for ", obj.Name)
		}
	}
}

type refKey struct {
	Namespace string
	Name      string
}

type refGroup struct {
	Ref       *events.ObjectReference
	Endpoints []*endpoint.Endpoint
}

func syncStatus(cl crd.DNSEndpointClient, ep []*endpoint.Endpoint, reason apiv1alpha1.ConditionReason) {
	fmt.Println("syncStatus (33)")
	ctx := context.TODO()
	if cl == nil {
		return
	}
	crds := make(map[refKey]*refGroup)
	for _, e := range ep {
		ref := e.RefObject()
		if ref.Kind == "DNSEndpoint" {
			key := refKey{Namespace: ref.Namespace, Name: ref.Name}
			g, ok := crds[key]
			if !ok || g == nil {
				g = &refGroup{Ref: ref}
				crds[key] = g
			}
			g.Endpoints = append(g.Endpoints, e)
		}
	}
	for key, _ := range crds {
		dnsEndpoint, err := cl.Get(ctx, key.Namespace, key.Name)
		if err != nil {
			log.Warnf("Could not get the CRD %s/%s: %v", key.Namespace, key.Name, err)
			continue
		}
		if dnsEndpoint.Status.ObservedGeneration == dnsEndpoint.Generation {
			continue
		}

		// TODO: provide as well all TXT records info
		// Accepted the DNSEndpoint by updating its status, not making changes to .Status.ObservedGeneration yet
		apiv1alpha1.SetProgrammed(
			dnsEndpoint,
			fmt.Sprintf("All (%d) records successfully provisioned", len(dnsEndpoint.Spec.Endpoints)))

		// dnsEndpoint.Status.ObservedGeneration = dnsEndpoint.Generation
		// Update the ObservedGeneration
		_, err = cl.UpdateStatus(ctx, dnsEndpoint)
		if err != nil {
			log.Warnf("Could not update status of the CRD: %v", err)
		}
	}
}
