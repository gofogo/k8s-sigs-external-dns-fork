package istio

import (
	"context"
	"fmt"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"sigs.k8s.io/external-dns/endpoint"

	"sigs.k8s.io/external-dns/source/utils"
)

// filterByAnnotationsV1Alpha3 filters a list of configs by a given annotation selector.
func FilterByAnnotationsV1Alpha3(annotationFilter string, gateways []*networkingv1alpha3.Gateway) ([]*networkingv1alpha3.Gateway, error) {
	selector, err := utils.ParseAnnotationFilter(annotationFilter)
	if err != nil {
		return nil, err
	}

	// empty filter returns original list
	if selector.Empty() {
		return gateways, nil
	}

	var filteredList []*networkingv1alpha3.Gateway

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

func TargetsFromGatewayV1Alpha3(ctx context.Context, gw Gateway, svcInformer coreinformers.ServiceInformer, gateway *networkingv1alpha3.Gateway) (endpoint.Targets, error) {
	ingressStr, ok := gateway.Annotations[IstioGatewayIngressSource]
	if ok && ingressStr != "" {
		return targetsFromIngressV1Alpha3(ctx, gw, ingressStr, gateway)
	}
	return utils.EndpointTargetsFromServices(svcInformer, gateway.Namespace, gateway.Spec.Selector), nil
}

func targetsFromIngressV1Alpha3(ctx context.Context, gw Gateway, ingressStr string, gateway *networkingv1alpha3.Gateway) (endpoint.Targets, error) {
	namespace, name, err := gwNamespaceWithNameV1Alpha3(ingressStr, gateway)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Ingress annotation on Gateway (%s/%s): %w", gateway.Namespace, gateway.Name, err)
	}
	return gw.TargetsFromIngress(ctx, name, namespace)
}

func gwNamespaceWithNameV1Alpha3(ingressStr string, gateway *networkingv1alpha3.Gateway) (string, string, error) {
	namespace, name, err := utils.ParseIngress(ingressStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse Ingress annotation on Gateway (%s/%s): %w", gateway.Namespace, gateway.Name, err)
	}
	if namespace == "" {
		namespace = gateway.Namespace
	}

	return name, namespace, nil
}
