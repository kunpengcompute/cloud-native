#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

TAP_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
TAP_POLICY_MANAGER_ROOT="${TAP_ROOT}/api/policy-manager"

versions=("v1alpha1")

function generate_code() {
  API_VERSION="$1"
  API_PATH="${TAP_POLICY_MANAGER_ROOT}/${API_VERSION}"

  protoc \
  --proto_path="${API_PATH}" \
  --go_opt=paths=source_relative \
  --go_out="${API_PATH}" \
  --go-grpc_opt=paths=source_relative \
  --go-grpc_out="${API_PATH}" \
  "api.proto"
}

for v in "${versions[@]}"; do
  generate_code "${v}"
done