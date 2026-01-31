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

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

func TestLoadResources_updatesStatusWhenProvided(t *testing.T) {
	ctx := t.Context()

	toRawObject := func(_ *testing.T, obj k8sruntime.Object) k8sruntime.RawExtension {
		return k8sruntime.RawExtension{Object: obj}
	}

	t.Run("ingress status is persisted via UpdateStatus", func(t *testing.T) {
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "ing", Namespace: "default"},
			Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
				},
			},
		}

		client, err := LoadResources(ctx, Scenario{Resources: []ResourceWithDependencies{
			{Resource: toRawObject(t, ing)},
		}})
		require.NoError(t, err)

		got, err := client.NetworkingV1().Ingresses("default").Get(ctx, "ing", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, got.Status.LoadBalancer.Ingress, 1)
		assert.Equal(t, "1.2.3.4", got.Status.LoadBalancer.Ingress[0].IP)
	})

	t.Run("service status is persisted via UpdateStatus", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb.example.com"}},
				},
			},
		}

		client, err := LoadResources(ctx, Scenario{Resources: []ResourceWithDependencies{
			{Resource: toRawObject(t, svc)},
		}})
		require.NoError(t, err)

		got, err := client.CoreV1().Services("default").Get(ctx, "svc", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, got.Status.LoadBalancer.Ingress, 1)
		assert.Equal(t, "lb.example.com", got.Status.LoadBalancer.Ingress[0].Hostname)
	})

	t.Run("status is not updated when LB status is empty", func(t *testing.T) {
		ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "ing-empty", Namespace: "default"}}
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-empty", Namespace: "default"}}

		client, err := LoadResources(ctx, Scenario{Resources: []ResourceWithDependencies{
			{Resource: toRawObject(t, ing)},
			{Resource: toRawObject(t, svc)},
		}})
		require.NoError(t, err)

		gotIng, err := client.NetworkingV1().Ingresses("default").Get(ctx, "ing-empty", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, gotIng.Status.LoadBalancer.Ingress, 0)

		gotSvc, err := client.CoreV1().Services("default").Get(ctx, "svc-empty", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Len(t, gotSvc.Status.LoadBalancer.Ingress, 0)
	})
}
