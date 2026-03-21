/*
Copyright 2017 The Kubernetes Authors.

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
	"testing"
	"time"

	openshift "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// MockClientGenerator is a shared mock for ClientGenerator used across source tests.
type MockClientGenerator struct {
	mock.Mock
	kubeClient              kubernetes.Interface
	gatewayClient           gateway.Interface
	istioClient             istioclient.Interface
	dynamicKubernetesClient dynamic.Interface
	openshiftClient         openshift.Interface
}

func (m *MockClientGenerator) KubeClient() (kubernetes.Interface, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.kubeClient = args.Get(0).(kubernetes.Interface)
		return m.kubeClient, nil
	}
	return nil, args.Error(1)
}

func (m *MockClientGenerator) GatewayClient() (gateway.Interface, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	m.gatewayClient = args.Get(0).(gateway.Interface)
	return m.gatewayClient, nil
}

func (m *MockClientGenerator) IstioClient() (istioclient.Interface, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.istioClient = args.Get(0).(istioclient.Interface)
		return m.istioClient, nil
	}
	return nil, args.Error(1)
}

func (m *MockClientGenerator) DynamicKubernetesClient() (dynamic.Interface, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.dynamicKubernetesClient = args.Get(0).(dynamic.Interface)
		return m.dynamicKubernetesClient, nil
	}
	return nil, args.Error(1)
}

func (m *MockClientGenerator) OpenShiftClient() (openshift.Interface, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.openshiftClient = args.Get(0).(openshift.Interface)
		return m.openshiftClient, nil
	}
	return nil, args.Error(1)
}

func (m *MockClientGenerator) RESTConfig() (*rest.Config, error) {
	args := m.Called()
	if args.Error(1) == nil {
		return args.Get(0).(*rest.Config), nil
	}
	return nil, args.Error(1)
}

func TestConfig_ClientGenerator(t *testing.T) {
	cfg := &Config{
		KubeConfig:     "/path/to/kubeconfig",
		APIServerURL:   "https://api.example.com",
		RequestTimeout: 30 * time.Second,
		UpdateEvents:   false,
	}

	gen := cfg.ClientGenerator()

	assert.Equal(t, "/path/to/kubeconfig", gen.KubeConfig)
	assert.Equal(t, "https://api.example.com", gen.APIServerURL)
	assert.Equal(t, 30*time.Second, gen.RequestTimeout)
}

func TestConfig_ClientGenerator_UpdateEvents(t *testing.T) {
	cfg := &Config{
		KubeConfig:     "/path/to/kubeconfig",
		APIServerURL:   "https://api.example.com",
		RequestTimeout: 30 * time.Second,
		UpdateEvents:   true, // Special case
	}

	gen := cfg.ClientGenerator()

	assert.Equal(t, time.Duration(0), gen.RequestTimeout, "UpdateEvents should set timeout to 0")
}

func TestConfig_ClientGenerator_Caching(t *testing.T) {
	cfg := &Config{
		KubeConfig:     "/path/to/kubeconfig",
		APIServerURL:   "https://api.example.com",
		RequestTimeout: 30 * time.Second,
		UpdateEvents:   false,
	}

	// Call ClientGenerator twice
	gen1 := cfg.ClientGenerator()
	gen2 := cfg.ClientGenerator()

	// Should return the same instance (cached)
	assert.Same(t, gen1, gen2, "ClientGenerator should return the same cached instance")
}

// TestSingletonClientGenerator_RESTConfig_TimeoutPropagation verifies timeout configuration
func TestSingletonClientGenerator_RESTConfig_TimeoutPropagation(t *testing.T) {
	testCases := []struct {
		name           string
		requestTimeout time.Duration
	}{
		{
			name:           "30 second timeout",
			requestTimeout: 30 * time.Second,
		},
		{
			name:           "60 second timeout",
			requestTimeout: 60 * time.Second,
		},
		{
			name:           "zero timeout (for watches)",
			requestTimeout: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gen := &SingletonClientGenerator{
				KubeConfig:     "",
				APIServerURL:   "",
				RequestTimeout: tc.requestTimeout,
			}

			// Verify the generator was configured with correct timeout
			assert.Equal(t, tc.requestTimeout, gen.RequestTimeout,
				"SingletonClientGenerator should have the configured RequestTimeout")

			config, err := gen.RESTConfig()

			// Even if config creation failed, verify the timeout was set in generator
			assert.Equal(t, tc.requestTimeout, gen.RequestTimeout,
				"RequestTimeout should remain unchanged after RESTConfig() call")

			// If config was successfully created, verify timeout propagated correctly
			if err == nil {
				require.NotNil(t, config, "Config should not be nil when error is nil")
				assert.Equal(t, tc.requestTimeout, config.Timeout,
					"REST config should have timeout matching RequestTimeout field")
			}
		})
	}
}

// TestConfig_ClientGenerator_RESTConfig_Integration verifies Config → ClientGenerator → RESTConfig flow
func TestConfig_ClientGenerator_RESTConfig_Integration(t *testing.T) {
	t.Run("normal timeout is propagated", func(t *testing.T) {
		cfg := &Config{
			KubeConfig:     "",
			APIServerURL:   "",
			RequestTimeout: 45 * time.Second,
			UpdateEvents:   false,
		}

		gen := cfg.ClientGenerator()

		// Verify ClientGenerator has correct timeout
		assert.Equal(t, 45*time.Second, gen.RequestTimeout,
			"ClientGenerator should have the configured RequestTimeout")

		config, err := gen.RESTConfig()

		// Even if config creation fails, the timeout setting should be correct
		assert.Equal(t, 45*time.Second, gen.RequestTimeout,
			"RequestTimeout should remain 45s after RESTConfig() call")

		if err == nil {
			require.NotNil(t, config, "Config should not be nil when error is nil")
			assert.Equal(t, 45*time.Second, config.Timeout,
				"RESTConfig should propagate the timeout")
		}
	})

	t.Run("UpdateEvents sets timeout to zero", func(t *testing.T) {
		cfg := &Config{
			KubeConfig:     "",
			APIServerURL:   "",
			RequestTimeout: 45 * time.Second,
			UpdateEvents:   true, // Should override to 0
		}

		gen := cfg.ClientGenerator()

		// When UpdateEvents=true, ClientGenerator sets timeout to 0 (for long-running watches)
		assert.Equal(t, time.Duration(0), gen.RequestTimeout,
			"ClientGenerator should have zero timeout when UpdateEvents=true")

		config, err := gen.RESTConfig()

		// Verify the timeout is 0, regardless of whether config was created
		assert.Equal(t, time.Duration(0), gen.RequestTimeout,
			"RequestTimeout should remain 0 after RESTConfig() call")

		if err == nil {
			require.NotNil(t, config, "Config should not be nil when error is nil")
			assert.Equal(t, time.Duration(0), config.Timeout,
				"RESTConfig should have zero timeout for watch operations")
		}
	})
}

// TestSingletonClientGenerator_RESTConfig_SharedAcrossClients verifies singleton is shared
func TestSingletonClientGenerator_RESTConfig_SharedAcrossClients(t *testing.T) {
	gen := &SingletonClientGenerator{
		KubeConfig:     "/nonexistent/path/to/kubeconfig",
		APIServerURL:   "",
		RequestTimeout: 30 * time.Second,
	}

	// Get REST config multiple times
	restConfig1, err1 := gen.RESTConfig()
	restConfig2, err2 := gen.RESTConfig()
	restConfig3, err3 := gen.RESTConfig()

	// Verify singleton behavior - all should return same instance
	assert.Same(t, restConfig1, restConfig2, "RESTConfig should return same instance on second call")
	assert.Same(t, restConfig1, restConfig3, "RESTConfig should return same instance on third call")

	// Verify the internal field matches
	assert.Same(t, restConfig1, gen.restConfig,
		"Internal restConfig field should match returned value")

	// Verify first call had error (no valid kubeconfig)
	assert.Error(t, err1, "First call should return error when kubeconfig is invalid")

	// Due to sync.Once bug, subsequent calls won't return the error
	// This is documented in the TODO comment on SingletonClientGenerator
	require.NoError(t, err2, "Second call does not return error due to sync.Once bug")
	require.NoError(t, err3, "Third call does not return error due to sync.Once bug")
}
