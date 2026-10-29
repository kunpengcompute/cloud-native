// Package templatecreator 在本地 Linux host 上构建 E2B-native Template，
// 不要求部署 E2B control plane。
//
// PrepareRuntime 安装版本固定的 kernel、Firecracker、envd 和 busybox。
// NewCreator 持有进程级 NBD/network/proxy 资源；每次 CreateTemplate 调用
// 使用隔离的可写 storage 和真实 upstream Builder。只有完整 final/ancestor
// 对象闭包及其 completion manifest 都发布成功后，调用才返回成功。
package templatecreator
