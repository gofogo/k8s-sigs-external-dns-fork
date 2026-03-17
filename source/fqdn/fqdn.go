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

package fqdn

import (
	"bytes"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/external-dns/endpoint"
)

// TemplateEngine holds the three FQDN-related templates and the combine-with-annotation
// flag used across sources. The zero value is valid and represents an unconfigured engine.
type TemplateEngine struct {
	fqdn       *template.Template
	target     *template.Template
	fqdnTarget *template.Template
	combine    bool
}

// NewTemplateEngine parses all three template strings into a TemplateEngine.
// Empty strings produce nil inner templates (IsConfigured returns false).
// Returns an error if any non-empty template string fails to parse.
func NewTemplateEngine(fqdnStr, targetStr, fqdnTargetStr string, combineFQDN bool) (TemplateEngine, error) {
	fqdnTmpl, err := parseTemplate(fqdnStr)
	if err != nil {
		return TemplateEngine{}, fmt.Errorf("fqdn template: %w", err)
	}
	targetTmpl, err := parseTemplate(targetStr)
	if err != nil {
		return TemplateEngine{}, fmt.Errorf("target template: %w", err)
	}
	fqdnTargetTmpl, err := parseTemplate(fqdnTargetStr)
	if err != nil {
		return TemplateEngine{}, fmt.Errorf("fqdn-target template: %w", err)
	}
	return TemplateEngine{
		fqdn:       fqdnTmpl,
		target:     targetTmpl,
		fqdnTarget: fqdnTargetTmpl,
		combine:    combineFQDN,
	}, nil
}

// IsConfigured reports whether the FQDN template is set and ready to use.
func (e TemplateEngine) IsConfigured() bool {
	return e.fqdn != nil
}

// Combining reports whether the engine is configured to combine template-based
// endpoints with annotation-based endpoints.
func (e TemplateEngine) Combining() bool {
	return e.combine
}

// ExecFQDN executes the FQDN template against a Kubernetes object and returns hostnames.
func (e TemplateEngine) ExecFQDN(obj kubeObject) ([]string, error) {
	return execTemplate(e.fqdn, obj)
}

// ExecTarget executes the Target template against a Kubernetes object and returns targets.
func (e TemplateEngine) ExecTarget(obj kubeObject) ([]string, error) {
	return execTemplate(e.target, obj)
}

// ExecFQDNTarget executes the FQDNTarget template against a Kubernetes object and returns hostname:target pairs.
func (e TemplateEngine) ExecFQDNTarget(obj kubeObject) ([]string, error) {
	return execTemplate(e.fqdnTarget, obj)
}

// CombineWithEndpoints merges annotation-based endpoints with template-based endpoints.
func (e TemplateEngine) CombineWithEndpoints(
	endpoints []*endpoint.Endpoint,
	templateFunc func() ([]*endpoint.Endpoint, error),
) ([]*endpoint.Endpoint, error) {
	if e.fqdn == nil && e.target == nil && e.fqdnTarget == nil {
		return endpoints, nil
	}

	if !e.combine && len(endpoints) > 0 {
		return endpoints, nil
	}

	templatedEndpoints, err := templateFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints from template: %w", err)
	}

	if e.combine {
		return append(endpoints, templatedEndpoints...), nil
	}
	return templatedEndpoints, nil
}

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

type kubeObject interface {
	runtime.Object
	metav1.Object
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
