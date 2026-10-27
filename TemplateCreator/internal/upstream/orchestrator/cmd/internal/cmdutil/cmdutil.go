// Copyright (c) 2023-2026 e2b Devs Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cmdutil provides shared utilities for CLI commands.
// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package cmdutil

import (
	"io"
	"log"
	"os"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

// SuppressNoisyLogs disables verbose output from OTEL tracing, LaunchDarkly, and standard log.
// Only ERROR level and above will be logged.
func SuppressNoisyLogs() {
	// Silence standard log package
	log.SetOutput(io.Discard)
	// Replace global zap logger with error-only logger
	setErrorOnlyLogger()
}

// SuppressNoisyLogsKeepStdLog disables verbose output but keeps standard log enabled.
func SuppressNoisyLogsKeepStdLog() {
	setErrorOnlyLogger()
}

// setErrorOnlyLogger replaces the global zap logger with one that only logs errors.
func setErrorOnlyLogger() {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	errLogger, err := cfg.Build()
	if err == nil {
		zap.ReplaceGlobals(errLogger)
	}
}

// GetHeaderInfo reads a header file and returns total size and block size.
func GetHeaderInfo(headerPath string) (totalSize, blockSize uint64) {
	data, err := os.ReadFile(headerPath)
	if err != nil {
		return 0, 0
	}
	h, err := header.DeserializeBytes(data)
	if err != nil {
		return 0, 0
	}

	return h.Metadata.Size, h.Metadata.BlockSize
}

// GetFileSizes returns the logical size and actual on-disk size of a file.
func GetFileSizes(path string) (logical, actual int64, err error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0, 0, err
	}

	return stat.Size, stat.Blocks * 512, nil
}

// GetActualFileSize returns only the actual on-disk size of a file.
func GetActualFileSize(path string) (int64, error) {
	_, actual, err := GetFileSizes(path)

	return actual, err
}

// ArtifactInfo contains information about a build artifact.
type ArtifactInfo struct {
	Name       string
	File       string
	HeaderFile string
}

// MainArtifacts returns the list of main artifacts (rootfs, memfile).
func MainArtifacts() []ArtifactInfo {
	return []ArtifactInfo{
		{"Rootfs", storage.RootfsName, storage.RootfsName + storage.HeaderSuffix},
		{"Memfile", storage.MemfileName, storage.MemfileName + storage.HeaderSuffix},
	}
}

// SmallArtifacts returns the list of small artifacts (headers, snapfile, metadata).
func SmallArtifacts() []struct{ Name, File string } {
	return []struct{ Name, File string }{
		{"Rootfs header", storage.RootfsName + storage.HeaderSuffix},
		{"Memfile header", storage.MemfileName + storage.HeaderSuffix},
		{"Snapfile", storage.SnapfileName},
		{"Metadata", storage.MetadataName},
	}
}
