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

// RecordTypeConfig provides a configuration-based approach for specifying
// which DNS record types a provider supports. This reduces boilerplate code
// across providers that previously had similar SupportedRecordType methods.
type RecordTypeConfig struct {
	// Additional contains record types supported beyond the base types.
	// Base types (A, AAAA, CNAME, SRV, TXT, NS) are always included.
	Additional []string
}

// NewRecordTypeConfig creates a new RecordTypeConfig with the specified
// additional record types beyond the base types.
func NewRecordTypeConfig(additional ...string) RecordTypeConfig {
	return RecordTypeConfig{
		Additional: additional,
	}
}

// Supports returns true if the given record type is supported.
// Base types (A, AAAA, CNAME, SRV, TXT, NS) are always supported.
// Additional types can be configured per provider.
func (c RecordTypeConfig) Supports(recordType string) bool {
	// Check base types first
	if SupportedRecordType(recordType) {
		return true
	}
	// Check additional types
	for _, t := range c.Additional {
		if t == recordType {
			return true
		}
	}
	return false
}

// DefaultRecordTypeConfig is the default configuration with no additional types.
var DefaultRecordTypeConfig = RecordTypeConfig{}

// MXRecordTypeConfig is a common configuration that adds MX support.
var MXRecordTypeConfig = NewRecordTypeConfig("MX")

// MXNAPTRRecordTypeConfig is a configuration that adds MX and NAPTR support (used by AWS).
var MXNAPTRRecordTypeConfig = NewRecordTypeConfig("MX", "NAPTR")
