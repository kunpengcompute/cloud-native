package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	templatecreator "gitcode.com/boostkit/cloud-native.git/TemplateCreator"
)

// cliDefaultTimeout 限制单个 runtime 资产下载所能占用的最长时间。
const cliDefaultTimeout = 5 * time.Minute

// prepareRuntimeUsage 是完整帮助页，architecture 和 timeout 直接复用实际默认值。
var prepareRuntimeUsage = fmt.Sprintf(`prepare-runtime 运行时资产准备命令

用途：
  根据版本化 manifest 下载并校验 TemplateCreator 所需的 runtime 资产。

用法：
  prepare-runtime -manifest <路径> -runtime-root <目录> [可选参数]

必填参数：
  -manifest <路径>      版本化 runtime manifest 的绝对路径
  -runtime-root <目录>  安装 runtime 资产的绝对目录

可选参数：
  -arch <架构>            目标架构（默认 %s）
  -e2b-infra-root <目录>  已安装 e2b-infra 文件系统的可选根目录；若提供，必须为绝对路径
  -timeout <时长>         每个 runtime 资产的下载超时（默认 %s）
  -h, --help              显示帮助信息

示例：
  sudo prepare-runtime \
    -manifest "$PWD/runtime-assets/manifest.json" \
    -runtime-root /opt/templatecreator/runtime \
    -arch "$(go env GOARCH)" \
    > /opt/templatecreator/runtime-info.json

结果：
  成功后向标准输出写入 runtime-info.json 内容；可按示例重定向保存，供 templatecreator-demo 使用。
`, runtime.GOARCH, cliDefaultTimeout)

func main() {
	log.SetFlags(0)
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// parseFlags 保留原有的参数解析入口，方便内部调用和单元测试复用。
func parseFlags(arguments []string) (templatecreator.PrepareRuntimeRequest, error) {
	request, _, err := parseFlagsWithHelp(arguments)
	return request, err
}

// parseFlagsWithHelp 先将 FlagSet 的诊断与帮助暂存到内存，避免普通参数错误污染标准输出。
func parseFlagsWithHelp(arguments []string) (templatecreator.PrepareRuntimeRequest, []byte, error) {
	flags := flag.NewFlagSet("prepare-runtime", flag.ContinueOnError)
	var flagOutput bytes.Buffer
	flags.SetOutput(&flagOutput)
	flags.Usage = func() {
		if _, writeErr := io.WriteString(flags.Output(), prepareRuntimeUsage); writeErr != nil {
			// usage 输出失败时 flag 包正走向退出，写标准错误兜底。
			_, _ = fmt.Fprintln(os.Stderr, writeErr)
		}
	}

	var request templatecreator.PrepareRuntimeRequest
	flags.StringVar(&request.ManifestPath, "manifest", "", "版本化 runtime manifest 的绝对路径")
	flags.StringVar(&request.RuntimeRoot, "runtime-root", "", "安装 runtime 资产的绝对目录")
	flags.StringVar(&request.Architecture, "arch", runtime.GOARCH, "目标架构，例如 amd64 或 arm64")
	flags.StringVar(&request.E2BInfraRoot, "e2b-infra-root", "", "包含已安装 e2b-infra 文件系统的可选根目录；若提供，必须为绝对路径")
	flags.DurationVar(&request.Timeout, "timeout", cliDefaultTimeout, "单个 runtime 资产的下载超时")
	if err := flags.Parse(arguments); err != nil {
		return templatecreator.PrepareRuntimeRequest{}, flagOutput.Bytes(), fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return templatecreator.PrepareRuntimeRequest{}, nil, fmt.Errorf("positional arguments are not supported: %v", flags.Args())
	}
	if request.ManifestPath == "" {
		return templatecreator.PrepareRuntimeRequest{}, nil, fmt.Errorf("-manifest is required")
	}
	if request.RuntimeRoot == "" {
		return templatecreator.PrepareRuntimeRequest{}, nil, fmt.Errorf("-runtime-root is required")
	}

	return request, nil, nil
}

// run 先处理无副作用的 help/参数路径，再准备资产并输出可供 Demo 读取的 JSON。
func run(ctx context.Context, arguments []string, output io.Writer) error {
	request, help, err := parseFlagsWithHelp(arguments)
	if errors.Is(err, flag.ErrHelp) {
		// help 属于正常退出路径；只有此时才把暂存文本复制到 stdout，并保留真实写入错误。
		_, writeErr := io.Copy(output, bytes.NewReader(help))
		return writeErr
	}
	if err != nil {
		return err
	}
	result, err := templatecreator.PrepareRuntime(ctx, request)
	if err != nil {
		return err
	}

	// stdout 是供后续命令读取的 JSON contract：仅暴露已验证路径和来源信息，
	// 不泄露本地复用源路径，也为未来可能加入的凭据字段保留安全边界。
	response := prepareRuntimeOutput{
		Architecture:    result.Runtime.Architecture,
		InfraCommit:     result.Runtime.InfraCommit,
		TemplateVersion: result.Runtime.TemplateVersion,
		RuntimeRoot:     result.Runtime.RuntimeDir,
		Assets:          make([]preparedAssetOutput, 0, len(result.Assets)),
	}
	for _, asset := range result.Assets {
		response.Assets = append(response.Assets, preparedAssetOutput{
			Type:    asset.Type,
			Version: asset.Version,
			Path:    asset.Path,
			SHA256:  asset.SHA256,
			Source:  asset.Source,
		})
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}

// prepareRuntimeOutput 是 CLI 对外稳定的最小 JSON schema，省略 SourcePath 等本机细节。
type prepareRuntimeOutput struct {
	// Architecture/InfraCommit/TemplateVersion/RuntimeRoot 描述整套 runtime ABI 与根位置。
	Architecture    string `json:"architecture"`
	InfraCommit     string `json:"infra_commit"`
	TemplateVersion string `json:"template_version"`
	RuntimeRoot     string `json:"runtime_root"`
	// Assets 列出四个实际安装并通过 checksum 的 runtime 文件。
	Assets []preparedAssetOutput `json:"assets"`
}

// preparedAssetOutput 是 CLI 可以安全公开的单个资产信息。
type preparedAssetOutput struct {
	// Type/Version 标识资产，Path 是最终绝对安装路径。
	Type    string `json:"type"`
	Version string `json:"version"`
	Path    string `json:"path"`
	// SHA256 验证内容，Source 说明采用 existing/bundled/local_reuse/download 中哪一种来源。
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}
