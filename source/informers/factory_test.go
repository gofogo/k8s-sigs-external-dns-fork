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

package informers

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateKubeInformerFactory(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "specific namespace",
			namespace: "default",
		},
		{
			name:      "empty namespace (all namespaces)",
			namespace: "",
		},
		{
			name:      "custom namespace",
			namespace: "kube-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			factory := CreateKubeInformerFactory(client, tt.namespace)

			if factory == nil {
				t.Error("CreateKubeInformerFactory() returned nil")
			}
		})
	}
}

func TestStartAndSyncInformerFactory(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{
			name:    "successful sync",
			timeout: 2 * time.Second,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			factory := CreateKubeInformerFactory(client, "")

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := StartAndSyncInformerFactory(ctx, factory)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartAndSyncInformerFactory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
