package istio

import (
	"context"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/external-dns/endpoint"

	"sigs.k8s.io/external-dns/source/utils"
)

// IstioGatewayIngressSource is the annotation used to determine if the gateway is implemented by an Ingress object
// instead of a standard LoadBalancer service type
const IstioGatewayIngressSource = "external-dns.alpha.kubernetes.io/ingress"

type Gateway struct {
	kubeClient kubernetes.Interface
}

// NewGateway creates a new Gateway source
func NewGateway(kubeClient kubernetes.Interface) Gateway {
	return Gateway{
		kubeClient: kubeClient,
	}
}

func (sc *Gateway) EndpointsFromGateway(_ context.Context, hostnames []string, annotations map[string]string, resource string, targets endpoint.Targets) ([]*endpoint.Endpoint, error) {
	var endpoints []*endpoint.Endpoint

	ttl := utils.TTLFromAnnotations(annotations, resource)
	providerSpecific, setIdentifier := utils.ProviderSpecificAnnotations(annotations)

	for _, host := range hostnames {
		endpoints = append(endpoints, utils.EndpointsForHostname(host, targets, ttl, providerSpecific, setIdentifier, resource)...)
	}
	return endpoints, nil
}

// targetsFromIngress extracts targets from an Ingress annotation
func (sc *Gateway) TargetsFromIngress(ctx context.Context, ingressStr string, namespace string) (endpoint.Targets, error) {
	ingress, err := sc.kubeClient.NetworkingV1().Ingresses(namespace).Get(ctx, ingressStr, metav1.GetOptions{})
	if err != nil {
		log.Error(err)
		return nil, err
	}

	var targets endpoint.Targets
	for _, lb := range ingress.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		} else if lb.Hostname != "" {
			targets = append(targets, lb.Hostname)
		}
	}
	return targets, nil
}
