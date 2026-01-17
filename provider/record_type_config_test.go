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

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordTypeConfig_Supports(t *testing.T) {
	tests := []struct {
		name       string
		config     RecordTypeConfig
		recordType string
		want       bool
	}{
		// Default config (no additional types)
		{
			name:       "default config supports A",
			config:     DefaultRecordTypeConfig,
			recordType: "A",
			want:       true,
		},
		{
			name:       "default config supports AAAA",
			config:     DefaultRecordTypeConfig,
			recordType: "AAAA",
			want:       true,
		},
		{
			name:       "default config supports CNAME",
			config:     DefaultRecordTypeConfig,
			recordType: "CNAME",
			want:       true,
		},
		{
			name:       "default config supports TXT",
			config:     DefaultRecordTypeConfig,
			recordType: "TXT",
			want:       true,
		},
		{
			name:       "default config supports SRV",
			config:     DefaultRecordTypeConfig,
			recordType: "SRV",
			want:       true,
		},
		{
			name:       "default config supports NS",
			config:     DefaultRecordTypeConfig,
			recordType: "NS",
			want:       true,
		},
		{
			name:       "default config does not support MX",
			config:     DefaultRecordTypeConfig,
			recordType: "MX",
			want:       false,
		},
		{
			name:       "default config does not support NAPTR",
			config:     DefaultRecordTypeConfig,
			recordType: "NAPTR",
			want:       false,
		},
		{
			name:       "default config does not support PTR",
			config:     DefaultRecordTypeConfig,
			recordType: "PTR",
			want:       false,
		},

		// MX config
		{
			name:       "MX config supports A",
			config:     MXRecordTypeConfig,
			recordType: "A",
			want:       true,
		},
		{
			name:       "MX config supports MX",
			config:     MXRecordTypeConfig,
			recordType: "MX",
			want:       true,
		},
		{
			name:       "MX config does not support NAPTR",
			config:     MXRecordTypeConfig,
			recordType: "NAPTR",
			want:       false,
		},

		// MX+NAPTR config (AWS)
		{
			name:       "MX+NAPTR config supports A",
			config:     MXNAPTRRecordTypeConfig,
			recordType: "A",
			want:       true,
		},
		{
			name:       "MX+NAPTR config supports MX",
			config:     MXNAPTRRecordTypeConfig,
			recordType: "MX",
			want:       true,
		},
		{
			name:       "MX+NAPTR config supports NAPTR",
			config:     MXNAPTRRecordTypeConfig,
			recordType: "NAPTR",
			want:       true,
		},
		{
			name:       "MX+NAPTR config does not support PTR",
			config:     MXNAPTRRecordTypeConfig,
			recordType: "PTR",
			want:       false,
		},

		// Custom config
		{
			name:       "custom config supports custom type",
			config:     NewRecordTypeConfig("CAA", "DNSKEY"),
			recordType: "CAA",
			want:       true,
		},
		{
			name:       "custom config supports base types",
			config:     NewRecordTypeConfig("CAA"),
			recordType: "A",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.Supports(tt.recordType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewRecordTypeConfig(t *testing.T) {
	config := NewRecordTypeConfig("MX", "NAPTR", "CAA")

	assert.Equal(t, []string{"MX", "NAPTR", "CAA"}, config.Additional)
	assert.True(t, config.Supports("MX"))
	assert.True(t, config.Supports("NAPTR"))
	assert.True(t, config.Supports("CAA"))
	assert.True(t, config.Supports("A"))
	assert.False(t, config.Supports("PTR"))
}

func TestPredefinedConfigs(t *testing.T) {
	// Verify the predefined configs have expected additional types
	assert.Empty(t, DefaultRecordTypeConfig.Additional)
	assert.Equal(t, []string{"MX"}, MXRecordTypeConfig.Additional)
	assert.Equal(t, []string{"MX", "NAPTR"}, MXNAPTRRecordTypeConfig.Additional)
}
