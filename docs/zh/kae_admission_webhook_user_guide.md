# KAE Admission Webhook 使用指南

## 功能说明

KAE Admission Webhook 与 KAE Device Plugin 运行在同一个 DaemonSet 进程中。Webhook 默认关闭，开启后会在 Pod 创建或更新时自动注入 KAE extended resource 和配置的环境变量，业务 Pod 无需手动声明 KAE resource。

每个就绪的 Device Plugin Pod 都是 Webhook Service 的一个 Endpoint。集群中只创建一个 Service 和一个 `MutatingWebhookConfiguration`，API Server 每次请求只会选择一个 Endpoint。

默认注入结果如下：

```yaml
resources:
  requests:
    kae.kunpeng.com/hisi_hpre: "1"
  limits:
    kae.kunpeng.com/hisi_hpre: "1"
```

默认行为：

- Webhook 默认关闭。
- 处理 Pod 的 CREATE 和 UPDATE 请求。
- 跳过 `kube-system`、`kube-public`、`kube-node-lease` 和 Webhook 自身 namespace。
- include 列表为空时按 exclude 列表过滤；include 列表非空时只向列表中的 namespace 注入。
- namespace 同时出现在 include 和 exclude 列表中时，include 优先。
- 默认修改第 0 个普通容器。
- Pod 已声明任意 `kae.kunpeng.com/*` resource 时，不再注入 resource。
- 只添加缺失的环境变量，不覆盖同名变量。
- Kustomize 和 Helm 默认使用 `failurePolicy: Ignore`，可根据集群可用性要求调整为 `Fail`。

## 环境要求

- Kubernetes 1.16 或更高版本。
- 使用 Helm 部署时需要 Helm 3，建议 Helm 3.8 或更高版本。
- KAE 节点已经创建 VF，并安装 KAE 驱动。
- API Server 可以访问节点 Pod 网络的 TCP 9443 端口。
- 已构建或拉取 `kae-device-plugin:1.0` 镜像。

Kubernetes 1.16 部署使用 `admissionregistration.k8s.io/v1` 的 `MutatingWebhookConfiguration`，AdmissionReview 协议使用 `v1beta1`。

## 启动参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--enable-admission-webhook` | `false` | 是否启用 Webhook |
| `--webhook-listen-addr` | `:9443` | HTTPS 监听地址 |
| `--webhook-tls-cert-file` | `/tls/tls.crt` | 服务端证书路径 |
| `--webhook-tls-key-file` | `/tls/tls.key` | 服务端私钥路径 |
| `--webhook-default-kae-resource` | `hisi_hpre` | 默认注入的 KAE 资源 |
| `--webhook-default-kae-count` | `1` | 默认注入数量 |
| `--webhook-target-container-index` | `0` | 目标普通容器下标 |
| `--webhook-inject-envs` | `OPENSSL_ENGINES=/usr/local/lib/engines-1.1` | 注入的 `KEY=VALUE` 列表 |
| `--webhook-included-namespaces` | 空 | 允许注入的 namespace 列表 |
| `--webhook-excluded-namespaces` | Kubernetes 系统 namespace | 排除的 namespace 列表 |

Webhook 开启后，如果配置无效、证书加载失败或 controller-runtime Manager 运行失败，整个 Device Plugin 进程退出并由 DaemonSet 重新拉起。

## TLS 证书要求

Admission Webhook 必须使用 HTTPS。服务端证书的 DNS SAN 至少包含：

```text
kae-admission-webhook
kae-admission-webhook.kae-system
kae-admission-webhook.kae-system.svc
```

TLS Secret 固定使用以下名称：

```text
kae-system/kae-admission-webhook-tls
```

`MutatingWebhookConfiguration.webhooks[0].clientConfig.caBundle` 必须包含签发服务端证书的 CA，而不是普通服务端叶子证书。

## 使用 Kustomize 部署

### 使用 Kubernetes 1.16 CSR API

Kubernetes 1.16 使用 `certificates.k8s.io/v1beta1`。该版本可以不填写 `signerName`；在 Kubernetes 1.18 之前，配置了 cluster signing CA 的 kube-controller-manager 会为已批准的 CSR 签发证书。详细行为参见 [Kubernetes CSR 文档](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)。

#### 使用部署脚本（推荐）

脚本将 CSR 创建和 Helm 安装拆成两个阶段，CSR 审批仍由管理员手工执行。

1. 创建 namespace、私钥和 CSR。

    ```bash
    ./hack/kae-admission-webhook-csr.sh request
    ```

    私钥和 CSR 默认保存在 `/tmp/kae-admission-webhook-csr`。CSR 创建完成后，脚本会输出检查和审批命令。

2. 检查并批准 CSR。

    ```bash
    kubectl get csr kae-admission-webhook -o yaml
    kubectl certificate approve kae-admission-webhook
    ```

3. 指定 kube-controller-manager 实际使用的 cluster signing CA，并完成 Secret 创建和 Helm 安装。

    ```bash
    CLUSTER_SIGNING_CA_FILE=/path/to/cluster-signing-ca.crt \
      ./hack/kae-admission-webhook-csr.sh install
    ```

    `install` 阶段会等待签发结果，校验证书 SAN、证书私钥匹配关系及 CA 信任关系，然后创建 TLS Secret，并执行 `helm upgrade --install`。安装成功后默认删除本地私钥目录；设置 `KEEP_WORK_DIR=true` 可以保留文件用于排障。

    可以通过环境变量修改默认配置：

    ```bash
    NAMESPACE=kae-system \
    RELEASE_NAME=kae-device-plugin \
    CSR_NAME=kae-admission-webhook \
    WORK_DIR=/tmp/kae-admission-webhook-csr \
      ./hack/kae-admission-webhook-csr.sh request
    ```

    额外 Helm 参数可以追加到 `install` 命令末尾：

    ```bash
    CLUSTER_SIGNING_CA_FILE=/path/to/cluster-signing-ca.crt \
      ./hack/kae-admission-webhook-csr.sh install \
      --set image.repository=example.com/kae-device-plugin \
      --set image.tag=1.0
    ```

    如果集群中已经存在由 Kustomize 或手工创建的同名 Service、DaemonSet 或 `MutatingWebhookConfiguration`，Helm 不会自动接管，需要先删除旧资源或补充 Helm ownership metadata。

#### 使用 CSR 证书和 Kustomize 部署

以下命令均在仓库根目录执行。

1. 创建 namespace。

    ```bash
    kubectl apply -f config/kae-device-plugin/overlay/webhook/namespace.yaml
    ```
  
2. 生成服务端私钥和 CSR。

    ```bash
    openssl genrsa -out /tmp/kae-admission-webhook.key 2048
    chmod 600 /tmp/kae-admission-webhook.key

    cat > /tmp/kae-admission-webhook-csr.conf <<'EOF'
    [req]
    distinguished_name = req_distinguished_name
    req_extensions = v3_req
    prompt = no

    [req_distinguished_name]
    CN = kae-admission-webhook.kae-system.svc

    [v3_req]
    basicConstraints = CA:FALSE
    keyUsage = digitalSignature, keyEncipherment
    extendedKeyUsage = serverAuth
    subjectAltName = @alt_names

    [alt_names]
    DNS.1 = kae-admission-webhook
    DNS.2 = kae-admission-webhook.kae-system
    DNS.3 = kae-admission-webhook.kae-system.svc
    EOF

    openssl req -new \
      -key /tmp/kae-admission-webhook.key \
      -out /tmp/kae-admission-webhook.csr \
      -config /tmp/kae-admission-webhook-csr.conf
    ```

3. 创建并批准 Kubernetes CSR。

    如果同名 CSR 已存在且不再使用，先执行：

    ```bash
    kubectl delete csr kae-admission-webhook
    ```

    ```bash
    CSR_BUNDLE=$(base64 -w0 /tmp/kae-admission-webhook.csr)

    cat <<EOF | kubectl apply -f -
    apiVersion: certificates.k8s.io/v1beta1
    kind: CertificateSigningRequest
    metadata:
      name: kae-admission-webhook
    spec:
      groups:
        - system:authenticated
      request: ${CSR_BUNDLE}
      usages:
        - digital signature
        - key encipherment
        - server auth
    EOF

    kubectl certificate approve kae-admission-webhook
    ```

4. 导出签发后的服务端证书。

    ```bash
    kubectl get csr kae-admission-webhook

    kubectl get csr kae-admission-webhook \
      -o jsonpath='{.status.certificate}' | base64 -d \
      > /tmp/kae-admission-webhook.crt
    ```

5. 创建 TLS Secret。

    ```bash
    kubectl -n kae-system create secret tls kae-admission-webhook-tls \
      --cert=/tmp/kae-admission-webhook.crt \
      --key=/tmp/kae-admission-webhook.key
    ```

6. 选择部署方式。

    TLS Secret 创建完成后，可以使用 Kustomize 或 Helm 部署。两种方式都会创建 Device Plugin、Webhook Service 和 `MutatingWebhookConfiguration`，不要在同一个集群中同时使用，避免同名资源冲突。

    使用 Kustomize 部署：

    ```bash
    kubectl apply -k config/kae-device-plugin/overlay/webhook
    ```

    使用 Helm 部署：
    传入的参数可以根据实际需要进行修改。

    ```bash
    CLUSTER_SIGNING_CA_FILE=/path/to/your-cluster-signing-ca.crt
    CA_BUNDLE=$(base64 -w0 "${CLUSTER_SIGNING_CA_FILE}")

    helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
      --namespace kae-system \
      --set admissionWebhook.enabled=true \
      --set admissionWebhook.cert.mode=manual \
      --set-string admissionWebhook.cert.caBundle="${CA_BUNDLE}" \
      --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}' \
      --set-string admissionWebhook.injectEnvs='OPENSSL_ENGINES=/usr/local/lib/engines-1.1\,KAE_LOG_LEVEL=info'
    ```

7. 如果第 6 步使用 Kustomize，将签发 CA 写入 `MutatingWebhookConfiguration`。如果第 6 步已经使用 Helm 并传入 `admissionWebhook.cert.caBundle`，则不需要再执行本步骤。

    `CLUSTER_SIGNING_CA_FILE` 应指向 kube-controller-manager 的 `--cluster-signing-cert-file`。

    ```bash
    CLUSTER_SIGNING_CA_FILE=/path/to/cluster-signing-ca.crt
    CA_BUNDLE=$(base64 -w0 "${CLUSTER_SIGNING_CA_FILE}")

    kubectl patch mutatingwebhookconfiguration kae-admission-webhook \
      --type=json \
      -p="[{'op':'replace','path':'/webhooks/0/clientConfig/caBundle','value':'${CA_BUNDLE}'}]"
    ```

### 使用自签名证书

1. 创建 namespace。

    ```bash
    kubectl apply -f config/kae-device-plugin/overlay/webhook/namespace.yaml
    ```

2. 创建 CA。

    ```bash
    openssl genrsa -out /tmp/kae-webhook-ca.key 2048
    openssl req -x509 -new -nodes \
      -key /tmp/kae-webhook-ca.key \
      -sha256 -days 3650 \
      -subj "/CN=kae-admission-webhook-ca" \
      -out /tmp/kae-webhook-ca.crt
    ```

3. 创建服务端私钥和 CSR 配置。

    ```bash
    openssl genrsa -out /tmp/kae-admission-webhook.key 2048

    cat > /tmp/kae-admission-webhook-csr.conf <<'EOF'
    [req]
    distinguished_name = req_distinguished_name
    req_extensions = v3_req
    prompt = no

    [req_distinguished_name]
    CN = kae-admission-webhook.kae-system.svc

    [v3_req]
    basicConstraints = CA:FALSE
    keyUsage = digitalSignature, keyEncipherment
    extendedKeyUsage = serverAuth
    subjectAltName = @alt_names

    [alt_names]
    DNS.1 = kae-admission-webhook
    DNS.2 = kae-admission-webhook.kae-system
    DNS.3 = kae-admission-webhook.kae-system.svc
    EOF

    openssl req -new \
      -key /tmp/kae-admission-webhook.key \
      -out /tmp/kae-admission-webhook.csr \
      -config /tmp/kae-admission-webhook-csr.conf
    ```

4. 使用 CA 签发服务端证书。

    ```bash
    openssl x509 -req \
      -in /tmp/kae-admission-webhook.csr \
      -CA /tmp/kae-webhook-ca.crt \
      -CAkey /tmp/kae-webhook-ca.key \
      -CAcreateserial \
      -out /tmp/kae-admission-webhook.crt \
      -days 365 -sha256 \
      -extensions v3_req \
      -extfile /tmp/kae-admission-webhook-csr.conf
    ```

5. 创建 TLS Secret 并部署。

    ```bash
    kubectl -n kae-system create secret tls kae-admission-webhook-tls \
      --cert=/tmp/kae-admission-webhook.crt \
      --key=/tmp/kae-admission-webhook.key

    kubectl apply -k config/kae-device-plugin/overlay/webhook
    ```

6. 写入 CA Bundle。

    ```bash
    CA_BUNDLE=$(base64 -w0 /tmp/kae-webhook-ca.crt)

    kubectl patch mutatingwebhookconfiguration kae-admission-webhook \
      --type=json \
      -p="[{'op':'replace','path':'/webhooks/0/clientConfig/caBundle','value':'${CA_BUNDLE}'}]"
    ```

    Kustomize 清单中的 `caBundle` 默认为空。默认 `failurePolicy` 为 `Ignore`，因此 CA patch 完成前 Webhook 不会阻塞 Pod 创建，但也不会完成 KAE 注入，应尽快写入正确的 CA Bundle。

## 使用 Helm 部署

### 仅部署 Device Plugin

Webhook 默认关闭：

```bash
helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --create-namespace
```

如果 `kae-system` 已存在，省略 `--create-namespace`。

### 使用手动证书启用 Webhook

先创建 namespace 和 TLS Secret，然后将 CA Bundle 传给 Chart：

```bash
kubectl create namespace kae-system

kubectl -n kae-system create secret tls kae-admission-webhook-tls \
  --cert=/path/to/tls.crt \
  --key=/path/to/tls.key

CA_BUNDLE=$(base64 -w0 /path/to/ca.crt)

helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --set admissionWebhook.enabled=true \
  --set admissionWebhook.cert.mode=manual \
  --set-string admissionWebhook.cert.caBundle="${CA_BUNDLE}" \
  --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}' \
  --set-string admissionWebhook.injectEnvs='OPENSSL_ENGINES=/usr/local/lib/engines-1.1\,KAE_LOG_LEVEL=info'
```

手动证书模式下，Chart 会在 `caBundle` 为空时拒绝渲染。

### 使用 cert-manager 启用 Webhook

集群必须已经安装 cert-manager、cert-manager webhook 和 cainjector。Chart 会创建 Issuer、CA Certificate 和服务端 Certificate，并由 cainjector 写入 CA Bundle。

```bash
helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --create-namespace \
  --set admissionWebhook.enabled=true \
  --set admissionWebhook.cert.mode=certManager \
  --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}' \
  --set-string admissionWebhook.injectEnvs='OPENSSL_ENGINES=/usr/local/lib/engines-1.1\,KAE_LOG_LEVEL=info'
```

如果 namespace 已存在，省略 `--create-namespace`。

## 修改注入配置

Kustomize 参数位于：

```text
config/kae-device-plugin/overlay/webhook/daemonset_patch.yaml
```

例如注入 ZIP 资源和环境变量：

```yaml
- -webhook-default-kae-resource=hisi_zip
- -webhook-default-kae-count=1
- -webhook-inject-envs=KAE_MODE=auto,KAE_LOG_LEVEL=info
- -webhook-included-namespaces=tenant-a,tenant-b
```

Helm 可以通过 values 或命令行设置：

```bash
helm upgrade --install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --set admissionWebhook.enabled=true \
  --set admissionWebhook.cert.mode=certManager \
  --set admissionWebhook.defaultKaeResource=hisi_hpre \
  --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}' \
  --set-string admissionWebhook.injectEnvs='KAE_MODE=auto\,KAE_LOG_LEVEL=info'
```

不配置 `includedNamespaces` 时，Webhook 继续向 `excludedNamespaces` 之外的 namespace 注入。配置后只向 include 列表中的 namespace 注入；同名 include 条目优先于 exclude。

## 验证部署

1. 检查组件、Endpoint 和 CA Bundle。

    ```bash
    kubectl -n kae-system get daemonset,pod,service,endpoints
    kubectl get mutatingwebhookconfiguration kae-admission-webhook
    kubectl get mutatingwebhookconfiguration kae-admission-webhook \
      -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | wc -c
    ```

2. 创建测试 Pod。

    ```bash
    cat <<'EOF' | kubectl apply -f -
    apiVersion: v1
    kind: Pod
    metadata:
      name: kae-webhook-test
      namespace: default
    spec:
      containers:
        - name: test
          image: busybox
          command: ["sleep", "3600"]
    EOF
    ```

3. 查看注入结果并删除测试 Pod。

    ```bash
    kubectl get pod kae-webhook-test -n default -o yaml
    kubectl delete pod kae-webhook-test -n default
    ```

## 常见问题

### `x509: certificate signed by unknown authority`

检查 `caBundle` 是否为签发服务端证书的 CA，并检查证书 SAN 是否包含 Service DNS。

### `remote error: tls: bad certificate`

检查 TLS Secret 中的证书和私钥是否匹配：

```bash
kubectl -n kae-system get secret kae-admission-webhook-tls
openssl x509 -in /path/to/tls.crt -noout -text
```

### Service 没有 Endpoint

```bash
kubectl -n kae-system get pod -l app=kunpeng-kae-plugin
kubectl -n kae-system get endpoints kae-admission-webhook
```

确认 Pod Ready、9443 端口监听正常，并且 TLS Secret 已挂载。

### Pod 创建被阻塞

检查 Webhook Pod 日志、Service Endpoint、证书和 CA Bundle。如果需要强制阻止未完成注入的 Pod 创建，可将 `failurePolicy` 调整为 `Fail`。

## 卸载

Kustomize：

```bash
kubectl delete -k config/kae-device-plugin/overlay/webhook
```

Helm：

```bash
helm uninstall kae-device-plugin --namespace kae-system
```

手动创建的 TLS Secret 和 namespace 不会由 Helm 自动删除。
