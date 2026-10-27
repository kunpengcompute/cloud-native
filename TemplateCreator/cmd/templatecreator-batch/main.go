package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/preflight"
)

const usage = `TemplateCreator 批量创建 Template

用法：templatecreator-batch -requests <JSON 文件> -runtime-info <runtime-info.json> -scratch-root <目录> -store-root <目录> [选项]

选项：
  -requests        UTF-8 JSON 请求文件绝对路径；schema_version 固定为 1
  -runtime-info    prepare-runtime 输出的 runtime-info.json 绝对路径
  -scratch-root    已存在的可写临时目录绝对路径
  -store-root      已存在的可写 Local Store 绝对路径
  -export-root     可选导出根目录绝对路径；省略时不导出
  -max-parallel    同一 Creator 的最大物理构建数（默认 1）
  -timeout         每项从 worker 开始处理起的超时（默认 30m）
  -proxy-port      sandbox proxy 本地端口（默认 5007）
  -log-level       off、info 或 debug（默认 info）
  -format          fromImage 或 fromDockerfile（默认 fromImage）
  -publication     simple 或 all（默认 all）
  -vcpu            默认 guest vCPU（默认 2）
  -memory-mb       默认 guest 内存 MiB（默认 1024）
  -disk-size-mb    默认 guest 根盘 MiB（默认 4096）
  -hugepages       默认启用 host HugePages

请求文件示例：
  {"schema_version":1,"requests":[{"request_id":"base","image_ref":"docker.io/library/debian:bookworm-slim"}]}

build_override 中数值 0 表示继承默认；提供 build_override 后 huge_pages 的 false 会覆盖默认。
steps 的空数组会清空默认 steps。所有请求静态校验及 host preflight 完成后才初始化 Creator。
stdout 最终只输出一个 JSON 对象；构建日志及 request_id 关联事件写 stderr。
`

type options struct {
	Requests, RuntimeInfo, ScratchRoot, StoreRoot, ExportRoot string
	MaxParallel                                               int
	Timeout                                                   time.Duration
	ProxyPort                                                 uint16
	LogLevel                                                  string
	Format                                                    templatecreator.BuildFormat
	Publication                                               templatecreator.PublicationMode
	Defaults                                                  templatecreator.BuildSpec
}

type inputFile struct {
	SchemaVersion int         `json:"schema_version"`
	Requests      []inputItem `json:"requests"`
}
type inputItem struct {
	RequestID      string                          `json:"request_id"`
	ImageRef       string                          `json:"image_ref"`
	ImageTarPath   string                          `json:"image_tar_path"`
	IdempotencyKey string                          `json:"idempotency_key"`
	Format         templatecreator.BuildFormat     `json:"format"`
	Publication    templatecreator.PublicationMode `json:"publication"`
	BuildOverride  *inputBuild                     `json:"build_override"`
	Labels         map[string]string               `json:"labels"`
}
type inputBuild struct {
	VCPU         int64        `json:"vcpu"`
	MemoryMB     int64        `json:"memory_mb"`
	DiskSizeMB   int64        `json:"disk_size_mb"`
	HugePages    bool         `json:"huge_pages"`
	StartCommand string       `json:"start_command"`
	ReadyCommand string       `json:"ready_command"`
	Steps        *[]inputStep `json:"steps"`
}
type inputStep struct {
	Type string   `json:"type"`
	Args []string `json:"args"`
}
type batchItem struct {
	RequestID string
	Request   templatecreator.CreateRequest
}
type itemOutput struct {
	RequestID  string                                         `json:"request_id"`
	Result     *templatecreator.CreateResult                  `json:"result,omitempty"`
	Export     *templatecreator.ExportTemplateArtifactsResult `json:"export,omitempty"`
	Stage      string                                         `json:"stage,omitempty"`
	Error      string                                         `json:"error,omitempty"`
	TemplateID string                                         `json:"template_id,omitempty"`
	BuildID    string                                         `json:"build_id,omitempty"`
}
type batchOutput struct {
	SchemaVersion int          `json:"schema_version"`
	Results       []itemOutput `json:"results"`
	Succeeded     int          `json:"succeeded"`
	Failed        int          `json:"failed"`
	Canceled      int          `json:"canceled"`
	CloseError    string       `json:"close_error,omitempty"`
}

type batchCreator interface {
	CreateTemplate(context.Context, templatecreator.CreateRequest) (templatecreator.CreateResult, error)
	Close(context.Context) error
}
type dependencies struct {
	loadRuntime     func(string) (templatecreator.RuntimeConfig, runtimePaths, error)
	preflight       func(preflight.Config) preflight.Report
	capacity        func([]batchItem, int) error
	newCreator      func(context.Context, templatecreator.CreatorConfig) (batchCreator, error)
	exportArtifacts func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error)
}

// lockedWriter 防止批量事件和 Creator 日志并发写入时交错。
type lockedWriter struct {
	mu     sync.Mutex
	target io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.target.Write(p)
}

// logCounters 记录批量调度器状态；这些数字是请求/worker 数，不代表真实物理构建槽位。
func logCounters(w io.Writer, requestID, stage, templateID, buildID string, waiting, activeWorkers, finished int64) {
	if w == nil {
		return
	}
	encoded, _ := json.Marshal(struct {
		RequestID        string `json:"request_id"`
		Stage            string `json:"stage"`
		TemplateID       string `json:"template_id"`
		BuildID          string `json:"build_id"`
		WaitingRequests  int64  `json:"waiting_requests"`
		ActiveWorkers    int64  `json:"active_workers"`
		FinishedRequests int64  `json:"finished_requests"`
	}{requestID, stage, templateID, buildID, waiting, activeWorkers, finished})
	_, _ = fmt.Fprintln(w, string(encoded))
}

// redirectProcessStdout 保留唯一 JSON 输出流；upstream telemetry 的 fmt.Printf
// 直接写全局 os.Stdout，必须在启动任何 Builder goroutine 前转到 stderr。
func redirectProcessStdout() *os.File {
	jsonOutput := os.Stdout
	os.Stdout = os.Stderr
	return jsonOutput
}

func main() {
	jsonOutput := redirectProcessStdout()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	deps := dependencies{loadRuntime: loadRuntimeInfo, preflight: preflight.Run,
		newCreator: func(ctx context.Context, cfg templatecreator.CreatorConfig) (batchCreator, error) {
			return templatecreator.NewCreator(ctx, cfg)
		},
		exportArtifacts: templatecreator.ExportTemplateArtifacts}
	if err := run(ctx, os.Args[1:], jsonOutput, os.Stderr, deps); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (options, bool, error) {
	f := flag.NewFlagSet("templatecreator-batch", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	o := options{RuntimeInfo: templatecreator.DefaultRuntimeInfoPath, ScratchRoot: templatecreator.DefaultScratchRoot, StoreRoot: templatecreator.DefaultStoreRoot,
		MaxParallel: templatecreator.DefaultMaxParallel, Timeout: templatecreator.DefaultBuildTimeout, ProxyPort: templatecreator.DefaultProxyPort,
		LogLevel: string(templatecreator.DefaultLogLevel), Format: templatecreator.DefaultBuildFormat, Publication: templatecreator.DefaultPublicationMode, Defaults: templatecreator.DefaultBuildSpec()}
	var port uint
	var format, publication string
	f.StringVar(&o.Requests, "requests", "", "批量请求 JSON 文件")
	f.StringVar(&o.RuntimeInfo, "runtime-info", o.RuntimeInfo, "runtime-info.json")
	f.StringVar(&o.ScratchRoot, "scratch-root", o.ScratchRoot, "临时目录")
	f.StringVar(&o.StoreRoot, "store-root", o.StoreRoot, "Local Store")
	f.StringVar(&o.ExportRoot, "export-root", "", "可选导出根目录")
	f.IntVar(&o.MaxParallel, "max-parallel", o.MaxParallel, "最大并发数")
	f.DurationVar(&o.Timeout, "timeout", o.Timeout, "每项超时")
	f.UintVar(&port, "proxy-port", uint(o.ProxyPort), "proxy 端口")
	f.StringVar(&o.LogLevel, "log-level", o.LogLevel, "日志级别")
	f.StringVar(&format, "format", string(o.Format), "默认构建格式")
	f.StringVar(&publication, "publication", string(o.Publication), "默认发布模式")
	f.Int64Var(&o.Defaults.VCPU, "vcpu", o.Defaults.VCPU, "默认 vCPU")
	f.Int64Var(&o.Defaults.MemoryMB, "memory-mb", o.Defaults.MemoryMB, "默认内存 MiB")
	f.Int64Var(&o.Defaults.DiskSizeMB, "disk-size-mb", o.Defaults.DiskSizeMB, "默认根盘 MiB")
	f.BoolVar(&o.Defaults.HugePages, "hugepages", false, "默认使用 HugePages")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, true, nil
		}
		return o, false, err
	}
	if f.NArg() != 0 {
		return o, false, fmt.Errorf("positional arguments are not supported")
	}
	o.ProxyPort = uint16(port)
	o.Format = templatecreator.BuildFormat(format)
	o.Publication = templatecreator.PublicationMode(publication)
	for name, path := range map[string]string{"-requests": o.Requests, "-runtime-info": o.RuntimeInfo, "-scratch-root": o.ScratchRoot, "-store-root": o.StoreRoot} {
		if path == "" || !filepath.IsAbs(path) {
			return o, false, fmt.Errorf("%s requires an absolute path", name)
		}
	}
	if o.ExportRoot != "" && !filepath.IsAbs(o.ExportRoot) {
		return o, false, fmt.Errorf("-export-root requires an absolute path")
	}
	if o.MaxParallel <= 0 || o.Timeout <= 0 || port == 0 || port > 65535 {
		return o, false, fmt.Errorf("max-parallel, timeout and proxy-port must be positive and valid")
	}
	if o.Defaults.VCPU <= 0 || o.Defaults.MemoryMB <= 0 || o.Defaults.DiskSizeMB <= 0 {
		return o, false, fmt.Errorf("default build resources must be positive")
	}
	if o.Format != templatecreator.BuildFormatFromImage && o.Format != templatecreator.BuildFormatFromDockerfile {
		return o, false, fmt.Errorf("unsupported format %q", o.Format)
	}
	if o.Publication != templatecreator.PublicationSimple && o.Publication != templatecreator.PublicationAll {
		return o, false, fmt.Errorf("unsupported publication %q", o.Publication)
	}
	if o.LogLevel != "off" && o.LogLevel != "info" && o.LogLevel != "debug" {
		return o, false, fmt.Errorf("unsupported log level %q", o.LogLevel)
	}
	return o, false, nil
}

// decodeStrict 拒绝未知字段、尾随 JSON 和无效 UTF-8；输入不得夹带额外请求。
func decodeStrict(data []byte, value any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON document is not UTF-8")
	}
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON document")
	}
	if err := checkUniqueKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}

// checkUniqueKeys 逐层遍历对象，防止重复字段被 encoding/json 静默覆盖。
func checkUniqueKeys(decoder *json.Decoder) error {
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				// encoding/json 对结构体字段大小写不敏感；按相同规则拒绝别名覆盖。
				folded := strings.ToLower(key)
				if seen[folded] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[folded] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	return walk()
}

func loadRequests(path string) ([]batchItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read requests: %w", err)
	}
	var input inputFile
	if err := decodeStrict(data, &input); err != nil {
		return nil, fmt.Errorf("decode requests: %w", err)
	}
	if input.SchemaVersion != 1 || len(input.Requests) == 0 {
		return nil, fmt.Errorf("schema_version must be 1 and requests must be nonempty")
	}
	seen := make(map[string]bool, len(input.Requests))
	result := make([]batchItem, len(input.Requests))
	for i, item := range input.Requests {
		if strings.TrimSpace(item.RequestID) == "" || seen[item.RequestID] {
			return nil, fmt.Errorf("requests[%d]: request_id is empty or duplicate", i)
		}
		seen[item.RequestID] = true
		if (strings.TrimSpace(item.ImageRef) == "") == (strings.TrimSpace(item.ImageTarPath) == "") {
			return nil, fmt.Errorf("requests[%d]: choose exactly one image input", i)
		}
		if item.ImageTarPath != "" && !filepath.IsAbs(item.ImageTarPath) {
			return nil, fmt.Errorf("requests[%d]: image_tar_path must be absolute", i)
		}
		if item.Format != "" && item.Format != templatecreator.BuildFormatFromImage && item.Format != templatecreator.BuildFormatFromDockerfile {
			return nil, fmt.Errorf("requests[%d]: invalid format", i)
		}
		if item.Publication != "" && item.Publication != templatecreator.PublicationSimple && item.Publication != templatecreator.PublicationAll {
			return nil, fmt.Errorf("requests[%d]: invalid publication", i)
		}
		request := templatecreator.CreateRequest{ImageRef: item.ImageRef, ImageTarPath: item.ImageTarPath, IdempotencyKey: item.IdempotencyKey, Format: item.Format, Publication: item.Publication, Labels: item.Labels}
		if item.BuildOverride != nil {
			b := item.BuildOverride
			if b.VCPU < 0 || b.MemoryMB < 0 || b.DiskSizeMB < 0 {
				return nil, fmt.Errorf("requests[%d]: build_override resources must not be negative", i)
			}
			spec := templatecreator.BuildSpec{VCPU: b.VCPU, MemoryMB: b.MemoryMB, DiskSizeMB: b.DiskSizeMB, HugePages: b.HugePages, StartCommand: b.StartCommand, ReadyCommand: b.ReadyCommand}
			if b.Steps != nil {
				spec.Steps = make([]templatecreator.TemplateStep, len(*b.Steps))
				for j, step := range *b.Steps {
					spec.Steps[j] = templatecreator.TemplateStep{Type: step.Type, Args: step.Args}
				}
			}
			request.BuildOverride = &spec
		}
		result[i] = batchItem{RequestID: item.RequestID, Request: request}
	}
	return result, nil
}

func resolvedBuild(defaults templatecreator.BuildSpec, override *templatecreator.BuildSpec) templatecreator.BuildSpec {
	result := defaults
	if override == nil {
		return result
	}
	if override.VCPU != 0 {
		result.VCPU = override.VCPU
	}
	if override.MemoryMB != 0 {
		result.MemoryMB = override.MemoryMB
	}
	if override.DiskSizeMB != 0 {
		result.DiskSizeMB = override.DiskSizeMB
	}
	result.HugePages = override.HugePages
	if override.StartCommand != "" {
		result.StartCommand = override.StartCommand
	}
	if override.ReadyCommand != "" {
		result.ReadyCommand = override.ReadyCommand
	}
	if override.Steps != nil {
		result.Steps = override.Steps
	}
	return result
}

func executeBatch(ctx context.Context, creator batchCreator, items []batchItem, workers int, timeout time.Duration, exportRoot string, exporter func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error), logs io.Writer) batchOutput {
	output := batchOutput{SchemaVersion: 1, Results: make([]itemOutput, len(items))}
	for i, item := range items {
		output.Results[i].RequestID = item.RequestID
	}
	if workers > len(items) {
		workers = len(items)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	var waiting, occupied, finished atomic.Int64
	waiting.Store(int64(len(items)))
	var eventLogs io.Writer
	if logs != nil {
		eventLogs = &lockedWriter{target: logs}
	}
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				item := items[index]
				waiting.Add(-1)
				if ctx.Err() != nil {
					output.Results[index] = itemOutput{RequestID: item.RequestID, Stage: "canceled", Error: ctx.Err().Error()}
					finished.Add(1)
					logCounters(eventLogs, item.RequestID, "canceled", "", "", waiting.Load(), occupied.Load(), finished.Load())
					continue
				}
				itemCtx, cancel := context.WithTimeout(ctx, timeout)
				occupied.Add(1)
				logCounters(eventLogs, item.RequestID, "worker_started", "", "", waiting.Load(), occupied.Load(), finished.Load())
				result, err := creator.CreateTemplate(itemCtx, item.Request)
				entry := itemOutput{RequestID: item.RequestID}
				if err != nil {
					entry.Stage = "create"
					entry.Error = err.Error()
					var buildErr *templatecreator.BuildError
					if errors.As(err, &buildErr) {
						entry.Stage = string(buildErr.Phase)
						entry.TemplateID = buildErr.TemplateID
						entry.BuildID = buildErr.BuildID
					}
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						entry.Stage = "canceled"
					}
				} else {
					entry.Result = &result
					entry.TemplateID = result.TemplateID
					entry.BuildID = result.BuildID
					if exportRoot != "" && exporter != nil {
						exported, exportErr := exporter(itemCtx, templatecreator.ExportTemplateArtifactsRequest{Artifacts: result.Artifacts, OutputRoot: exportRoot})
						if exportErr != nil {
							entry.Stage = "export"
							entry.Error = exportErr.Error()
						} else {
							entry.Export = &exported
						}
					}
				}
				cancel()
				occupied.Add(-1)
				finished.Add(1)
				if entry.Stage == "" {
					logCounters(eventLogs, item.RequestID, "completed", entry.TemplateID, entry.BuildID, waiting.Load(), occupied.Load(), finished.Load())
				} else {
					logCounters(eventLogs, item.RequestID, entry.Stage, entry.TemplateID, entry.BuildID, waiting.Load(), occupied.Load(), finished.Load())
				}
				output.Results[index] = entry
			}
		}()
	}
dispatch:
	for i := range items {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	for i := range output.Results {
		entry := &output.Results[i]
		if entry.Stage == "" && entry.Result == nil {
			entry.Stage = "canceled"
			if ctx.Err() != nil {
				entry.Error = ctx.Err().Error()
			} else {
				entry.Error = "request was not started"
			}
		}
		switch entry.Stage {
		case "":
			output.Succeeded++
		case "canceled":
			output.Canceled++
		default:
			output.Failed++
		}
	}
	logCounters(eventLogs, "", "batch_finished", "", "", 0, 0, int64(len(items)))
	return output
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, deps dependencies) error {
	o, help, err := parseFlags(args)
	if err != nil {
		return err
	}
	if help {
		_, err = io.WriteString(stdout, usage)
		return err
	}
	items, err := loadRequests(o.Requests)
	if err != nil {
		return err
	}
	loader := deps.loadRuntime
	if loader == nil {
		loader = loadRuntimeInfo
	}
	runtimeCfg, paths, err := loader(o.RuntimeInfo)
	if err != nil {
		return err
	}
	runtimeCfg.ProxyPort = o.ProxyPort
	check := deps.preflight
	if check == nil {
		check = preflight.Run
	}
	for _, item := range items {
		build := resolvedBuild(o.Defaults, item.Request.BuildOverride)
		report := check(preflight.Config{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Architecture: runtimeCfg.Architecture,
			ImageRef: item.Request.ImageRef, ImageTarPath: item.Request.ImageTarPath, ScratchRoot: o.ScratchRoot, StoreRoot: o.StoreRoot,
			KernelPath: paths.Kernel, FirecrackerPath: paths.Firecracker, EnvdPath: paths.Envd, BusyboxPath: paths.Busybox,
			KernelSHA256: paths.KernelSHA256, FirecrackerSHA256: paths.FirecrackerSHA256, EnvdSHA256: paths.EnvdSHA256, BusyboxSHA256: paths.BusyboxSHA256,
			MemoryMB: build.MemoryMB, HugePages: build.HugePages})
		if !report.Ready() {
			for _, c := range report.Checks {
				if !c.OK {
					return fmt.Errorf("preflight request_id=%q check=%s: %s", item.RequestID, c.Name, c.Detail)
				}
			}
			return fmt.Errorf("preflight failed")
		}
	}
	capacity := deps.capacity
	if capacity == nil {
		capacity = func(items []batchItem, parallel int) error {
			return checkCapacityAt(items, parallel, o.ScratchRoot, o.StoreRoot)
		}
	}
	if err := capacityWithDefaults(capacity, items, o.Defaults, o.MaxParallel); err != nil {
		return fmt.Errorf("capacity preflight: %w", err)
	}
	newCreator := deps.newCreator
	if newCreator == nil {
		return fmt.Errorf("Creator factory is unavailable")
	}
	logWriter := &lockedWriter{target: stderr}
	var creatorLogs io.Writer = logWriter
	if o.LogLevel == "off" {
		creatorLogs = nil
	}
	storeURL := (&url.URL{Scheme: "file", Path: filepath.Clean(o.StoreRoot)}).String()
	creator, err := newCreator(ctx, templatecreator.CreatorConfig{ScratchRoot: o.ScratchRoot, Store: templatecreator.StoreConfig{URL: storeURL}, Runtime: runtimeCfg,
		Defaults: o.Defaults, Format: o.Format, Publication: o.Publication, MaxParallel: o.MaxParallel, LogWriter: creatorLogs, LogLevel: templatecreator.LogLevel(o.LogLevel)})
	if err != nil {
		return fmt.Errorf("initialize Creator: %w", err)
	}
	output := executeBatch(ctx, creator, items, o.MaxParallel, o.Timeout, o.ExportRoot, deps.exportArtifacts, logWriter)
	// Close 使用独立 context：批次取消不能阻止进程资源清理。
	if closeErr := creator.Close(context.Background()); closeErr != nil {
		output.CloseError = closeErr.Error()
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return err
	}
	if output.Failed > 0 || output.Canceled > 0 || output.CloseError != "" {
		return fmt.Errorf("batch finished with %d failed, %d canceled; close error: %s", output.Failed, output.Canceled, output.CloseError)
	}
	return nil
}
