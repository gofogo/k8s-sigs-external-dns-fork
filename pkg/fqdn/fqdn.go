package fqdn

import (
	"strings"
	"sync/atomic"
	"text/template"
)

// TODO:
// - [ ] Add tests
// - [ ] Add docs
// - [ ] Add examples
// - [ ] Add validation

var (
	RegisterFqdn = NewFqdnRegister()
	featureGates = &atomic.Value{}
)

func NewFqdnRegister() *FqdnRegistry {
	return &FqdnRegistry{
		Sources: []*Fqdn{},
		mName:   make(map[string]FeatureSpec),
	}
}

type Template struct {
	fqdnTemplate *template.Template
}

func parseTemplate(fqdnTpl string) (*template.Template, error) {
	if fqdnTpl == "" {
		return nil, nil
	}
	// TODO: externalise this functions
	// TODO: should generate docs for each custom function
	funcs := template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
	}
	return template.New("endpoint").Funcs(funcs).Parse(fqdnTpl)
}

func ParseTemplate(input string) (*template.Template, error) {
	return parseTemplate(input)
}

func (m *FqdnRegistry) MustRegister(input Fqdn) {
	featureGates.Store(input)
	if _, exists := m.mName[input.Name]; exists {
		return
	} else {
		m.mName[input.Name] = input.Spec
	}
}
