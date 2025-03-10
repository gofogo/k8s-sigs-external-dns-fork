package fake

import (
	"sigs.k8s.io/external-dns/pkg/fqdn"
)

// TODO: move it to sources
func init() {
	fqdn.RegisterFqdn.MustRegister(fqdn.Fqdn{
		Name: "fake",
		Spec: fqdn.FeatureSpec{
			Description: "FQDN template for the 'fake' source. Default is 'example.com'",
			Example:     []string{"--fqdn-template=domain.org"},
		},
	})
}
