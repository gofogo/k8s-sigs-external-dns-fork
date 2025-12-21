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

package registry

import (
	"fmt"

	sdkdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/aws"
	"sigs.k8s.io/external-dns/registry/aws_sd"
	registrydynamodb "sigs.k8s.io/external-dns/registry/dynamodb"
	"sigs.k8s.io/external-dns/registry/noop"
	"sigs.k8s.io/external-dns/registry/txt"
)

// SelectRegistry selects the appropriate registry implementation based on the configuration in cfg.
// It initializes and returns a registry along with any error encountered during setup.
// Supported registry types include: dynamodb, noop, txt, and aws-sd.
func SelectRegistry(cfg *externaldns.Config, p provider.Provider) (Registry, error) {
	factory, ok := registryFactories[cfg.Registry]
	if !ok {
		return nil, fmt.Errorf("unknown registry: %s", cfg.Registry)
	}
	return factory(cfg, p)
}

type registryFactory func(cfg *externaldns.Config, p provider.Provider) (Registry, error)

var registryFactories = map[string]registryFactory{
	"dynamodb": selectDynamoDBRegistry,
	"noop":     selectNoopRegistry,
	"txt":      selectTXTRegistry,
	"aws-sd":   selectAWSSDRegistry,
}

var dynamodbClientFactory = func(cfg *externaldns.Config, opts ...func(*sdkdynamodb.Options)) registrydynamodb.DynamoDBAPI {
	return sdkdynamodb.NewFromConfig(aws.CreateDefaultV2Config(cfg), opts...)
}

func selectDynamoDBRegistry(cfg *externaldns.Config, p provider.Provider) (Registry, error) {
	var dynamodbOpts []func(*sdkdynamodb.Options)
	if cfg.AWSDynamoDBRegion != "" {
		dynamodbOpts = []func(*sdkdynamodb.Options){
			func(opts *sdkdynamodb.Options) {
				opts.Region = cfg.AWSDynamoDBRegion
			},
		}
	}
	client := dynamodbClientFactory(cfg, dynamodbOpts...)
	return registrydynamodb.NewDynamoDBRegistry(p, cfg.TXTOwnerID, client, cfg.AWSDynamoDBTable, cfg.TXTPrefix, cfg.TXTSuffix, cfg.TXTWildcardReplacement, cfg.ManagedDNSRecordTypes, cfg.ExcludeDNSRecordTypes, []byte(cfg.TXTEncryptAESKey), cfg.TXTCacheInterval)
}

func selectNoopRegistry(_ *externaldns.Config, p provider.Provider) (Registry, error) {
	return noop.NewNoopRegistry(p)
}

func selectTXTRegistry(cfg *externaldns.Config, p provider.Provider) (Registry, error) {
	return txt.NewTXTRegistry(p, cfg.TXTPrefix, cfg.TXTSuffix, cfg.TXTOwnerID, cfg.TXTCacheInterval, cfg.TXTWildcardReplacement, cfg.ManagedDNSRecordTypes, cfg.ExcludeDNSRecordTypes, cfg.TXTEncryptEnabled, []byte(cfg.TXTEncryptAESKey), cfg.TXTOwnerOld)
}

func selectAWSSDRegistry(cfg *externaldns.Config, p provider.Provider) (Registry, error) {
	return aws_sd.NewAWSSDRegistry(p, cfg.TXTOwnerID)
}
