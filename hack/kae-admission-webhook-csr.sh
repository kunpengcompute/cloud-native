#!/usr/bin/env bash

set -euo pipefail

umask 077

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "${script_dir}/.." && pwd)

NAMESPACE=${NAMESPACE:-kae-system}
RELEASE_NAME=${RELEASE_NAME:-kae-device-plugin}
CHART_PATH=${CHART_PATH:-${repo_root}/charts/kae-device-plugin}
CSR_NAME=${CSR_NAME:-kae-admission-webhook}
SERVICE_NAME=${SERVICE_NAME:-kae-admission-webhook}
TLS_SECRET_NAME=${TLS_SECRET_NAME:-kae-admission-webhook-tls}
WORK_DIR=${WORK_DIR:-/tmp/kae-admission-webhook-csr}
WAIT_TIMEOUT_SECONDS=${WAIT_TIMEOUT_SECONDS:-300}
WAIT_INTERVAL_SECONDS=${WAIT_INTERVAL_SECONDS:-2}
KEEP_WORK_DIR=${KEEP_WORK_DIR:-false}

KUBECTL_BIN=${KUBECTL_BIN:-kubectl}
HELM_BIN=${HELM_BIN:-helm}
OPENSSL_BIN=${OPENSSL_BIN:-openssl}
BASE64_BIN=${BASE64_BIN:-base64}

private_key_file="${WORK_DIR}/tls.key"
csr_file="${WORK_DIR}/request.csr"
openssl_config_file="${WORK_DIR}/openssl.cnf"
certificate_file="${WORK_DIR}/tls.crt"

usage() {
    cat <<EOF
Usage:
  $0 request
  CLUSTER_SIGNING_CA_FILE=/path/to/ca.crt $0 install [additional helm arguments]

Workflow:
  1. Run '$0 request'.
  2. Review and approve the CSR with 'kubectl certificate approve ${CSR_NAME}'.
  3. Run '$0 install' with CLUSTER_SIGNING_CA_FILE set.

Optional environment variables:
  NAMESPACE, RELEASE_NAME, CHART_PATH, CSR_NAME, SERVICE_NAME,
  TLS_SECRET_NAME, WORK_DIR, WAIT_TIMEOUT_SECONDS, WAIT_INTERVAL_SECONDS,
  KEEP_WORK_DIR, KUBECTL_BIN, HELM_BIN, OPENSSL_BIN, BASE64_BIN.
EOF
}

fail() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

base64_file() {
    "${BASE64_BIN}" < "$1" | tr -d '\n'
}

ensure_namespace() {
    if ! "${KUBECTL_BIN}" get namespace "${NAMESPACE}" >/dev/null 2>&1; then
        "${KUBECTL_BIN}" create namespace "${NAMESPACE}"
    fi
}

create_request() {
    require_command "${KUBECTL_BIN}"
    require_command "${OPENSSL_BIN}"
    require_command "${BASE64_BIN}"

    ensure_namespace
    if "${KUBECTL_BIN}" get csr "${CSR_NAME}" >/dev/null 2>&1; then
        fail "CSR ${CSR_NAME} already exists; inspect or delete it before creating a new request"
    fi
    if [[ -e "${private_key_file}" || -e "${csr_file}" ]]; then
        fail "work directory ${WORK_DIR} already contains CSR state; remove it or choose another WORK_DIR"
    fi

    mkdir -p "${WORK_DIR}"
    chmod 700 "${WORK_DIR}"

    cat > "${openssl_config_file}" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = ${SERVICE_NAME}.${NAMESPACE}.svc

[v3_req]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
EOF

    "${OPENSSL_BIN}" genrsa -out "${private_key_file}" 2048 >/dev/null 2>&1
    "${OPENSSL_BIN}" req -new \
        -key "${private_key_file}" \
        -out "${csr_file}" \
        -config "${openssl_config_file}"

    request_bundle=$(base64_file "${csr_file}")
    cat <<EOF | "${KUBECTL_BIN}" apply -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${CSR_NAME}
spec:
  groups:
    - system:authenticated
  request: ${request_bundle}
  usages:
    - digital signature
    - key encipherment
    - server auth
EOF

    cat <<EOF
CSR ${CSR_NAME} created.
Review the request, then approve it manually:

  kubectl get csr ${CSR_NAME} -o yaml
  kubectl certificate approve ${CSR_NAME}

After approval, run:

  CLUSTER_SIGNING_CA_FILE=/path/to/cluster-signing-ca.crt $0 install
EOF
}

wait_for_certificate() {
    [[ "${WAIT_TIMEOUT_SECONDS}" =~ ^[0-9]+$ ]] || fail "WAIT_TIMEOUT_SECONDS must be a non-negative integer"
    [[ "${WAIT_INTERVAL_SECONDS}" =~ ^[0-9]+$ ]] || fail "WAIT_INTERVAL_SECONDS must be a non-negative integer"

    start_time=$(date +%s)
    while true; do
        conditions=$("${KUBECTL_BIN}" get csr "${CSR_NAME}" \
            -o jsonpath='{range .status.conditions[*]}{.type}{"\n"}{end}' 2>/dev/null || true)
        if grep -Eq 'Denied|Failed' <<<"${conditions}"; then
            fail "CSR ${CSR_NAME} was denied or failed"
        fi

        certificate_bundle=$("${KUBECTL_BIN}" get csr "${CSR_NAME}" \
            -o jsonpath='{.status.certificate}' 2>/dev/null || true)
        if [[ -n "${certificate_bundle}" ]]; then
            printf '%s' "${certificate_bundle}" | "${BASE64_BIN}" --decode > "${certificate_file}"
            return
        fi

        now=$(date +%s)
        if (( now - start_time >= WAIT_TIMEOUT_SECONDS )); then
            fail "timed out waiting for CSR ${CSR_NAME} to be signed"
        fi
        sleep "${WAIT_INTERVAL_SECONDS}"
    done
}

validate_certificate() {
    "${OPENSSL_BIN}" x509 -in "${certificate_file}" -noout >/dev/null
    "${OPENSSL_BIN}" verify -CAfile "${CLUSTER_SIGNING_CA_FILE}" "${certificate_file}" >/dev/null
    "${OPENSSL_BIN}" x509 -in "${certificate_file}" -noout -text | \
        grep -Fq "DNS:${SERVICE_NAME}.${NAMESPACE}.svc" || \
        fail "signed certificate does not contain the required Service DNS SAN"

    key_digest=$("${OPENSSL_BIN}" pkey -in "${private_key_file}" -pubout 2>/dev/null | \
        "${OPENSSL_BIN}" dgst -sha256)
    cert_digest=$("${OPENSSL_BIN}" x509 -in "${certificate_file}" -pubkey -noout | \
        "${OPENSSL_BIN}" pkey -pubin -pubout 2>/dev/null | \
        "${OPENSSL_BIN}" dgst -sha256)
    [[ "${key_digest}" == "${cert_digest}" ]] || fail "signed certificate does not match the generated private key"
}

apply_tls_secret() {
    tls_cert=$(base64_file "${certificate_file}")
    tls_key=$(base64_file "${private_key_file}")
    cat <<EOF | "${KUBECTL_BIN}" apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${TLS_SECRET_NAME}
  namespace: ${NAMESPACE}
type: kubernetes.io/tls
data:
  tls.crt: ${tls_cert}
  tls.key: ${tls_key}
EOF
}

cleanup_work_dir() {
    if [[ "${KEEP_WORK_DIR}" == "true" ]]; then
        printf 'Keeping CSR files in %s\n' "${WORK_DIR}"
        return
    fi
    rm -f -- "${private_key_file}" "${csr_file}" "${openssl_config_file}" "${certificate_file}"
    if ! rmdir -- "${WORK_DIR}" 2>/dev/null; then
        printf 'Work directory is not empty and was kept: %s\n' "${WORK_DIR}" >&2
    fi
}

install_release() {
    require_command "${KUBECTL_BIN}"
    require_command "${HELM_BIN}"
    require_command "${OPENSSL_BIN}"
    require_command "${BASE64_BIN}"

    [[ -n "${CLUSTER_SIGNING_CA_FILE:-}" ]] || fail "CLUSTER_SIGNING_CA_FILE must be set"
    [[ -f "${CLUSTER_SIGNING_CA_FILE}" ]] || fail "cluster signing CA not found: ${CLUSTER_SIGNING_CA_FILE}"
    [[ -f "${private_key_file}" && -f "${csr_file}" && -f "${openssl_config_file}" ]] || \
        fail "CSR state not found in ${WORK_DIR}; run '$0 request' first"
    [[ -d "${CHART_PATH}" ]] || fail "Helm chart not found: ${CHART_PATH}"
    "${KUBECTL_BIN}" get csr "${CSR_NAME}" >/dev/null 2>&1 || fail "CSR ${CSR_NAME} does not exist"

    wait_for_certificate
    validate_certificate
    apply_tls_secret

    ca_bundle=$(base64_file "${CLUSTER_SIGNING_CA_FILE}")
    "${HELM_BIN}" upgrade --install "${RELEASE_NAME}" "${CHART_PATH}" \
        --namespace "${NAMESPACE}" \
        --set admissionWebhook.enabled=true \
        --set admissionWebhook.cert.mode=manual \
        --set-string "admissionWebhook.cert.caBundle=${ca_bundle}" \
        "$@"

    cleanup_work_dir
    printf 'KAE Device Plugin and admission webhook installed successfully\n'
}

action=${1:-}
case "${action}" in
request)
    [[ "$#" -eq 1 ]] || fail "request does not accept additional arguments"
    create_request
    ;;
install)
    shift
    install_release "$@"
    ;;
-h | --help | help)
    usage
    ;;
*)
    usage >&2
    exit 1
    ;;
esac
