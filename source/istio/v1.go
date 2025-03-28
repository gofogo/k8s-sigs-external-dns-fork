package istio

import (
	"context"
	"fmt"

	networkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source/utils"
)

func FilterByAnnotationsV1(annotationFilter string, gateways []*networkingv1.Gateway) ([]*networkingv1.Gateway, error) {
	selector, err := utils.ParseAnnotationFilter(annotationFilter)
	if err != nil {
		return nil, err
	}

	// empty filter returns original list
	if selector.Empty() {
		return gateways, nil
	}

	var filteredList []*networkingv1.Gateway

	for _, gw := range gateways {
		// convert the annotations to an equivalent label selector
		annotations := labels.Set(gw.Annotations)

		// include if the annotations match the selector
		if selector.Matches(annotations) {
			filteredList = append(filteredList, gw)
		}
	}

	return filteredList, nil
}

func TargetsFromGatewayV1(ctx context.Context, gw Gateway, svcInformer coreinformers.ServiceInformer, gateway *networkingv1.Gateway) (endpoint.Targets, error) {
	ingressStr, ok := gateway.Annotations[IstioGatewayIngressSource]
	if ok && ingressStr != "" {
		return targetsFromIngressV1(ctx, gw, ingressStr, gateway)
	}
	return utils.EndpointTargetsFromServices(svcInformer, gateway.Namespace, gateway.Spec.Selector), nil
}

func targetsFromIngressV1(ctx context.Context, gw Gateway, ingressStr string, gateway *networkingv1.Gateway) (endpoint.Targets, error) {
	namespace, name, err := gwNameWithNamespaceV1(ingressStr, gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ingress annotation on Gateway (%s/%s): %w", gateway.Namespace, gateway.Name, err)
	}
	return gw.TargetsFromIngress(ctx, name, namespace)
}

func gwNameWithNamespaceV1(ingressStr string, gateway *networkingv1.Gateway) (string, string, error) {
	namespace, name, err := utils.ParseIngress(ingressStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse Ingress annotation on Gateway (%s/%s): %w", gateway.Namespace, gateway.Name, err)
	}
	if namespace == "" {
		namespace = gateway.Namespace
	}

	return name, namespace, nil
}
