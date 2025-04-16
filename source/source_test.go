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
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestParseTemplate(t *testing.T) {
	for _, tt := range []struct {
		name                     string
		annotationFilter         string
		fqdnTemplate             string
		combineFQDNAndAnnotation bool
		expectError              bool
	}{
		{
			name:         "invalid template",
			expectError:  true,
			fqdnTemplate: "{{.Name",
		},
		{
			name:        "valid empty template",
			expectError: false,
		},
		{
			name:         "valid template",
			expectError:  false,
			fqdnTemplate: "{{.Name}}-{{.Namespace}}.ext-dns.test.com",
		},
		{
			title:       "TTL annotation value is set correctly using duration (fractional)",
			annotations: map[string]string{ttlAnnotationKey: "20.5s"},
			expectedTTL: endpoint.TTL(20),
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			ttl := getTTLFromAnnotations(tc.annotations, "resource/test")
			assert.Equal(t, tc.expectedTTL, ttl)
		})
	}
}

func TestSuitableType(t *testing.T) {
	for _, tc := range []struct {
		target, recordType, expected string
	}{
		{"8.8.8.8", "", "A"},
		{"2001:db8::1", "", "AAAA"},
		{"::ffff:c0a8:101", "", "AAAA"},
		{"foo.example.org", "", "CNAME"},
		{"bar.eu-central-1.elb.amazonaws.com", "", "CNAME"},
	} {

		recordType := suitableType(tc.target)

		if recordType != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, recordType)
		}
	}
}

func TestGetProviderSpecificCloudflareAnnotations(t *testing.T) {
	for _, tc := range []struct {
		title         string
		annotations   map[string]string
		expectedKey   string
		expectedValue bool
	}{
		{
			title:         "Cloudflare proxied annotation is set correctly to true",
			annotations:   map[string]string{CloudflareProxiedKey: "true"},
			expectedKey:   CloudflareProxiedKey,
			expectedValue: true,
		},
		{
			title:         "Cloudflare proxied annotation is set correctly to false",
			annotations:   map[string]string{CloudflareProxiedKey: "false"},
			expectedKey:   CloudflareProxiedKey,
			expectedValue: false,
		},
		{
			title: "Cloudflare proxied annotation among another annotations is set correctly to true",
			annotations: map[string]string{
				"random annotation 1": "random value 1",
				CloudflareProxiedKey:  "false",
				"random annotation 2": "random value 2",
			},
			expectedKey:   CloudflareProxiedKey,
			expectedValue: false,
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			providerSpecificAnnotations, _ := getProviderSpecificAnnotations(tc.annotations)
			for _, providerSpecificAnnotation := range providerSpecificAnnotations {
				if providerSpecificAnnotation.Name == tc.expectedKey {
					assert.Equal(t, strconv.FormatBool(tc.expectedValue), providerSpecificAnnotation.Value)
					return
				}
			}
			t.Errorf("Cloudflare provider specific annotation %s is not set correctly to %v", tc.expectedKey, tc.expectedValue)
		})
	}

	for _, tc := range []struct {
		title         string
		annotations   map[string]string
		expectedKey   string
		expectedValue string
	}{
		{
			title:         "Cloudflare custom hostname annotation is set correctly",
			annotations:   map[string]string{CloudflareCustomHostnameKey: "a.foo.fancybar.com"},
			expectedKey:   CloudflareCustomHostnameKey,
			expectedValue: "a.foo.fancybar.com",
		},
		{
			title: "Cloudflare custom hostname annotation among another annotations is set correctly",
			annotations: map[string]string{
				"random annotation 1":       "random value 1",
				CloudflareCustomHostnameKey: "a.foo.fancybar.com",
				"random annotation 2":       "random value 2"},
			expectedKey:   CloudflareCustomHostnameKey,
			expectedValue: "a.foo.fancybar.com",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			providerSpecificAnnotations, _ := getProviderSpecificAnnotations(tc.annotations)
			for _, providerSpecificAnnotation := range providerSpecificAnnotations {
				if providerSpecificAnnotation.Name == tc.expectedKey {
					assert.Equal(t, tc.expectedValue, providerSpecificAnnotation.Value)
					return
				}
			}
			t.Errorf("Cloudflare provider specific annotation %s is not set correctly to %s", tc.expectedKey, tc.expectedValue)
		})
	}

	for _, tc := range []struct {
		title         string
		annotations   map[string]string
		expectedKey   string
		expectedValue string
	}{
		{
			title:         "Cloudflare region key annotation is set correctly",
			annotations:   map[string]string{CloudflareRegionKey: "us"},
			expectedKey:   CloudflareRegionKey,
			expectedValue: "us",
		},
		{
			title: "Cloudflare region key annotation among another annotations is set correctly",
			annotations: map[string]string{
				"random annotation 1": "random value 1",
				CloudflareRegionKey:   "us",
				"random annotation 2": "random value 2",
			},
			expectedKey:   CloudflareRegionKey,
			expectedValue: "us",
		},
	} {
		t.Run(tc.title, func(t *testing.T) {
			providerSpecificAnnotations, _ := getProviderSpecificAnnotations(tc.annotations)
			for _, providerSpecificAnnotation := range providerSpecificAnnotations {
				if providerSpecificAnnotation.Name == tc.expectedKey {
					assert.Equal(t, tc.expectedValue, providerSpecificAnnotation.Value)
					return
				}
			}
			t.Errorf("Cloudflare provider specific annotation %s is not set correctly to %v", tc.expectedKey, tc.expectedValue)
		})
	}
}

func TestFqdnTemplate(t *testing.T) {
	tests := []struct {
		name          string
		fqdnTemplate  string
		expectedError bool
	}{
		{
			name:          "empty template",
			fqdnTemplate:  "",
			expectedError: false,
		},
		{
			name:          "valid template",
			fqdnTemplate:  "{{ .Name }}.example.com",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := parseTemplate(tt.fqdnTemplate)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, tmpl)
			} else {
				assert.NoError(t, err)
				if tt.fqdnTemplate == "" {
					assert.Nil(t, tmpl)
				} else {
					assert.NotNil(t, tmpl)
				}
			}
		})
	}
}

type mockInformerFactory struct {
	syncResults map[reflect.Type]bool
}

func (m *mockInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool {
	return m.syncResults
}

func TestWaitForCacheSync(t *testing.T) {
	tests := []struct {
		name        string
		syncResults map[reflect.Type]bool
		expectError bool
	}{
		{
			name:        "all caches synced",
			syncResults: map[reflect.Type]bool{reflect.TypeOf(""): true},
			expectError: false,
		},
		{
			name:        "some caches not synced",
			syncResults: map[reflect.Type]bool{reflect.TypeOf(""): false},
			expectError: true,
		},
		{
			name:        "context timeout",
			syncResults: map[reflect.Type]bool{reflect.TypeOf(""): false},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			factory := &mockInformerFactory{syncResults: tt.syncResults}
			err := waitForCacheSync(ctx, factory)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
