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
	"sigs.k8s.io/external-dns/source/annotations"
)

// GetHostnamesFromAnnotations extracts hostnames from resource annotations.
// Returns an empty slice if ignoreHostnameAnnotation is true.
// This helper ensures consistent handling of the ignoreHostnameAnnotation flag.
func GetHostnamesFromAnnotations(
	resourceAnnotations map[string]string,
	ignoreHostnameAnnotation bool,
) []string {
	if ignoreHostnameAnnotation {
		return nil
	}
	return annotations.HostnamesFromAnnotations(resourceAnnotations)
}
