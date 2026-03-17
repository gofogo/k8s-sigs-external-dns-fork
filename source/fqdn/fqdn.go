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
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
//
// Logic:
//   - If no template is configured, returns original endpoints unchanged
//   - If the engine was created with combineFQDN=true, appends templated endpoints to existing
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

type kubeObject interface {
	runtime.Object
	metav1.Object
}
