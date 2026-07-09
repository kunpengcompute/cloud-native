# KAE Device Plugin Helm Chart

该 Chart 用于部署 Kunpeng KAE Device Plugin，并可选在同一个 DaemonSet 进程中启用 KAE Admission Webhook。完整证书和排障说明请参见 [KAE Admission Webhook 使用指南](../../../../docs/zh/kae_admission_webhook_user_guide.md)。

## 环境要求

- Kubernetes 1.16 或更高版本。
- Helm 3，建议 Helm 3.8 或更高版本。
- 使用 cert-manager 模式时，集群必须已安装 cert-manager 和 cainjector。

Chart 不创建 Namespace。可以通过 `--create-namespace` 创建 release namespace，也可以安装到已经存在的 namespace。

## 仅部署 Device Plugin

Webhook 默认关闭：

```bash
helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --create-namespace
```

namespace 已存在时：

```bash
helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system
```

## 使用 Kubernetes 1.16 CSR 脚本启用 Webhook

Chart 提供两阶段脚本简化 CSR、TLS Secret 和 Helm 安装流程。CSR 审批不会自动执行。

```bash
./hack/kae-admission-webhook-csr.sh request

kubectl get csr kae-admission-webhook -o yaml
kubectl certificate approve kae-admission-webhook

CLUSTER_SIGNING_CA_FILE=/path/to/cluster-signing-ca.crt \
  ./hack/kae-admission-webhook-csr.sh install
```

脚本会使用 `manual` 证书模式调用 `helm upgrade --install`。cluster signing CA 的路径取决于集群部署方式，必须由集群管理员确认。

## 使用手动证书启用 Webhook

服务端证书必须包含 `kae-admission-webhook.kae-system.svc` DNS SAN。

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

手动模式下，`admissionWebhook.cert.caBundle` 不能为空。

## 使用 cert-manager 启用 Webhook

```bash
helm install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --create-namespace \
  --set admissionWebhook.enabled=true \
  --set admissionWebhook.cert.mode=certManager \
  --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}' \
  --set-string admissionWebhook.injectEnvs='OPENSSL_ENGINES=/usr/local/lib/engines-1.1\,KAE_LOG_LEVEL=info'
```

Chart 会创建 Issuer、CA Certificate 和服务端 Certificate。cainjector 会自动更新 `MutatingWebhookConfiguration` 的 CA Bundle。

## 注入环境变量

```bash
helm upgrade --install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --set admissionWebhook.enabled=true \
  --set admissionWebhook.cert.mode=certManager \
  --set-string admissionWebhook.injectEnvs='KAE_MODE=auto\,KAE_LOG_LEVEL=info'
```

## 限制注入 namespace

```bash
helm upgrade --install kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --reuse-values \
  --set 'admissionWebhook.includedNamespaces={tenant-a,tenant-b}'
```

`includedNamespaces` 为空时按 `excludedNamespaces` 过滤；非空时只向 include 列表中的 namespace 注入。namespace 同时出现在两个列表中时，include 优先。

## 常用参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `image.repository` | `kae-device-plugin` | 镜像仓库 |
| `image.tag` | `1.0` | 镜像标签 |
| `devicePlugin.enabled` | `true` | 是否部署 Device Plugin |
| `devicePlugin.kernelVfDrivers` | `hisi_hpre,hisi_zip,hisi_sec2` | VF 驱动列表 |
| `admissionWebhook.enabled` | `false` | 是否启用 Webhook |
| `admissionWebhook.port` | `9443` | Webhook 监听端口 |
| `admissionWebhook.defaultKaeResource` | `hisi_hpre` | 默认 KAE 资源 |
| `admissionWebhook.defaultKaeCount` | `1` | 默认设备数量 |
| `admissionWebhook.targetContainerIndex` | `0` | 目标普通容器下标 |
| `admissionWebhook.injectEnvs` | `OPENSSL_ENGINES=/usr/local/lib/engines-1.1` | `KEY=VALUE` 环境变量列表 |
| `admissionWebhook.includedNamespaces` | `[]` | 允许注入的 namespace |
| `admissionWebhook.excludedNamespaces` | Kubernetes 系统 namespace | 排除的 namespace |
| `admissionWebhook.failurePolicy` | `Ignore` | Webhook 调用失败策略 |
| `admissionWebhook.cert.mode` | `manual` | `manual` 或 `certManager` |
| `admissionWebhook.cert.caBundle` | `""` | 手动模式下的 CA Bundle |

`admissionWebhook.enabled=true` 要求 `devicePlugin.enabled=true`。

## 升级和卸载

```bash
helm upgrade kae-device-plugin config/kae-device-plugin/charts/kae-device-plugin \
  --namespace kae-system \
  --reuse-values

helm uninstall kae-device-plugin --namespace kae-system
```
