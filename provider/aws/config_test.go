/*
Copyright 2023 The Kubernetes Authors.

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

package aws

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
)

func Test_newV2Config(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	defer os.Unsetenv("AWS_REGION")
	defer os.Unsetenv("AWS_EC2_METADATA_DISABLED")

	t.Run("should use profile from credentials file", func(t *testing.T) {
		// setup
		credsFile, err := prepareCredentialsFile(t)
		defer os.Remove(credsFile.Name())
		require.NoError(t, err)
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsFile.Name())
		defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

		// when
		cfg, err := newV2Config(AWSSessionConfig{Profile: "profile2"})
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(context.Background())

		// then
		assert.NoError(t, err)
		assert.Equal(t, "AKID2345", creds.AccessKeyID)
		assert.Equal(t, "SECRET2", creds.SecretAccessKey)
	})

	t.Run("should respect env variables without profile", func(t *testing.T) {
		// setup
		os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "topsecret")
		defer os.Unsetenv("AWS_ACCESS_KEY_ID")
		defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

		// when
		cfg, err := newV2Config(AWSSessionConfig{})
		require.NoError(t, err)
		creds, err := cfg.Credentials.Retrieve(context.Background())

		// then
		assert.NoError(t, err)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", creds.AccessKeyID)
		assert.Equal(t, "topsecret", creds.SecretAccessKey)
	})

	t.Run("should not error when AWS_CA_BUNDLE set", func(t *testing.T) {
		// setup
		os.Setenv("AWS_CA_BUNDLE", "../../internal/testresources/ca.pem")
		defer os.Unsetenv("AWS_CA_BUNDLE")

		// when
		_, err := newV2Config(AWSSessionConfig{})
		require.NoError(t, err)

		// then
		assert.NoError(t, err)
	})

	t.Run("should configure assume role credentials", func(t *testing.T) {
		// setup
		os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "topsecret")
		defer os.Unsetenv("AWS_ACCESS_KEY_ID")
		defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

		// when
		cfg, err := newV2Config(AWSSessionConfig{
			AssumeRole:           "arn:aws:iam::123456789012:role/example",
			AssumeRoleExternalID: "external-id",
		})

		// then
		require.NoError(t, err)
		require.NotNil(t, cfg.Credentials)
		assert.Contains(t, fmt.Sprintf("%T", cfg.Credentials), "CredentialsCache")
	})
}

func prepareCredentialsFile(t *testing.T) (*os.File, error) {
	credsFile, err := os.CreateTemp("", "aws-*.creds")
	require.NoError(t, err)
	_, err = credsFile.WriteString("[profile1]\naws_access_key_id=AKID1234\naws_secret_access_key=SECRET1\n\n[profile2]\naws_access_key_id=AKID2345\naws_secret_access_key=SECRET2\n")
	require.NoError(t, err)
	err = credsFile.Close()
	require.NoError(t, err)
	return credsFile, err
}

func TestCreateV2Configs(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	defer os.Unsetenv("AWS_REGION")
	defer os.Unsetenv("AWS_EC2_METADATA_DISABLED")

	t.Run("returns default profile when none configured", func(t *testing.T) {
		// setup
		os.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
		os.Setenv("AWS_SECRET_ACCESS_KEY", "topsecret")
		defer os.Unsetenv("AWS_ACCESS_KEY_ID")
		defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

		cfg := &externaldns.Config{
			AWSAPIRetries: 3,
		}

		// when
		configs := CreateV2Configs(cfg)

		// then
		require.Len(t, configs, 1)
		_, ok := configs[defaultAWSProfile]
		assert.True(t, ok)
	})

	t.Run("returns profile configs when configured", func(t *testing.T) {
		// setup
		credsFile, err := prepareCredentialsFile(t)
		defer os.Remove(credsFile.Name())
		require.NoError(t, err)
		os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsFile.Name())
		defer os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")

		cfg := &externaldns.Config{
			AWSProfiles:   []string{"profile1", "profile2"},
			AWSAPIRetries: 2,
		}

		// when
		configs := CreateV2Configs(cfg)

		// then
		require.Len(t, configs, 2)
		creds, err := configs["profile1"].Credentials.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "AKID1234", creds.AccessKeyID)
		assert.Equal(t, "SECRET1", creds.SecretAccessKey)

		creds, err = configs["profile2"].Credentials.Retrieve(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "AKID2345", creds.AccessKeyID)
		assert.Equal(t, "SECRET2", creds.SecretAccessKey)
	})
}

func TestCreateConfigFatalOnError(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	defer os.Unsetenv("AWS_REGION")
	defer os.Unsetenv("AWS_EC2_METADATA_DISABLED")

	t.Run("CreateDefaultV2Config exits on load error", func(t *testing.T) {
		os.Setenv("AWS_CA_BUNDLE", "missing-ca.pem")
		defer os.Unsetenv("AWS_CA_BUNDLE")

		exitCode := 0
		restore := withLogrusExitFunc(func(code int) {
			exitCode = code
			panic("exit")
		})
		defer restore()

		assert.Panics(t, func() {
			CreateDefaultV2Config(&externaldns.Config{})
		})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("CreateV2Configs exits on load error", func(t *testing.T) {
		os.Setenv("AWS_CA_BUNDLE", "missing-ca.pem")
		defer os.Unsetenv("AWS_CA_BUNDLE")

		exitCode := 0
		restore := withLogrusExitFunc(func(code int) {
			exitCode = code
			panic("exit")
		})
		defer restore()

		assert.Panics(t, func() {
			CreateV2Configs(&externaldns.Config{AWSProfiles: []string{"profile1"}})
		})
		assert.Equal(t, 1, exitCode)
	})
}

func withLogrusExitFunc(exitFunc func(int)) func() {
	logger := logrus.StandardLogger()
	previousExitFunc := logger.ExitFunc
	logger.ExitFunc = exitFunc
	return func() {
		logger.ExitFunc = previousExitFunc
	}
}
