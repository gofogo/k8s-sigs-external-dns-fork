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
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/external-dns/endpoint"
	templatetest "sigs.k8s.io/external-dns/source/template/testutil"
)

// Validate that fakeSource implements Source.
var _ Source = &fakeSource{}

func TestFakeSourceEndpoints(t *testing.T) {
	sc, err := NewFakeSource(&Config{})
	require.NoError(t, err)

	endpoints, err := sc.Endpoints(t.Context())
	require.NoError(t, err)

	// One endpoint per known record type.
	assert.Len(t, endpoints, len(endpoint.KnownRecordTypes))

	byType := make(map[string]*endpoint.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		byType[ep.RecordType] = ep
	}

	for _, rt := range endpoint.KnownRecordTypes {
		assert.Contains(t, byType, rt, "missing endpoint for record type %s", rt)
	}
}

func TestFakeSource_ARecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeA)
	require.Len(t, ep.Targets, 1)
	ip := net.ParseIP(ep.Targets[0])
	assert.NotNil(t, ip, "A record target %q is not a valid IP", ep.Targets[0])
	assert.NotNil(t, ip.To4(), "A record target %q must be IPv4", ep.Targets[0])
}

func TestFakeSource_AAAARecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeAAAA)
	require.Len(t, ep.Targets, 1)
	addr, err := netip.ParseAddr(ep.Targets[0])
	require.NoError(t, err, "AAAA record target %q is not a valid IP address", ep.Targets[0])
	assert.True(t, addr.Is6() && !addr.Is4In6(), "AAAA record target %q must be native IPv6", ep.Targets[0])
}

func TestFakeSource_CNAMERecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeCNAME)
	require.Len(t, ep.Targets, 1)
	assert.True(t, strings.HasSuffix(ep.Targets[0], "."+defaultFQDNTemplate),
		"CNAME target %q should be under %s", ep.Targets[0], defaultFQDNTemplate)
}

func TestFakeSource_TXTRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeTXT)
	require.NotEmpty(t, ep.Targets)
}

func TestFakeSource_SRVRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeSRV)
	assert.True(t, strings.HasPrefix(ep.DNSName, "_sip._udp."), "SRV DNSName %q should start with _sip._udp.", ep.DNSName)
	require.Len(t, ep.Targets, 1)
	assert.True(t, ep.Targets.ValidateSRVRecord(), "SRV target %q is invalid", ep.Targets[0])
}

func TestFakeSource_NSRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeNS)
	assert.Equal(t, defaultFQDNTemplate, ep.DNSName)
	require.Len(t, ep.Targets, 1)
	assert.True(t, strings.HasSuffix(ep.Targets[0], "."+defaultFQDNTemplate),
		"NS target %q should be under %s", ep.Targets[0], defaultFQDNTemplate)
}

func TestFakeSource_PTRRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypePTR)
	assert.True(t, ep.ValidatePTRRecord(), "PTR record is invalid: %v", ep)
}

func TestFakeSource_MXRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeMX)
	assert.Equal(t, defaultFQDNTemplate, ep.DNSName)
	require.Len(t, ep.Targets, 1)
	_, err := endpoint.NewMXRecord(ep.Targets[0])
	assert.NoError(t, err, "MX target %q is invalid", ep.Targets[0])
}

func TestFakeSource_NAPTRRecord(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeNAPTR)
	assert.True(t, strings.HasPrefix(ep.DNSName, "_sip._udp."), "NAPTR DNSName %q should start with _sip._udp.", ep.DNSName)
	require.NotEmpty(t, ep.Targets)
}

func TestFakeSource_GenerateEndpointForType_RefObject(t *testing.T) {
	ep := mustGenerateEndpointForType(t, endpoint.RecordTypeA)
	require.NotNil(t, ep.RefObject())
	assert.Equal(t, "Pod", ep.RefObject().Kind)
}

func TestFakeSource_FQDNTemplate(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		wantDomain string
	}{
		{
			name:       "template expression",
			template:   "{{.Name}}.my-company.com",
			wantDomain: "fake.my-company.com",
		},
		{
			name:       "plain domain",
			template:   "my-company.com",
			wantDomain: "my-company.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := NewFakeSource(&Config{
				TemplateEngine: templatetest.MustEngine(t, tt.template, "", "", false),
			})
			require.NoError(t, err)

			endpoints, err := sc.Endpoints(t.Context())
			require.NoError(t, err)
			require.NotEmpty(t, endpoints)

			for _, ep := range endpoints {
				if ep.RecordType == endpoint.RecordTypePTR {
					continue // PTR names are reverse-DNS, not under the configured domain
				}
				assert.True(t, strings.HasSuffix(ep.DNSName, "."+tt.wantDomain) || ep.DNSName == tt.wantDomain,
					"endpoint DNSName %q should be under %s", ep.DNSName, tt.wantDomain)
			}
		})
	}
}

// mustGenerateEndpointForType is a test helper that generates an endpoint for the given type.
func mustGenerateEndpointForType(t *testing.T, recordType string) *endpoint.Endpoint {
	t.Helper()
	sc, err := NewFakeSource(&Config{})
	require.NoError(t, err)
	fs := sc.(*fakeSource)
	ep, err := fs.generateEndpointForType(recordType)
	require.NoError(t, err)
	require.NotNil(t, ep, "endpoint for type %s should not be nil", recordType)
	return ep
}
