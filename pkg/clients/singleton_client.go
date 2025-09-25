package clients

import (
	"sync"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	openshift "github.com/openshift/client-go/route/clientset/versioned"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/dynamic"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
)

// SingletonClientGenerator stores provider clients and guarantees that only one instance of each client
// will be generated throughout the application lifecycle.
//
// Thread Safety: Uses sync.Once for each client type to ensure thread-safe initialization.
// This is important because external-dns may create multiple sources concurrently.
//
// Memory Efficiency: Prevents creating multiple instances of expensive client objects
// that maintain their own connection pools and caches.
//
// Configuration: Clients are configured using KubeConfig, APIServerURL, and RequestTimeout
// which are set during SingletonClientGenerator initialization.
type SingletonClientGenerator struct {
	KubeConfig      string
	APIServerURL    string
	RequestTimeout  time.Duration
	kubeClient      kubernetes.Interface
	gatewayClient   gateway.Interface
	istioClient     *istioclient.Clientset
	cfClient        *cfclient.Client
	dynKubeClient   dynamic.Interface
	openshiftClient openshift.Interface
	kubeOnce        sync.Once
	gatewayOnce     sync.Once
	istioOnce       sync.Once
	cfOnce          sync.Once
	dynCliOnce      sync.Once
	openshiftOnce   sync.Once
}
