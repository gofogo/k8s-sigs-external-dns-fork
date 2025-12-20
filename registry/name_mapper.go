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

package registry

import (
	"strings"

	"sigs.k8s.io/external-dns/endpoint"
)

const recordTemplate = "%{record_type}"

// NameMapper is the interface for mapping between the endpoint for the source
// and the endpoint for the TXT record.
type NameMapper interface {
	ToEndpointName(string) (endpointName string, recordType string)
	ToTXTName(string, string) string
	RecordTypeInAffix() bool
}

// AffixNameMapper is a name mapper based on prefix/suffix affixes.
type AffixNameMapper struct {
	prefix              string
	suffix              string
	wildcardReplacement string
}

var _ NameMapper = AffixNameMapper{}

// NewAffixNameMapper returns a new AffixNameMapper.
func NewAffixNameMapper(prefix, suffix, wildcardReplacement string) AffixNameMapper {
	return AffixNameMapper{
		prefix:              strings.ToLower(prefix),
		suffix:              strings.ToLower(suffix),
		wildcardReplacement: strings.ToLower(wildcardReplacement),
	}
}

func getSupportedTypes() []string {
	return []string{
		endpoint.RecordTypeA,
		endpoint.RecordTypeAAAA,
		endpoint.RecordTypeCNAME,
		endpoint.RecordTypeNS,
		endpoint.RecordTypeMX,
		endpoint.RecordTypeSRV,
		endpoint.RecordTypeNAPTR,
	}
}

// extractRecordTypeDefaultPosition extracts record type from the default position
// when not using '%{record_type}' in the prefix/suffix.
func extractRecordTypeDefaultPosition(name string) (string, string) {
	nameS := strings.Split(name, "-")
	for _, t := range getSupportedTypes() {
		if nameS[0] == strings.ToLower(t) {
			return strings.TrimPrefix(name, nameS[0]+"-"), t
		}
	}
	return name, ""
}

// dropAffixExtractType strips TXT record to find an endpoint name it manages
// it also returns the record type.
func (pr AffixNameMapper) dropAffixExtractType(name string) (string, string) {
	prefix := pr.prefix
	suffix := pr.suffix

	if pr.RecordTypeInAffix() {
		for _, t := range getSupportedTypes() {
			tLower := strings.ToLower(t)
			iPrefix := strings.ReplaceAll(prefix, recordTemplate, tLower)
			iSuffix := strings.ReplaceAll(suffix, recordTemplate, tLower)

			if pr.isPrefix() && strings.HasPrefix(name, iPrefix) {
				return strings.TrimPrefix(name, iPrefix), t
			}

			if pr.isSuffix() && strings.HasSuffix(name, iSuffix) {
				return strings.TrimSuffix(name, iSuffix), t
			}
		}

		// handle old TXT records
		prefix = pr.dropAffixTemplate(prefix)
		suffix = pr.dropAffixTemplate(suffix)
	}

	if pr.isPrefix() && strings.HasPrefix(name, prefix) {
		return extractRecordTypeDefaultPosition(strings.TrimPrefix(name, prefix))
	}

	if pr.isSuffix() && strings.HasSuffix(name, suffix) {
		return extractRecordTypeDefaultPosition(strings.TrimSuffix(name, suffix))
	}

	return "", ""
}

func (pr AffixNameMapper) dropAffixTemplate(name string) string {
	return strings.ReplaceAll(name, recordTemplate, "")
}

func (pr AffixNameMapper) isPrefix() bool {
	return len(pr.suffix) == 0
}

func (pr AffixNameMapper) isSuffix() bool {
	return len(pr.prefix) == 0 && len(pr.suffix) > 0
}

// ToEndpointName converts the TXT record name to the managed endpoint name.
func (pr AffixNameMapper) ToEndpointName(txtDNSName string) (string, string) {
	lowerDNSName := strings.ToLower(txtDNSName)

	// drop prefix
	if pr.isPrefix() {
		return pr.dropAffixExtractType(lowerDNSName)
	}

	// drop suffix
	if pr.isSuffix() {
		dc := strings.Count(pr.suffix, ".")
		DNSName := strings.SplitN(lowerDNSName, ".", 2+dc)
		domainWithSuffix := strings.Join(DNSName[:1+dc], ".")

		r, rType := pr.dropAffixExtractType(domainWithSuffix)
		if !strings.Contains(lowerDNSName, ".") {
			return r, rType
		}
		return r + "." + DNSName[1+dc], rType
	}
	return "", ""
}

// RecordTypeInAffix reports whether the affix includes the record type template.
func (pr AffixNameMapper) RecordTypeInAffix() bool {
	if strings.Contains(pr.prefix, recordTemplate) {
		return true
	}
	if strings.Contains(pr.suffix, recordTemplate) {
		return true
	}
	return false
}

func (pr AffixNameMapper) normalizeAffixTemplate(afix, recordType string) string {
	if strings.Contains(afix, recordTemplate) {
		return strings.ReplaceAll(afix, recordTemplate, recordType)
	}
	return afix
}

// ToTXTName converts an endpoint name into its TXT record name.
func (pr AffixNameMapper) ToTXTName(endpointDNSName, recordType string) string {
	DNSName := strings.SplitN(endpointDNSName, ".", 2)
	recordType = strings.ToLower(recordType)
	recordT := recordType + "-"

	prefix := pr.normalizeAffixTemplate(pr.prefix, recordType)
	suffix := pr.normalizeAffixTemplate(pr.suffix, recordType)

	// If specified, replace a leading asterisk in the generated txt record name with some other string
	if pr.wildcardReplacement != "" && DNSName[0] == "*" {
		DNSName[0] = pr.wildcardReplacement
	}

	if !pr.RecordTypeInAffix() {
		DNSName[0] = recordT + DNSName[0]
	}

	if len(DNSName) < 2 {
		return prefix + DNSName[0] + suffix
	}

	return prefix + DNSName[0] + suffix + "." + DNSName[1]
}
