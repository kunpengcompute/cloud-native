#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
script="${repo_root}/hack/kae-admission-webhook-csr.sh"
test_dir=$(mktemp -d)
trap 'rm -rf "${test_dir}"' EXIT

fake_bin="${test_dir}/bin"
mkdir -p "${fake_bin}"

cat > "${fake_bin}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$1" == "get" && "$2" == "namespace" ]]; then
    [[ -f "${FAKE_NAMESPACE_CREATED}" ]]
    exit
fi

if [[ "$1" == "create" && "$2" == "namespace" ]]; then
    touch "${FAKE_NAMESPACE_CREATED}"
    exit
fi

if [[ "$1" == "get" && "$2" == "csr" ]]; then
    args="$*"
    if [[ "${args}" == *"status.certificate"* ]]; then
        [[ -f "${FAKE_CERTIFICATE_B64}" ]] && cat "${FAKE_CERTIFICATE_B64}"
        exit
    fi
    if [[ "${args}" == *"status.conditions"* ]]; then
        [[ -f "${FAKE_CERTIFICATE_B64}" ]] && printf '%s\n' Approved
        exit
    fi
    [[ -f "${FAKE_CSR_CREATED}" ]]
    exit
fi

if [[ "$1" == "apply" && "$2" == "-f" && "$3" == "-" ]]; then
    manifest=$(cat)
    if grep -q "kind: CertificateSigningRequest" <<<"${manifest}"; then
        printf '%s\n' "${manifest}" > "${FAKE_CSR_MANIFEST}"
        touch "${FAKE_CSR_CREATED}"
    elif grep -q "kind: Secret" <<<"${manifest}"; then
        printf '%s\n' "${manifest}" > "${FAKE_SECRET_MANIFEST}"
    else
        printf 'unexpected manifest\n' >&2
        exit 1
    fi
    exit
fi

printf 'unexpected kubectl arguments: %s\n' "$*" >&2
exit 1
EOF

cat > "${fake_bin}/helm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "${FAKE_HELM_LOG}"
EOF

chmod +x "${fake_bin}/kubectl" "${fake_bin}/helm"

export PATH="${fake_bin}:${PATH}"
export FAKE_NAMESPACE_CREATED="${test_dir}/namespace-created"
export FAKE_CSR_CREATED="${test_dir}/csr-created"
export FAKE_CSR_MANIFEST="${test_dir}/csr.yaml"
export FAKE_SECRET_MANIFEST="${test_dir}/secret.yaml"
export FAKE_CERTIFICATE_B64="${test_dir}/certificate.b64"
export FAKE_HELM_LOG="${test_dir}/helm.log"

work_dir="${test_dir}/work"
request_output="${test_dir}/request.out"

NAMESPACE=kae-test \
CSR_NAME=kae-test-csr \
WORK_DIR="${work_dir}" \
"${script}" request > "${request_output}"

test -f "${work_dir}/tls.key"
test -f "${work_dir}/request.csr"
grep -q "apiVersion: certificates.k8s.io/v1beta1" "${FAKE_CSR_MANIFEST}"
grep -q "name: kae-test-csr" "${FAKE_CSR_MANIFEST}"
! grep -q "signerName" "${FAKE_CSR_MANIFEST}"
grep -q "kubectl certificate approve kae-test-csr" "${request_output}"
openssl req -in "${work_dir}/request.csr" -noout -text | grep -q "DNS:kae-admission-webhook.kae-test.svc"

if NAMESPACE=kae-test CSR_NAME=kae-test-csr WORK_DIR="${work_dir}" \
    "${script}" request > /dev/null 2> "${test_dir}/duplicate-request.err"; then
    printf 'duplicate request unexpectedly succeeded\n' >&2
    exit 1
fi
grep -q "already exists" "${test_dir}/duplicate-request.err"

if env -u CLUSTER_SIGNING_CA_FILE \
    NAMESPACE=kae-test CSR_NAME=kae-test-csr WORK_DIR="${work_dir}" \
    "${script}" install > /dev/null 2> "${test_dir}/missing-ca.err"; then
    printf 'install without cluster signing CA unexpectedly succeeded\n' >&2
    exit 1
fi
grep -q "CLUSTER_SIGNING_CA_FILE must be set" "${test_dir}/missing-ca.err"

ca_key="${test_dir}/ca.key"
ca_cert="${test_dir}/ca.crt"
openssl genrsa -out "${ca_key}" 2048 >/dev/null 2>&1
openssl req -x509 -new -key "${ca_key}" -sha256 -days 1 \
    -subj "/CN=test-cluster-signing-ca" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -out "${ca_cert}" >/dev/null 2>&1
openssl x509 -req -in "${work_dir}/request.csr" \
    -CA "${ca_cert}" -CAkey "${ca_key}" -CAcreateserial \
    -days 1 -sha256 \
    -extensions v3_req -extfile "${work_dir}/openssl.cnf" \
    -out "${test_dir}/tls.crt" >/dev/null 2>&1
base64 < "${test_dir}/tls.crt" | tr -d '\n' > "${FAKE_CERTIFICATE_B64}"

CLUSTER_SIGNING_CA_FILE="${ca_cert}" \
NAMESPACE=kae-test \
RELEASE_NAME=kae-test-release \
CSR_NAME=kae-test-csr \
WORK_DIR="${work_dir}" \
WAIT_TIMEOUT_SECONDS=2 \
WAIT_INTERVAL_SECONDS=0 \
"${script}" install

grep -q "name: kae-admission-webhook-tls" "${FAKE_SECRET_MANIFEST}"
grep -q "namespace: kae-test" "${FAKE_SECRET_MANIFEST}"
grep -q "type: kubernetes.io/tls" "${FAKE_SECRET_MANIFEST}"

ca_bundle=$(base64 < "${ca_cert}" | tr -d '\n')
grep -q "upgrade --install kae-test-release" "${FAKE_HELM_LOG}"
grep -q -- "--namespace kae-test" "${FAKE_HELM_LOG}"
grep -q -- "admissionWebhook.enabled=true" "${FAKE_HELM_LOG}"
grep -q -- "admissionWebhook.cert.mode=manual" "${FAKE_HELM_LOG}"
grep -q -- "admissionWebhook.cert.caBundle=${ca_bundle}" "${FAKE_HELM_LOG}"
test ! -e "${work_dir}"

printf 'CSR Helm deployment script tests passed\n'
