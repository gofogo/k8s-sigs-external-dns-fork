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

package wrappers

import (
	"context"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source"
)

func ChaninedWrapper(input source.Source) (source.Source, error) {
	return nil, nil
}

type WrapperHandler interface {
	Handle(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error)
}

type EndpointHandlerChain struct {
	handlers []WrapperHandler
}

func WrapperHandlerChain(handlers ...WrapperHandler) *EndpointHandlerChain {
	return &EndpointHandlerChain{handlers: handlers}
}

func (c *EndpointHandlerChain) Handle(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	var err error
	for _, handler := range c.handlers {
		endpoints, err = handler.Handle(endpoints)
		if err != nil {
			return nil, err
		}
	}
	return endpoints, nil
}

type ChainedSource struct {
	chain source.Source
}

type OrderedWrapper struct {
	Order  int
	WrapFn func(source.Source) (source.Source, error)
}

func NewChainedSource(base source.Source, wrappers ...func(source.Source) (source.Source, error)) (source.Source, error) {
	wrapped := base
	var err error
	for _, wrap := range wrappers {
		wrapped, err = wrap(wrapped)
		if err != nil {
			return nil, err
		}
	}
	return &ChainedSource{chain: wrapped}, nil
}

func (c *ChainedSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	return c.chain.Endpoints(ctx)
}

func (c *ChainedSource) AddEventHandler(ctx context.Context, handler func()) {
	c.chain.AddEventHandler(ctx, handler)
}
