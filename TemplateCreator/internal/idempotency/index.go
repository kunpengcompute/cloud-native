// Package idempotency 持久化已完成的 Local Store 构建结果，并在单个进程内串行化相同任务编号的请求。
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ErrConflict 表示某个 key 已经对应另一份摘要的完成请求。
var ErrConflict = errors.New("idempotency key conflicts with a different request")

// Index 将完成结果写入磁盘，并在单个进程内串行化同 key 调用方。
// Local Demo contract 不提供跨进程互斥。
type Index struct {
	// directory 是 Store 下专门保存任务完成记录的绝对目录。
	directory string
	// mu 只保护 locks map；每个 key 的实际等待由独立 channel 完成。
	mu    sync.Mutex
	locks map[string]chan struct{}
}

// record 是磁盘任务完成记录的严格 JSON schema。
type record struct {
	SchemaVersion int             `json:"schema_version"`
	Digest        string          `json:"digest"`
	Result        json.RawMessage `json:"result"`
}

// New 在 Local Artifact Store 根目录下创建与 manifest 并列的任务完成记录区。
func New(storeRoot string) (*Index, error) {
	if !filepath.IsAbs(storeRoot) {
		return nil, fmt.Errorf("idempotency store root must be absolute")
	}
	directory := filepath.Join(storeRoot, "_templatecreator", "idempotency")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	return &Index{directory: directory, locks: make(map[string]chan struct{})}, nil
}

// WithKey 返回摘要匹配的完成结果，或在当前进程中串行执行同 key build callback。
// callback 失败不会写记录，因此后续调用仍可用相同 key 重试；bool 表示是否命中已有结果。
func (index *Index) WithKey(ctx context.Context, key, digest string, build func() (json.RawMessage, error)) (json.RawMessage, bool, error) {
	lock := index.keyLock(key)
	// 容量为 1 的 channel 充当可取消的 per-key mutex；sync.Mutex 无法在等待期间响应 context。
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}

	path := index.recordPath(key)
	existing, err := os.ReadFile(path)
	if err == nil {
		record, err := decodeRecord(existing)
		if err != nil {
			return nil, false, fmt.Errorf("corrupt idempotency record: %w", err)
		}
		if record.Digest != digest {
			return nil, false, ErrConflict
		}
		return compactJSON(record.Result), true, nil
	}
	if !os.IsNotExist(err) {
		return nil, false, err
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	// 只有 callback 完成发布并返回有效 JSON 后才能创建记录；失败构建保持可重试。
	result, err := build()
	if err != nil {
		return nil, false, err
	}
	if len(result) == 0 || !json.Valid(result) {
		return nil, false, fmt.Errorf("idempotent build result must be valid JSON")
	}
	result = compactJSON(result)
	data, err := json.MarshalIndent(record{SchemaVersion: 1, Digest: digest, Result: result}, "", "  ")
	if err != nil {
		return nil, false, err
	}
	data = append(data, '\n')
	if err := writeAtomic(path, data); err != nil {
		return nil, false, err
	}
	return append(json.RawMessage(nil), result...), false, nil
}

// compactJSON 规范化结果并复制底层字节，避免保存调用方可继续修改的 slice。
func compactJSON(data json.RawMessage) json.RawMessage {
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		return append(json.RawMessage(nil), data...)
	}
	return append(json.RawMessage(nil), compact.Bytes()...)
}

// keyLock 返回进程内稳定的 per-key 可取消锁。
func (index *Index) keyLock(key string) chan struct{} {
	index.mu.Lock()
	defer index.mu.Unlock()
	lock := index.locks[key]
	if lock == nil {
		lock = make(chan struct{}, 1)
		index.locks[key] = lock
	}
	return lock
}

// recordPath 对任意用户 key 做 SHA-256 后再构造文件名，防止路径注入。
func (index *Index) recordPath(key string) string {
	// 原始 key 不参与路径拼接，也不会作为明文文件名泄露。
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(index.directory, hex.EncodeToString(hash[:])+".json")
}

// decodeRecord 使用严格 schema 读取单个 JSON value，拒绝未知字段和尾随内容。
func decodeRecord(data []byte) (record, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var result record
	if err := decoder.Decode(&result); err != nil {
		return record{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return record{}, fmt.Errorf("trailing JSON data")
	}
	if result.SchemaVersion != 1 || result.Digest == "" || len(result.Result) == 0 || !json.Valid(result.Result) {
		return record{}, fmt.Errorf("invalid record fields")
	}
	return result, nil
}

// writeAtomic 在同一目录写临时文件并 rename，使读者只能看到旧完整记录或新完整记录。
func writeAtomic(path string, data []byte) (returnErr error) {
	// write、fsync、chmod、rename 均围绕同一 filesystem directory 完成，
	// 最后再 fsync directory，避免 crash 后留下看似完成的半截 JSON。
	temporary, err := os.CreateTemp(filepath.Dir(path), ".idempotency-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err := os.Remove(temporaryPath); returnErr == nil && err != nil && !os.IsNotExist(err) {
			returnErr = err
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
