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

package kubeclient

import (
	"context"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	clientmetrics "k8s.io/client-go/tools/metrics"

	"sigs.k8s.io/external-dns/pkg/metrics"
)

const subsystem = "rest_client"

var (
	rateLimiterDuration = metrics.NewHistogramVecWithOpts(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "rate_limiter_duration_seconds",
			Help:      "Client-side rate limiter latency in seconds, partitioned by verb and URL path.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"verb", "url"},
	)
	requestSize = metrics.NewHistogramVecWithOpts(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "Request size in bytes, partitioned by verb and host.",
			Buckets:   prometheus.ExponentialBuckets(1, 10, 8),
		},
		[]string{"verb", "host"},
	)
	responseSize = metrics.NewHistogramVecWithOpts(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "Response size in bytes, partitioned by verb and host.",
			Buckets:   prometheus.ExponentialBuckets(1, 10, 8),
		},
		[]string{"verb", "host"},
	)
	retryRequestsTotal = metrics.NewCounterVecWithOpts(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "retry_requests_total",
			Help:      "Number of retried requests to the Kubernetes API, partitioned by status code, method, and host.",
		},
		[]string{"code", "method", "host"},
	)
)

func init() {
	metrics.RegisterMetric.MustRegister(rateLimiterDuration)
	metrics.RegisterMetric.MustRegister(requestSize)
	metrics.RegisterMetric.MustRegister(responseSize)
	metrics.RegisterMetric.MustRegister(retryRequestsTotal)

	clientmetrics.Register(clientmetrics.RegisterOpts{
		RateLimiterLatency: &latencyAdapter{rateLimiterDuration},
		RequestSize:        &sizeAdapter{requestSize},
		ResponseSize:       &sizeAdapter{responseSize},
		RequestRetry:       &retryAdapter{retryRequestsTotal},
	})
}

type latencyAdapter struct{ m metrics.HistogramVecMetric }

func (a *latencyAdapter) Observe(_ context.Context, verb string, u url.URL, latency time.Duration) {
	a.m.HistogramVec.WithLabelValues(verb, u.Path).Observe(latency.Seconds())
}

type sizeAdapter struct{ m metrics.HistogramVecMetric }

func (a *sizeAdapter) Observe(_ context.Context, verb, host string, size float64) {
	a.m.HistogramVec.WithLabelValues(verb, host).Observe(size)
}

type retryAdapter struct{ m metrics.CounterVecMetric }

func (a *retryAdapter) IncrementRetry(_ context.Context, code, method, host string) {
	a.m.CounterVec.WithLabelValues(code, method, host).Inc()
}
