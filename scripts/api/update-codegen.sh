#!/usr/bin/env bash

# https://github.com/kubernetes-sigs/kueue/blob/main/hack/update-codegen.sh

set -o errexit
set -o nounset
set -o pipefail

GO_CMD=${1:-go}
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
KUEUE_ROOT=$(realpath "${CURRENT_DIR}/..")
KUEUE_PKG="sigs.k8s.io/kueue"
CODEGEN_PKG=$(cd "${TOOLS_DIR}"; $GO_CMD list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)

cd "$CURRENT_DIR/.."

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

# Generating conversion and defaults functions
kube::codegen::gen_helpers \
  --boilerplate "${KUEUE_ROOT}/hack/boilerplate.go.txt" \
  "${KUEUE_ROOT}/apis"

# Generating OpenAPI for Kueue API extensions
kube::codegen::gen_openapi \
  --boilerplate "${KUEUE_ROOT}/hack/boilerplate.go.txt" \
  --output-dir "${KUEUE_ROOT}/apis/visibility/openapi" \
  --output-pkg "${KUEUE_PKG}/apis/visibility/openapi" \
  --update-report \
  "${KUEUE_ROOT}/apis/visibility"

externals=(
  "k8s.io/api/core/v1.PodTemplateSpec:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.Taint:k8s.io/client-go/applyconfigurations/core/v1"
  "k8s.io/api/core/v1.Toleration:k8s.io/client-go/applyconfigurations/core/v1"
)

apply_config_externals="${externals[0]}"
for external in "${externals[@]:1}"; do
  apply_config_externals="${apply_config_externals},${external}"
done

kube::codegen::gen_client \
  --boilerplate "${KUEUE_ROOT}/hack/boilerplate.go.txt" \
  --output-dir "${KUEUE_ROOT}/client-go" \
  --output-pkg "${KUEUE_PKG}/client-go" \
  --with-watch \
  --with-applyconfig \
  --applyconfig-externals "${apply_config_externals}" \
  "${KUEUE_ROOT}/apis"
