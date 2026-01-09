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

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// CreateKubeInformerFactory creates a shared informer factory with namespace filtering.
// The resync period is set to 0 to prevent unnecessary resyncs.
// If namespace is empty, the factory will watch all namespaces.
func CreateKubeInformerFactory(kubeClient kubernetes.Interface, namespace string) informers.SharedInformerFactory {
	return informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		0,
		informers.WithNamespace(namespace),
	)
}

// StartAndSyncInformerFactory starts an informer factory and waits for cache synchronization.
// It returns an error if the cache sync fails or the context is cancelled.
func StartAndSyncInformerFactory(ctx context.Context, factory informers.SharedInformerFactory) error {
	factory.Start(ctx.Done())
	return WaitForCacheSync(ctx, factory)
}
