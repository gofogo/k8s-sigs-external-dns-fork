package aws

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

func BenchmarkAdjustEndpoint100With3Properties(b *testing.B) {
	provider := buildProvider()
	records := buildEndpointsWithProperties(100, 3)

	for b.Loop() {
		_, err := provider.AdjustEndpoints(records)
		require.NoError(b, err)
	}
}

func BenchmarkAdjustEndpoint1000With3Properties(b *testing.B) {
	provider := buildProvider()
	records := buildEndpointsWithProperties(1000, 10)

	for b.Loop() {
		_, _ = provider.AdjustEndpoints(records)
	}
}

func BenchmarkAdjustEndpoint1000With3PropertiesHashMap(b *testing.B) {
	provider := buildProvider()
	provider.useHashMap = true
	records := buildEndpointsWithProperties(1000, 10)

	for b.Loop() {
		_, _ = provider.AdjustEndpoints(records)
	}
}

func buildProvider() *AWSProvider {
	client := NewRoute53APIStub(nil)

	provider := &AWSProvider{
		clients:               map[string]Route53API{defaultAWSProfile: client},
		batchChangeSize:       defaultBatchChangeSize,
		batchChangeSizeBytes:  defaultBatchChangeSizeBytes,
		batchChangeSizeValues: defaultBatchChangeSizeValues,
		batchChangeInterval:   defaultBatchChangeInterval,
		evaluateTargetHealth:  true,
		domainFilter:          &endpoint.DomainFilter{},
		zoneIDFilter:          provider.ZoneIDFilter{},
		zoneTypeFilter:        provider.ZoneTypeFilter{},
		zoneTagFilter:         provider.ZoneTagFilter{},
		dryRun:                false,
		zonesCache:            &zonesListCache{duration: 1 * time.Minute},
		failedChangesQueue:    make(map[string]Route53Changes),
	}
	return provider
}

func buildEndpointsWithProperties(nEndpoints int, prop int) []*endpoint.Endpoint {
	properties := map[string]string{
		providerSpecificAlias:                              "true",
		providerSpecificEvaluateTargetHealth:               "true",
		providerSpecificWeight:                             "100",
		providerSpecificRegion:                             "us-east-1",
		providerSpecificFailover:                           "PRIMARY",
		providerSpecificHealthCheckID:                      "asdf1234-as12-as12-as12-asdf12345678",
		providerSpecificGeoProximityLocationAWSRegion:      "us-east-1",
		providerSpecificMultiValueAnswer:                   "true",
		providerSpecificGeoProximityLocationLocalZoneGroup: "us-east-1a",
		providerSpecificGeolocationContinentCode:           "NA",
		providerSpecificGeolocationCountryCode:             "US",
	}
	endpoints := make([]*endpoint.Endpoint, nEndpoints)
	for i := 0; i < nEndpoints; i++ {
		recordType := endpoint.RecordTypeA
		if i%2 == 0 {
			recordType = endpoint.RecordTypeCNAME
		} else if i%3 == 1 {
			recordType = endpoint.RecordTypeAAAA
		}
		endpoints[i] = endpoint.NewEndpoint(fmt.Sprintf("index-%d.example.com", i), recordType)
		count := 0
		for k, v := range properties {
			count++
			endpoints[i].SetProviderSpecificProperty(k, v)
			if count >= prop {
				break
			}
		}
	}
	return endpoints
}
