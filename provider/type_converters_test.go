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

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal int64
		want       int64
	}{
		{"valid positive", "42", 0, 42},
		{"valid negative", "-10", 0, -10},
		{"valid zero", "0", 99, 0},
		{"invalid returns default", "abc", 100, 100},
		{"empty returns default", "", 50, 50},
		{"float returns default", "3.14", 0, 0},
		{"large number", "9223372036854775807", 0, 9223372036854775807},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInt64(tt.value, tt.defaultVal, "test")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseInt32(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal int32
		want       int32
	}{
		{"valid positive", "42", 0, 42},
		{"valid negative", "-10", 0, -10},
		{"invalid returns default", "abc", 100, 100},
		{"overflow returns default", "9223372036854775807", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInt32(tt.value, tt.defaultVal, "test")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseUint32(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal uint32
		want       uint32
	}{
		{"valid positive", "42", 0, 42},
		{"valid zero", "0", 99, 0},
		{"invalid returns default", "abc", 100, 100},
		{"negative returns default", "-10", 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUint32(tt.value, tt.defaultVal, "test")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal float64
		want       float64
	}{
		{"valid positive", "3.14", 0, 3.14},
		{"valid negative", "-2.5", 0, -2.5},
		{"valid integer", "42", 0, 42.0},
		{"invalid returns default", "abc", 1.5, 1.5},
		{"empty returns default", "", 0.5, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFloat64(tt.value, tt.defaultVal, "test")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		defaultVal bool
		want       bool
	}{
		{"true lowercase", "true", false, true},
		{"false lowercase", "false", true, false},
		{"TRUE uppercase", "TRUE", false, true},
		{"1 is true", "1", false, true},
		{"0 is false", "0", true, false},
		{"invalid returns default true", "abc", true, true},
		{"invalid returns default false", "abc", false, false},
		{"empty returns default", "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBool(tt.value, tt.defaultVal, "test")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseInt64OrError(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{"valid", "42", 42, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInt64OrError(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseFloat64OrError(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    float64
		wantErr bool
	}{
		{"valid float", "3.14", 3.14, false},
		{"valid integer", "42", 42.0, false},
		{"invalid", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFloat64OrError(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
