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

package factory

import (
	"context"

	kubeclient "sigs.k8s.io/external-dns/pkg/client"
	"sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/types"
)

// SourceConstructor is a function that creates a Source from context, client generator, and config.
type SourceConstructor func(ctx context.Context, p source.ClientGenerator, cfg *source.Config) (source.Source, error)

// Select creates a Source implementation by name using the factory pattern.
// Returns source.ErrSourceNotFound for unsupported source types.
func Select(ctx context.Context, name string, p source.ClientGenerator, cfg *source.Config) (source.Source, error) {
	constructor, ok := sourceConstructors(name)
	if !ok {
		return nil, source.ErrSourceNotFound
	}
	return constructor(ctx, p, cfg)
}

// ByNames returns multiple Sources given the names configured in cfg.
func ByNames(ctx context.Context, cfg *source.Config, p source.ClientGenerator) ([]source.Source, error) {
	sources := make([]source.Source, 0, len(cfg.Sources))
	for _, name := range cfg.Sources {
		src, err := Select(ctx, name, p, cfg)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, nil
}

// sourceConstructors looks up the constructor function for the given source name.
func sourceConstructors(name string) (SourceConstructor, bool) {
	constructors := map[string]SourceConstructor{
		types.Node:                source.NewNode,
		types.Service:             source.NewService,
		types.Ingress:             source.NewIngress,
		types.Pod:                 source.NewPod,
		types.GatewayHttpRoute:    source.NewGatewayHTTPRouteSource,
		types.GatewayGrpcRoute:    source.NewGatewayGRPCRouteSource,
		types.GatewayTlsRoute:     source.NewGatewayTLSRouteSource,
		types.GatewayTcpRoute:     source.NewGatewayTCPRouteSource,
		types.GatewayUdpRoute:     source.NewGatewayUDPRouteSource,
		types.IstioGateway:        source.NewIstioGateway,
		types.IstioVirtualService: source.NewIstioVirtualService,
		types.AmbassadorHost:      source.NewAmbassadorHost,
		types.ContourHTTPProxy:    source.NewContourHTTPProxy,
		types.GlooProxy:           source.NewGlooProxy,
		types.TraefikProxy:        source.NewTraefikProxy,
		types.OpenShiftRoute:      source.NewOpenShiftRoute,
		types.KongTCPIngress:      source.NewKongTCPIngress,
		types.F5VirtualServer:     source.NewF5VirtualServer,
		types.F5TransportServer:   source.NewF5TransportServer,
		types.Unstructured:        source.NewUnstructured,
		types.Fake:                source.NewFake,
		types.Connector:           source.NewConnector,
		types.CRD:                 buildCRDSource,
		types.SkipperRouteGroup:   buildSkipperRouteGroupSource,
	}
	c, ok := constructors[name]
	return c, ok
}

func buildCRDSource(_ context.Context, p source.ClientGenerator, cfg *source.Config) (source.Source, error) {
	client, err := p.KubeClient()
	if err != nil {
		return nil, err
	}
	crdClient, scheme, err := source.NewCRDClientForAPIVersionKind(client, cfg)
	if err != nil {
		return nil, err
	}
	return source.NewCRDSource(crdClient, cfg, scheme)
}

func buildSkipperRouteGroupSource(_ context.Context, _ source.ClientGenerator, cfg *source.Config) (source.Source, error) {
	apiServerURL := cfg.APIServerURL
	tokenPath := ""
	token := ""
	restConfig, err := kubeclient.GetRestConfig(cfg.KubeConfig, cfg.APIServerURL)
	if err == nil {
		apiServerURL = restConfig.Host
		tokenPath = restConfig.BearerTokenFile
		token = restConfig.BearerToken
	}
	return source.NewRouteGroupSource(cfg, token, tokenPath, apiServerURL)
}
