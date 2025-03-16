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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/linki/instrumented_http"
	openshift "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	istiofake "istio.io/client-go/pkg/clientset/versioned/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	fakeDynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	fakeKube "k8s.io/client-go/kubernetes/fake"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

type MockClientGenerator struct {
	mock.Mock
	kubeClient              kubernetes.Interface
	gatewayClient           gateway.Interface
	istioClient             istioclient.Interface
	cloudFoundryClient      *cfclient.Client
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

func (m *MockClientGenerator) CloudFoundryClient(cfAPIEndpoint string, cfUsername string, cfPassword string) (*cfclient.Client, error) {
	args := m.Called()
	if args.Error(1) == nil {
		m.cloudFoundryClient = args.Get(0).(*cfclient.Client)
		return m.cloudFoundryClient, nil
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

type ByNamesTestSuite struct {
	suite.Suite
}

func (suite *ByNamesTestSuite) TestAllInitialized() {
	mockClientGenerator := new(MockClientGenerator)
	mockClientGenerator.On("KubeClient").Return(fakeKube.NewSimpleClientset(), nil)
	mockClientGenerator.On("IstioClient").Return(istiofake.NewSimpleClientset(), nil)
	mockClientGenerator.On("DynamicKubernetesClient").Return(fakeDynamic.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{
				Group:    "projectcontour.io",
				Version:  "v1",
				Resource: "httpproxies",
			}: "HTTPPRoxiesList",
			{
				Group:    "contour.heptio.com",
				Version:  "v1beta1",
				Resource: "tcpingresses",
			}: "TCPIngressesList",
			{
				Group:    "configuration.konghq.com",
				Version:  "v1beta1",
				Resource: "tcpingresses",
			}: "TCPIngressesList",
			{
				Group:    "cis.f5.com",
				Version:  "v1",
				Resource: "virtualservers",
			}: "VirtualServersList",
			{
				Group:    "cis.f5.com",
				Version:  "v1",
				Resource: "transportservers",
			}: "TransportServersList",
			{
				Group:    "traefik.containo.us",
				Version:  "v1alpha1",
				Resource: "ingressroutes",
			}: "IngressRouteList",
			{
				Group:    "traefik.containo.us",
				Version:  "v1alpha1",
				Resource: "ingressroutetcps",
			}: "IngressRouteTCPList",
			{
				Group:    "traefik.containo.us",
				Version:  "v1alpha1",
				Resource: "ingressrouteudps",
			}: "IngressRouteUDPList",
			{
				Group:    "traefik.io",
				Version:  "v1alpha1",
				Resource: "ingressroutes",
			}: "IngressRouteList",
			{
				Group:    "traefik.io",
				Version:  "v1alpha1",
				Resource: "ingressroutetcps",
			}: "IngressRouteTCPList",
			{
				Group:    "traefik.io",
				Version:  "v1alpha1",
				Resource: "ingressrouteudps",
			}: "IngressRouteUDPList",
		}), nil)

	sources, err := ByNames(context.TODO(), mockClientGenerator, []string{"service", "ingress", "istio-gateway", "contour-httpproxy", "kong-tcpingress", "f5-virtualserver", "f5-transportserver", "traefik-proxy", "fake"}, &Config{})
	suite.NoError(err, "should not generate errors")
	suite.Len(sources, 9, "should generate all nine sources")
}

func (suite *ByNamesTestSuite) TestOnlyFake() {
	mockClientGenerator := new(MockClientGenerator)
	mockClientGenerator.On("KubeClient").Return(fakeKube.NewSimpleClientset(), nil)

	sources, err := ByNames(context.TODO(), mockClientGenerator, []string{"fake"}, &Config{})
	suite.NoError(err, "should not generate errors")
	suite.Len(sources, 1, "should generate fake source")
	suite.Nil(mockClientGenerator.kubeClient, "client should not be created")
}

func (suite *ByNamesTestSuite) TestSourceNotFound() {
	mockClientGenerator := new(MockClientGenerator)
	mockClientGenerator.On("KubeClient").Return(fakeKube.NewSimpleClientset(), nil)

	sources, err := ByNames(context.TODO(), mockClientGenerator, []string{"foo"}, &Config{})
	suite.Equal(err, ErrSourceNotFound, "should return source not found")
	suite.Len(sources, 0, "should not returns any source")
}

func (suite *ByNamesTestSuite) TestKubeClientFails() {
	mockClientGenerator := new(MockClientGenerator)
	mockClientGenerator.On("KubeClient").Return(nil, errors.New("foo"))

	_, err := ByNames(context.TODO(), mockClientGenerator, []string{"service"}, &Config{})
	suite.Error(err, "should return an error if kubernetes client cannot be created")

	_, err = ByNames(context.TODO(), mockClientGenerator, []string{"ingress"}, &Config{})
	suite.Error(err, "should return an error if kubernetes client cannot be created")

	_, err = ByNames(context.TODO(), mockClientGenerator, []string{"istio-gateway"}, &Config{})
	suite.Error(err, "should return an error if kubernetes client cannot be created")

	_, err = ByNames(context.TODO(), mockClientGenerator, []string{"kong-tcpingress"}, &Config{})
	suite.Error(err, "should return an error if kubernetes client cannot be created")
}

func (suite *ByNamesTestSuite) TestIstioClientFails() {
	mockClientGenerator := new(MockClientGenerator)
	mockClientGenerator.On("KubeClient").Return(fakeKube.NewSimpleClientset(), nil)
	mockClientGenerator.On("IstioClient").Return(nil, errors.New("foo"))
	mockClientGenerator.On("DynamicKubernetesClient").Return(nil, errors.New("foo"))

	_, err := ByNames(context.TODO(), mockClientGenerator, []string{"istio-gateway"}, &Config{})
	suite.Error(err, "should return an error if istio client cannot be created")

	_, err = ByNames(context.TODO(), mockClientGenerator, []string{"contour-httpproxy"}, &Config{})
	suite.Error(err, "should return an error if contour client cannot be created")
}

func TestCaptureUsesellesLogs(t *testing.T) {

	// log.SetOutput(ioutil.Discard)
	log.SetLevel(log.DebugLevel)
	// buf := testutils.LogsToBuffer(log.DebugLevel, t)
	// klog.SetLogger(logr.Discard())

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe("127.0.0.1:9099", nil))
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		// w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := instrumented_http.NewClient(nil, &instrumented_http.Callbacks{
		PathProcessor: func(path string) string {
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		},
		// QueryProcessor: instrumented_http.IdentityProcessor,
	})

	client.Timeout = 4 * time.Second

	func() {
		resp, err := client.Get(server.URL)
		if err != nil {
			fmt.Println("error", err)
			log.Fatal(err)
		}
		defer resp.Body.Close()

		fmt.Printf("%d\n", resp.StatusCode)
	}()

	func() {
		resp, err := client.Get("https://kubernetes.io/docs/search/?q=pods")
		if err != nil {
			fmt.Println("error", err)
			log.Fatal(err)
		}
		defer resp.Body.Close()

		fmt.Printf("%d\n", resp.StatusCode)
	}()

	// fmt.Println(buf.String())

	// cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
	// 	return instrumented_http.NewTransport(rt, &instrumented_http.Callbacks{
	// 		PathProcessor: instrumented_http.LastPathElementProcessor,
	// 	})
	// }

}

func TestServerWithoutCancelation(t *testing.T) {
	// fake.NewClientset()
	log.SetLevel(log.DebugLevel)

	// Create a new test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// w.WriteHeader(http.StatusOK)
		// w.Write([]byte("Hello, client"))
		r.Close = true
		r.Cancel = make(chan struct{})
		w.Header().Add("Content-Type", "application/json")
		r.Body.Close()
		fmt.Fprintln(w, `response from the mock server goes here`)

	}))
	defer server.Close()

	client := instrumented_http.NewClient(nil, &instrumented_http.Callbacks{
		PathProcessor: func(path string) string {
			fmt.Println("path:", path)
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		},
		// QueryProcessor: instrumented_http.IdentityProcessor,
	})

	// Make a request to the test server
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Print the response status and body
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %s\n", string(body))

}

func TestAnother(t *testing.T) {
	// Use the default transport which is an instance of *http.Transport that supports cancellation.
	client := &http.Client{
		Transport: http.DefaultTransport,
	}

	// Create a request with a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Cancel the request after 2 seconds.
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
		log.Println("Request cancelled")
	}()

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Request error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	log.Println("Response:", string(body))
}

func TestWriteTimeoutCancellation(t *testing.T) {
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body.Close() // to show that it is not https://github.com/golang/go/issues/23262

		time.Sleep(1 * time.Second)
		_, err := w.Write([]byte("ok"))
		ctxErr := r.Context().Err()

		_, _ = ctxErr, err // breakpoint here to see the errors, they are both nil
	}))
	backend.Config.WriteTimeout = 500 * time.Millisecond
	backend.EnableHTTP2 = true // you can also try false

	backend.Start()
	defer backend.Close()

	start := time.Now()
	res, err := backend.Client().Get(backend.URL)
	t.Logf("took %s", time.Since(start)) // for me this always takes 1s, although the write timeout is 500ms
	_, _ = res, err                      // breakpoint here, err is EOF, res is nil
}

func TestByNames(t *testing.T) {
	suite.Run(t, new(ByNamesTestSuite))
}
