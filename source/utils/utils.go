package utils

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"sigs.k8s.io/external-dns/endpoint"
)

const (
	// CloudflareProxiedKey The annotation used for determining if traffic will go through Cloudflare
	CloudflareProxiedKey        = "external-dns.alpha.kubernetes.io/cloudflare-proxied"
	CloudflareCustomHostnameKey = "external-dns.alpha.kubernetes.io/cloudflare-custom-hostname"

	targetAnnotationKey = "external-dns.alpha.kubernetes.io/target"

	SetIdentifierKey   = "external-dns.alpha.kubernetes.io/set-identifier"
	aliasAnnotationKey = "external-dns.alpha.kubernetes.io/alias"

	ttlAnnotationKey = "external-dns.alpha.kubernetes.io/ttl"
	ttlMinimum       = 1
	ttlMaximum       = math.MaxInt32
)

// TTLFromAnnotations TODO: copied from source.go. Refactor to avoid duplication.
// TTLFromAnnotations extracts the TTL from the annotations of the given resource.
func TTLFromAnnotations(annotations map[string]string, resource string) endpoint.TTL {
	ttlNotConfigured := endpoint.TTL(0)
	ttlAnnotation, exists := annotations[ttlAnnotationKey]
	if !exists {
		return ttlNotConfigured
	}
	ttlValue, err := parseTTL(ttlAnnotation)
	if err != nil {
		log.Warnf("%s: \"%v\" is not a valid TTL value: %v", resource, ttlAnnotation, err)
		return ttlNotConfigured
	}
	if ttlValue < ttlMinimum || ttlValue > ttlMaximum {
		log.Warnf("TTL value %q must be between [%d, %d]", ttlValue, ttlMinimum, ttlMaximum)
		return ttlNotConfigured
	}
	return endpoint.TTL(ttlValue)
}

// TODO: test
// TODO: copied from source.go. Refactor to avoid duplication.
// parseTTL parses TTL from string, returning duration in seconds.
// parseTTL supports both integers like "600" and durations based
// on Go Duration like "10m", hence "600" and "10m" represent the same value.
//
// Note: for durations like "1.5s" the fraction is omitted (resulting in 1 second
// for the example).
func parseTTL(s string) (ttlSeconds int64, err error) {
	ttlDuration, errDuration := time.ParseDuration(s)
	if errDuration != nil {
		ttlInt, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, errDuration
		}
		return ttlInt, nil
	}

	return int64(ttlDuration.Seconds()), nil
}

// TODO: test
func ProviderSpecificAnnotations(annotations map[string]string) (endpoint.ProviderSpecific, string) {
	providerSpecificAnnotations := endpoint.ProviderSpecific{}

	if v, exists := annotations[CloudflareProxiedKey]; exists {
		providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
			Name:  CloudflareProxiedKey,
			Value: v,
		})
	}
	if v, exists := annotations[CloudflareCustomHostnameKey]; exists {
		providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
			Name:  CloudflareCustomHostnameKey,
			Value: v,
		})
	}
	if getAliasFromAnnotations(annotations) {
		providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
			Name:  "alias",
			Value: "true",
		})
	}
	setIdentifier := ""
	for k, v := range annotations {
		if k == SetIdentifierKey {
			setIdentifier = v
		} else if strings.HasPrefix(k, "external-dns.alpha.kubernetes.io/aws-") {
			attr := strings.TrimPrefix(k, "external-dns.alpha.kubernetes.io/aws-")
			providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
				Name:  fmt.Sprintf("aws/%s", attr),
				Value: v,
			})
		} else if strings.HasPrefix(k, "external-dns.alpha.kubernetes.io/scw-") {
			attr := strings.TrimPrefix(k, "external-dns.alpha.kubernetes.io/scw-")
			providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
				Name:  fmt.Sprintf("scw/%s", attr),
				Value: v,
			})
		} else if strings.HasPrefix(k, "external-dns.alpha.kubernetes.io/ibmcloud-") {
			attr := strings.TrimPrefix(k, "external-dns.alpha.kubernetes.io/ibmcloud-")
			providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
				Name:  fmt.Sprintf("ibmcloud-%s", attr),
				Value: v,
			})
		} else if strings.HasPrefix(k, "external-dns.alpha.kubernetes.io/webhook-") {
			// Support for wildcard annotations for webhook providers
			attr := strings.TrimPrefix(k, "external-dns.alpha.kubernetes.io/webhook-")
			providerSpecificAnnotations = append(providerSpecificAnnotations, endpoint.ProviderSpecificProperty{
				Name:  fmt.Sprintf("webhook/%s", attr),
				Value: v,
			})
		}
	}
	return providerSpecificAnnotations, setIdentifier
}

// TODO: test
func EndpointsForHostname(hostname string, targets endpoint.Targets, ttl endpoint.TTL, providerSpecific endpoint.ProviderSpecific, setIdentifier string, resource string) []*endpoint.Endpoint {
	var endpoints []*endpoint.Endpoint

	var aTargets endpoint.Targets
	var aaaaTargets endpoint.Targets
	var cnameTargets endpoint.Targets

	for _, t := range targets {
		switch suitableType(t) {
		case endpoint.RecordTypeA:
			aTargets = append(aTargets, t)
		case endpoint.RecordTypeAAAA:
			aaaaTargets = append(aaaaTargets, t)
		default:
			cnameTargets = append(cnameTargets, t)
		}
	}

	if len(aTargets) > 0 {
		epA := endpoint.NewEndpointWithTTL(hostname, endpoint.RecordTypeA, ttl, aTargets...)
		if epA != nil {
			epA.ProviderSpecific = providerSpecific
			epA.SetIdentifier = setIdentifier
			if resource != "" {
				epA.Labels[endpoint.ResourceLabelKey] = resource
			}
			endpoints = append(endpoints, epA)
		}
	}

	if len(aaaaTargets) > 0 {
		epAAAA := endpoint.NewEndpointWithTTL(hostname, endpoint.RecordTypeAAAA, ttl, aaaaTargets...)
		if epAAAA != nil {
			epAAAA.ProviderSpecific = providerSpecific
			epAAAA.SetIdentifier = setIdentifier
			if resource != "" {
				epAAAA.Labels[endpoint.ResourceLabelKey] = resource
			}
			endpoints = append(endpoints, epAAAA)
		}
	}

	if len(cnameTargets) > 0 {
		epCNAME := endpoint.NewEndpointWithTTL(hostname, endpoint.RecordTypeCNAME, ttl, cnameTargets...)
		if epCNAME != nil {
			epCNAME.ProviderSpecific = providerSpecific
			epCNAME.SetIdentifier = setIdentifier
			if resource != "" {
				epCNAME.Labels[endpoint.ResourceLabelKey] = resource
			}
			endpoints = append(endpoints, epCNAME)
		}
	}

	return endpoints
}

func getAliasFromAnnotations(annotations map[string]string) bool {
	aliasAnnotation, exists := annotations[aliasAnnotationKey]
	return exists && aliasAnnotation == "true"
}

// suitableType returns the DNS resource record type suitable for the target.
// In this case type A/AAAA for IPs and type CNAME for everything else.
func suitableType(target string) string {
	netIP, err := netip.ParseAddr(target)
	if err == nil && netIP.Is4() {
		return endpoint.RecordTypeA
	} else if err == nil && netIP.Is6() {
		return endpoint.RecordTypeAAAA
	}
	return endpoint.RecordTypeCNAME
}

// TODO: test
func ParseIngress(ingress string) (namespace, name string, err error) {
	parts := strings.Split(ingress, "/")
	if len(parts) == 2 {
		namespace, name = parts[0], parts[1]
	} else if len(parts) == 1 {
		name = parts[0]
	} else {
		err = fmt.Errorf("invalid ingress name (name or namespace/name) found %q", ingress)
	}

	return
}

// TODO: test
func SelectorMatchesServiceSelector(selector, svcSelector map[string]string) bool {
	for k, v := range selector {
		if lbl, ok := svcSelector[k]; !ok || lbl != v {
			return false
		}
	}
	return true
}

// TargetsFromTargetAnnotation gets endpoints from optional "target" annotation.
// Returns empty endpoints array if none are found.
func TargetsFromTargetAnnotation(annotations map[string]string) endpoint.Targets {
	var targets endpoint.Targets

	// Get the desired hostname of the ingress from the annotation.
	targetAnnotation, exists := annotations[targetAnnotationKey]
	if exists && targetAnnotation != "" {
		// splits the hostname annotation and removes the trailing periods
		targetsList := strings.Split(strings.Replace(targetAnnotation, " ", "", -1), ",")
		for _, targetHostname := range targetsList {
			targetHostname = strings.TrimSuffix(targetHostname, ".")
			targets = append(targets, targetHostname)
		}
	}
	return targets
}

func EndpointTargetsFromServices(svcInformer coreinformers.ServiceInformer, namespace string, selector map[string]string) endpoint.Targets {
	targets := endpoint.Targets{}

	services, err := svcInformer.Lister().Services(namespace).List(labels.Everything())
	if err != nil {
		log.Error(err)
		return nil
	}

	for _, service := range services {
		if !SelectorMatchesServiceSelector(selector, service.Spec.Selector) {
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
	return targets
}

// ParseAnnotationFilter parses an annotation filter string into a labels.Selector.
// Returns nil if the annotation filter is invalid.
func ParseAnnotationFilter(annotationFilter string) (labels.Selector, error) {
	labelSelector, err := metav1.ParseToLabelSelector(annotationFilter)
	if err != nil {
		return nil, err
	}
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	return selector, nil
}
