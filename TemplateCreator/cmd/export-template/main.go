package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
)

const commandUsage = `TemplateCreator 原始 Template 产物导出命令

用途：
  从 Local Store completion manifest 导出可传输的 Template 完整对象闭包。
  产物按 <TemplateID>/<BuildID>/ 目录组织，不生成压缩包。

用法：
  export-template -store-root <目录> -manifest <key> [-output <输出根目录>]

必填参数：
  -store-root <目录>  TemplateCreator 的 Local Store 绝对目录
  -manifest <key>     构建结果返回的 ManifestKey，例如 _templatecreator/manifests/<template>/<build>.json
  -output <目录>      原始 Template 导出根目录；默认使用 <store-root>/exports

示例：
  export-template \
    -store-root /opt/templatecreator/store \
    -manifest _templatecreator/manifests/<template-id>/<build-id>.json \
    -output /opt/templatecreator/exports

成功结果：
  stdout 输出 JSON，包含输出目录、Template ID、Build ID 和对象数量。
`

type options struct {
	StoreRoot   string
	ManifestKey string
	OutputRoot  string
}

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	return runContext(context.Background(), arguments, stdout, stderr)
}

func runContext(ctx context.Context, arguments []string, stdout, stderr io.Writer) error {
	options, err := parseFlags(arguments)
	if errors.Is(err, flag.ErrHelp) {
		_, writeErr := io.WriteString(stdout, commandUsage)
		return writeErr
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stderr, "[1/2] 校验 completion manifest 和完整对象闭包")
	result, err := templatecreator.ExportTemplateArtifacts(ctx, templatecreator.ExportTemplateArtifactsRequest{
		Artifacts: templatecreator.ArtifactLocation{
			StoreURL: "file://" + filepath.Clean(options.StoreRoot), ManifestKey: options.ManifestKey,
		},
		OutputRoot: options.OutputRoot,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stderr, "[2/2] Template 原始产物目录已发布")
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func parseFlags(arguments []string) (options, error) {
	flags := flag.NewFlagSet("export-template", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var result options
	flags.StringVar(&result.StoreRoot, "store-root", "", "TemplateCreator Local Store 绝对目录")
	flags.StringVar(&result.ManifestKey, "manifest", "", "构建结果返回的 completion ManifestKey")
	flags.StringVar(&result.OutputRoot, "output", "", "原始 Template 导出根目录；默认使用 <store-root>/exports")
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("positional arguments are not supported: %v", flags.Args())
	}
	for _, required := range []struct{ name, value string }{
		{name: "-store-root", value: result.StoreRoot},
		{name: "-manifest", value: result.ManifestKey},
	} {
		if required.value == "" {
			return options{}, fmt.Errorf("%s is required", required.name)
		}
	}
	if !filepath.IsAbs(result.StoreRoot) {
		return options{}, errors.New("-store-root must be an absolute path")
	}
	if result.OutputRoot == "" {
		result.OutputRoot = filepath.Join(result.StoreRoot, "exports")
	}
	if !filepath.IsAbs(result.OutputRoot) {
		return options{}, errors.New("-output must be an absolute path")
	}

	return result, nil
}
