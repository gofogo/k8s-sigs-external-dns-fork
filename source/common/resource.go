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

package common

import (
	"fmt"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/source/annotations"
)

// BuildResourceIdentifier constructs a resource identifier string in the format
// "resourceType/namespace/name" for use in logging and annotation lookups.
func BuildResourceIdentifier(resourceType, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
}

// GetTTLForResource extracts TTL from annotations using the proper resource identifier.
// It constructs the resource identifier in the format "resourceType/namespace/name"
// and uses it to look up the TTL annotation.
func GetTTLForResource(
	resourceAnnotations map[string]string,
	resourceType, namespace, name string,
) endpoint.TTL {
	return annotations.TTLFromAnnotations(
		resourceAnnotations,
		BuildResourceIdentifier(resourceType, namespace, name),
	)
}
