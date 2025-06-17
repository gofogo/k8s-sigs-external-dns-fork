package informers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	SpecSelectorIndex = "spec.selector"
)

var (
	ServiceIndexers = cache.Indexers{
		SpecSelectorIndex: func(obj any) ([]string, error) {
			svc, ok := obj.(*corev1.Service)
			if !ok {
				// not tested
				return nil, nil
			}
			return []string{labels.Set(svc.Spec.Selector).String()}, nil
		},
	}
)
