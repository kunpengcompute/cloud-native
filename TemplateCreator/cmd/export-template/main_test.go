package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShowsBeginnerHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	help := stdout.String()
	for _, text := range []string{"export-template", "-store-root", "-manifest", "-output", "完整对象闭包", "不生成压缩包"} {
		if !strings.Contains(help, text) {
			t.Errorf("help missing %q:\n%s", text, help)
		}
	}
}

func TestParseFlagsRequiresAllPaths(t *testing.T) {
	if _, err := parseFlags([]string{"-store-root", "/store"}); err == nil || !strings.Contains(err.Error(), "-manifest") {
		t.Fatalf("parseFlags() error = %v, want -manifest required", err)
	}
}

func TestParseFlagsDefaultsOutputRootUnderStore(t *testing.T) {
	options, err := parseFlags([]string{"-store-root", "/store", "-manifest", "_templatecreator/manifests/tmpl/build.json"})
	if err != nil {
		t.Fatal(err)
	}
	if options.OutputRoot != "/store/exports" {
		t.Fatalf("OutputRoot = %q, want /store/exports", options.OutputRoot)
	}
}
