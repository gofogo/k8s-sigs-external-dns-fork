/*
Copyright 2026 The Kubernetes Authors.

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

	"github.com/prometheus/client_golang/prometheus"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/metrics"
	"sigs.k8s.io/external-dns/source"
)

var sourceEndpointsByType = metrics.NewGaugedVectorOpts(
	prometheus.GaugeOpts{
		Subsystem: "source",
		Name:      "endpoints_by_type",
		Help:      "Number of endpoints per source type, partitioned by source type and record type.",
	},
	[]string{"source_type", "record_type"},
)

func init() {
	metrics.RegisterMetric.MustRegister(sourceEndpointsByType)
}

// instrumentedSource wraps a Source and records per-source-type endpoint counts.
type instrumentedSource struct {
	source.Source
	sourceType string
}

// Endpoints calls the underlying source and records the endpoint counts by record type.
func (s *instrumentedSource) Endpoints(ctx context.Context) ([]*endpoint.Endpoint, error) {
	endpoints, err := s.Source.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]float64)
	for _, ep := range endpoints {
		counts[ep.RecordType]++
	}
	for recordType, count := range counts {
		sourceEndpointsByType.Gauge.WithLabelValues(s.sourceType, recordType).Set(count)
	}
	return endpoints, nil
}

// newInstrumentedSource wraps src so that Endpoints() records metrics labelled with sourceType.
func newInstrumentedSource(src source.Source, sourceType string) source.Source {
	return &instrumentedSource{Source: src, sourceType: sourceType}
}
