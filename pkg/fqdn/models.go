package fqdn

type FqdnRegistry struct {
	Sources []*Fqdn
	mName   map[string]FeatureSpec
}

type Fqdn struct {
	Name string
	Spec FeatureSpec
}

type FeatureSpec struct {
	// Description is the description of the feature
	Description string
	// Example is the default examples
	Example []string
}
