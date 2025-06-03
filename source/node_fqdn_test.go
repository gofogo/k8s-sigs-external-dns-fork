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

package source

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeSourceNewNodeSourceWithFqdn(t *testing.T) {
	for _, tt := range []struct {
		title            string
		annotationFilter string
		fqdnTemplate     string
		expectError      bool
	}{
		{
			title:        "invalid template",
			expectError:  true,
			fqdnTemplate: "{{.Name",
		},
		{
			title:       "valid empty template",
			expectError: false,
		},
		{
			title:        "valid template",
			expectError:  false,
			fqdnTemplate: "{{.Name}}-{{.Namespace}}.ext-dns.test.com",
		},
		{
			title:        "complex template",
			expectError:  false,
			fqdnTemplate: "{{range .Status.Addresses}}{{if and (eq .Type \"ExternalIP\") (isIPv4 .Address)}}{{.Address | replace \".\" \"-\"}}{{break}}{{end}}{{end}}.ext-dns.test.com",
		},
	} {
		t.Run(tt.title, func(t *testing.T) {
			_, err := NewNodeSource(
				t.Context(),
				fake.NewClientset(),
				tt.annotationFilter,
				tt.fqdnTemplate,
				labels.Everything(),
				true,
				true,
			)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestNodeSourceNewNodeSourceWithoutFqdn tests
func TestNodeSourceWithFqdn(t *testing.T) {
	nodes := generateTestFixtureNodes(2)

	kubernetes := fake.NewClientset()
	for _, node := range nodes.Items {
		_, err := kubernetes.CoreV1().Nodes().Create(t.Context(), &node, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	client, err := NewNodeSource(
		t.Context(),
		fake.NewClientset(),
		"",
		"",
		labels.Everything(),
		true,
		true,
	)
	assert.NoError(t, err)

	endpoints, err := client.Endpoints(t.Context())
	assert.NoError(t, err)
	for _, ep := range endpoints {
		fmt.Println(ep)
	}
}
