package source

import (
	"context"
	"fmt"
	"strings"

	// log "github.com/sirupsen/logrus"
	// "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source/utils"

	"sigs.k8s.io/external-dns/source/istio"

	networkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

// endpointsFromGatewayConfig extracts the endpoints from an Istio Gateway Config object
func (sc *gatewaySource) endpointsFromGatewayV1Alpha3(ctx context.Context, hostnames []string, gw *networkingv1alpha3.Gateway) ([]*endpoint.Endpoint, error) {
	resource := fmt.Sprintf("gateway/%s/%s", gw.Namespace, gw.Name)

	annotations := gw.Annotations

	targets := utils.TargetsFromTargetAnnotation(annotations)
	if len(targets) == 0 {
		var err error
		targets, err = istio.TargetsFromGatewayV1Alpha3(ctx, sc.gateway, sc.serviceInformer, gw)
		if err != nil {
			return nil, err
		}
	}

	return sc.gateway.EndpointsFromGateway(ctx, hostnames, annotations, resource, targets)
}

func (sc *gatewaySource) endpointsFromGatewayV1(ctx context.Context, hostnames []string, gw *networkingv1.Gateway) ([]*endpoint.Endpoint, error) {
	resource := fmt.Sprintf("gateway/%s/%s", gw.Namespace, gw.Name)
	annotations := gw.Annotations

	targets := utils.TargetsFromTargetAnnotation(annotations)
	if len(targets) == 0 {
		var err error
		targets, err = istio.TargetsFromGatewayV1(ctx, sc.gateway, sc.serviceInformer, gw)
		if err != nil {
			return nil, err
		}
	}
	return sc.gateway.EndpointsFromGateway(ctx, hostnames, annotations, resource, targets)
}

func (sc *gatewaySource) hostNamesFromGatewayV1Alpha3(gateway *networkingv1alpha3.Gateway) ([]string, error) {
	var hostnames []string
	for _, server := range gateway.Spec.Servers {
		for _, host := range server.Hosts {
			if host == "" {
				continue
			}

			parts := strings.Split(host, "/")

			// If the input hostname is of the form my-namespace/foo.bar.com, remove the namespace
			// before appending it to the list of endpoints to create
			if len(parts) == 2 {
				host = parts[1]
			}

			if host != "*" {
				hostnames = append(hostnames, host)
			}
		}
	}

	if !sc.ignoreHostnameAnnotation {
		hostnames = append(hostnames, getHostnamesFromAnnotations(gateway.Annotations)...)
	}

	return hostnames, nil
}

func (sc *gatewaySource) hostNamesFromGatewayV1(gateway *networkingv1.Gateway) ([]string, error) {
	var hostnames []string
	for _, server := range gateway.Spec.Servers {
		for _, host := range server.Hosts {
			if host == "" {
				continue
			}

			parts := strings.Split(host, "/")

			// If the input hostname is of the form my-namespace/foo.bar.com, remove the namespace
			// before appending it to the list of endpoints to create
			if len(parts) == 2 {
				host = parts[1]
			}

			if host != "*" {
				hostnames = append(hostnames, host)
			}
		}
	}

	if !sc.ignoreHostnameAnnotation {
		hostnames = append(hostnames, getHostnamesFromAnnotations(gateway.Annotations)...)
	}

	return hostnames, nil
}
