/*
Copyright 2026 The Kubernetes Authors.

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

package fqdn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/external-dns/endpoint"
)

func parseTemplate(input string) (*template.Template, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil //nolint:nilnil // nil template signals "not configured"; callers check IsConfigured()
	}
	funcs := template.FuncMap{
		"contains":   strings.Contains,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"trim":       strings.TrimSpace,
		"toLower":    strings.ToLower,
		"replace":    replace,
		"isIPv6":     isIPv6String,
		"isIPv4":     isIPv4String,
		"hasKey":     hasKey,
		"fromJson":   fromJson,
	}
	return template.New("endpoint").Funcs(funcs).Parse(input)
}

func execTemplate(tmpl *template.Template, obj kubeObject) ([]string, error) {
	if tmpl == nil {
		return []string{}, nil
	}
	if obj == nil {
		return nil, fmt.Errorf("object is nil")
	}
	// Kubernetes API doesn't populate TypeMeta (Kind/APIVersion) when retrieving
	// objects via informers, because the client already knows what type it requested.
	// Set it so templates can use .Kind and .APIVersion.
	// TODO: all sources to transform Informer().SetTransform()
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "" {
		gvks, _, err := scheme.Scheme.ObjectKinds(obj)
		if err == nil && len(gvks) > 0 {
			gvk = gvks[0]
		} else {
			// Fallback to reflection for types not in scheme
			gvk = schema.GroupVersionKind{Kind: reflect.TypeOf(obj).Elem().Name()}
		}
		obj.GetObjectKind().SetGroupVersionKind(gvk)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, obj); err != nil {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		return nil, fmt.Errorf("failed to apply template on %s %s/%s: %w", kind, obj.GetNamespace(), obj.GetName(), err)
	}
	hosts := strings.Split(buf.String(), ",")
	hostnames := make(map[string]struct{}, len(hosts))
	for _, name := range hosts {
		name = strings.TrimSpace(name)
		name = strings.TrimSuffix(name, ".")
		if name != "" {
			hostnames[name] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(hostnames)), nil
}

// replace all instances of oldValue with newValue in target string.
// adheres to syntax from https://masterminds.github.io/sprig/strings.html.
func replace(oldValue, newValue, target string) string {
	return strings.ReplaceAll(target, oldValue, newValue)
}

// isIPv6String reports whether the target string is an IPv6 address,
// including IPv4-mapped IPv6 addresses.
func isIPv6String(target string) bool {
	return endpoint.SuitableType(target) == endpoint.RecordTypeAAAA
}

// isIPv4String reports whether the target string is an IPv4 address.
func isIPv4String(target string) bool {
	return endpoint.SuitableType(target) == endpoint.RecordTypeA
}

// hasKey checks if a key exists in a map. This is needed because Go templates'
// `index` function returns the zero value ("") for missing keys, which is
// indistinguishable from keys with empty values. Kubernetes uses empty-value
// labels for markers (e.g., `service.kubernetes.io/headless: ""`), so we need
// explicit key existence checking.
func hasKey(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

// fromJson decodes a JSON string into a Go value (map, slice, etc.).
// This enables templates to work with structured data stored as JSON strings
// in complex labels or annotations or Configmap data fields, e.g. ranging over a list of entries:
//
//	{{ range $entry := (index .Data "entries" | fromJson) }}{{ index $entry "dns" }},{{ end }}
//
// Returns nil if the input is not valid JSON.
func fromJson(v string) any {
	var output any
	_ = json.Unmarshal([]byte(v), &output)
	return output
}
