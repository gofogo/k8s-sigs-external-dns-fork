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

package clients

import (
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	extdnshttp "sigs.k8s.io/external-dns/pkg/http"
)

// NewKubeClient returns a new Kubernetes client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func NewKubeClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*kubernetes.Clientset, error) {
	log.Infof("Instantiating new Kubernetes client")
	config, err := InstrumentedRESTConfig(kubeConfig, apiServerURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	log.Infof("Created Kubernetes client %s", config.Host)
	return client, nil
}

// GetRestConfig returns the REST client configuration for Kubernetes API access.
// Supports both in-cluster and external cluster configurations.
//
// Configuration Priority:
// 1. If kubeConfig is empty, tries the recommended home file (~/.kube/config)
// 2. If kubeConfig is still empty, uses in-cluster service account
// 3. Otherwise, uses the specified kubeConfig file
//
// API Server Override: The apiServerURL parameter can override the server URL
// from the kubeconfig file, useful for proxy scenarios or custom endpoints.
func GetRestConfig(kubeConfig, apiServerURL string) (*rest.Config, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}
	log.Debugf("apiServerURL: %s", apiServerURL)
	log.Debugf("kubeConfig: %s", kubeConfig)

	// evaluate whether to use kubeConfig-file or service-account-token
	var (
		config *rest.Config
		err    error
	)
	if kubeConfig == "" {
		log.Infof("Using inCluster-config based on serviceaccount-token")
		config, err = rest.InClusterConfig()
	} else {
		log.Infof("Using kubeConfig")
		config, err = clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	}
	if err != nil {
		return nil, err
	}

	return config, nil
}

// InstrumentedRESTConfig creates a REST config with request instrumentation for monitoring.
// Adds HTTP transport wrapper for Prometheus metrics collection and request timeout configuration.
//
// Path Processing: Simplifies URL paths for metrics by taking the last segment,
// reducing cardinality of metric labels for better performance.
//
// Timeout: Applies the specified request timeout to prevent hanging requests.
func InstrumentedRESTConfig(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*rest.Config, error) {
	config, err := GetRestConfig(kubeConfig, apiServerURL)
	if err != nil {
		return nil, err
	}

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return extdnshttp.NewInstrumentedTransport(rt)
	}

	config.Timeout = requestTimeout
	return config, nil
}
