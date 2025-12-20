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
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/provider/inmemory"
	"sigs.k8s.io/external-dns/registry/aws_sd"
	registrydynamodb "sigs.k8s.io/external-dns/registry/dynamodb"
	"sigs.k8s.io/external-dns/registry/noop"
	"sigs.k8s.io/external-dns/registry/txt"
)

type fakeDynamoDBAPI struct{}

func (fakeDynamoDBAPI) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{}, nil
}

func (fakeDynamoDBAPI) Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	return &dynamodb.ScanOutput{}, nil
}

func (fakeDynamoDBAPI) BatchExecuteStatement(context.Context, *dynamodb.BatchExecuteStatementInput, ...func(*dynamodb.Options)) (*dynamodb.BatchExecuteStatementOutput, error) {
	return &dynamodb.BatchExecuteStatementOutput{}, nil
}

func TestSelectRegistry(t *testing.T) {
	cfg := externaldns.NewConfig()
	cfg.TXTOwnerID = "owner"
	cfg.TXTPrefix = "txt."
	cfg.TXTCacheInterval = time.Minute
	cfg.AWSDynamoDBTable = "table"
	cfg.Registry = "noop"

	provider := inmemory.NewInMemoryProvider()

	tests := []struct {
		name         string
		registryName string
		assertType   any
	}{
		{
			name:         "noop",
			registryName: "noop",
			assertType:   &noop.NoopRegistry{},
		},
		{
			name:         "txt",
			registryName: "txt",
			assertType:   &txt.TXTRegistry{},
		},
		{
			name:         "aws-sd",
			registryName: "aws-sd",
			assertType:   &aws_sd.AWSSDRegistry{},
		},
		{
			name:         "dynamodb",
			registryName: "dynamodb",
			assertType:   &registrydynamodb.DynamoDBRegistry{},
		},
	}

	originalFactory := dynamodbClientFactory
	dynamodbClientFactory = func(*externaldns.Config, ...func(*dynamodb.Options)) registrydynamodb.DynamoDBAPI {
		return fakeDynamoDBAPI{}
	}
	t.Cleanup(func() {
		dynamodbClientFactory = originalFactory
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.Registry = tt.registryName
			reg, err := SelectRegistry(cfg, provider)
			require.NoError(t, err)
			require.IsType(t, tt.assertType, reg)
		})
	}
}

func TestSelectRegistryUnknown(t *testing.T) {
	cfg := externaldns.NewConfig()
	cfg.Registry = "nope"

	provider := inmemory.NewInMemoryProvider()

	reg, err := SelectRegistry(cfg, provider)
	require.Error(t, err)
	require.Nil(t, reg)
}
