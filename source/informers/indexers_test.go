package informers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestServiceIndexers_SpecSelectorIndex(t *testing.T) {
	svc := &corev1.Service{}
	svc.Spec.Selector = map[string]string{"app": "nginx", "env": "prod"}

	indexFunc := ServiceIndexers[SpecSelectorIndex]
	indexKeys, err := indexFunc(svc)

	assert.NoError(t, err)
	assert.Len(t, indexKeys, 1)
	assert.Equal(t, "app=nginx,env=prod", indexKeys[0])
}
