package kubeclient

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	apiv1alpha1 "sigs.k8s.io/external-dns/apis/v1alpha1"
)

// NewCRDClientForAPIVersionKind return rest client for the given apiVersion and kind of the CRD
func NewCRDClientForAPIVersionKind(
	client kubernetes.Interface,
	kubeConfig, apiServerURL, apiVersion, kind string) (*rest.RESTClient, *runtime.Scheme, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}

	config, err := clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, nil, err
	}
	apiResourceList, err := client.Discovery().ServerResourcesForGroupVersion(groupVersion.String())
	if err != nil {
		return nil, nil, fmt.Errorf("error listing resources in GroupVersion %q: %w", groupVersion.String(), err)
	}

	var crdAPIResource *metav1.APIResource
	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Kind == kind {
			crdAPIResource = &apiResource
			break
		}
	}
	if crdAPIResource == nil {
		return nil, nil, fmt.Errorf("unable to find Resource Kind %q in GroupVersion %q", kind, apiVersion)
	}

	scheme := runtime.NewScheme()
	_ = apiv1alpha1.AddToScheme(scheme)

	config.GroupVersion = &groupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}

	crdClient, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, nil, err
	}
	return crdClient, scheme, nil
}
