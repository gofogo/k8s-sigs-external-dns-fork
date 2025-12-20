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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/aws"
	"sigs.k8s.io/external-dns/registry/aws_sd"
	"sigs.k8s.io/external-dns/registry/dynamodb"
	"sigs.k8s.io/external-dns/registry/noop"
	"sigs.k8s.io/external-dns/registry/txt"
)

// SelectRegistry selects the appropriate registry implementation based on the configuration in cfg.
// It initializes and returns a registry along with any error encountered during setup.
// Supported registry types include: dynamodb, noop, txt, and aws-sd.
func SelectRegistry(cfg *externaldns.Config, p provider.Provider) (Registry, error) {
	var r Registry
	var err error
	switch cfg.Registry {
	case "dynamodb":
		var dynamodbOpts []func(*dynamodb.Options)
		if cfg.AWSDynamoDBRegion != "" {
			dynamodbOpts = []func(*dynamodb.Options){
				func(opts *dynamodb.Options) {
					opts.Region = cfg.AWSDynamoDBRegion
				},
			}
		}
		r, err = dynamodb.NewDynamoDBRegistry(p, cfg.TXTOwnerID, dynamodb.NewFromConfig(aws.CreateDefaultV2Config(cfg), dynamodbOpts...), cfg.AWSDynamoDBTable, cfg.TXTPrefix, cfg.TXTSuffix, cfg.TXTWildcardReplacement, cfg.ManagedDNSRecordTypes, cfg.ExcludeDNSRecordTypes, []byte(cfg.TXTEncryptAESKey), cfg.TXTCacheInterval)
	case "noop":
		r, err = noop.NewNoopRegistry(p)
	case "txt":
		r, err = txt.NewTXTRegistry(p, cfg.TXTPrefix, cfg.TXTSuffix, cfg.TXTOwnerID, cfg.TXTCacheInterval, cfg.TXTWildcardReplacement, cfg.ManagedDNSRecordTypes, cfg.ExcludeDNSRecordTypes, cfg.TXTEncryptEnabled, []byte(cfg.TXTEncryptAESKey), cfg.TXTOwnerOld)
	case "aws-sd":
		r, err = aws_sd.NewAWSSDRegistry(p, cfg.TXTOwnerID)
	default:
		log.Fatalf("unknown registry: %s", cfg.Registry)
	}
	return r, err
}
