/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"flag"
	"reflect"
	"testing"
)

func TestOptionsAddFlags(t *testing.T) {
	options := NewOptions()
	flags := flag.NewFlagSet("webhook", flag.ContinueOnError)
	options.AddFlags(flags)

	err := flags.Parse([]string{
		"--enable-admission-webhook=true",
		"--webhook-listen-addr=127.0.0.1:10443",
		"--webhook-tls-cert-file=/certs/server.crt",
		"--webhook-tls-key-file=/certs/server.key",
		"--webhook-default-kae-resource=kae.kunpeng.com/hisi_zip",
		"--webhook-default-kae-count=2",
		"--webhook-target-container-index=1",
		"--webhook-inject-envs=KAE_MODE=auto,KAE_OPTIONS=a=b",
		"--webhook-included-namespaces=tenant-a,tenant-b",
		"--webhook-excluded-namespaces=kube-system,custom-system",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !options.Enabled || options.ListenAddr != "127.0.0.1:10443" {
		t.Fatalf("unexpected enable/listen options: %+v", options)
	}
	if options.TLSCertFile != "/certs/server.crt" || options.TLSKeyFile != "/certs/server.key" {
		t.Fatalf("unexpected TLS options: %+v", options)
	}
	if options.DefaultResource != "kae.kunpeng.com/hisi_zip" || options.DefaultCount != 2 {
		t.Fatalf("unexpected resource options: %+v", options)
	}
	if options.TargetContainer != 1 || options.InjectEnvs != "KAE_MODE=auto,KAE_OPTIONS=a=b" {
		t.Fatalf("unexpected injection options: %+v", options)
	}
	if options.ExcludedNamespaces != "kube-system,custom-system" {
		t.Fatalf("ExcludedNamespaces = %q", options.ExcludedNamespaces)
	}
	if options.IncludedNamespaces != "tenant-a,tenant-b" {
		t.Fatalf("IncludedNamespaces = %q", options.IncludedNamespaces)
	}
}

func TestOptionsBuild(t *testing.T) {
	options := NewOptions()
	options.Enabled = true
	options.InjectEnvs = "KAE_MODE=auto,KAE_OPTIONS=a=b"
	options.IncludedNamespaces = "tenant-a, tenant-b,tenant-a"

	serverOptions, injectionConfig, err := options.Build("kae-system")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if serverOptions.Host != "" || serverOptions.Port != 9443 {
		t.Fatalf("server address = %s:%d, want :9443", serverOptions.Host, serverOptions.Port)
	}
	if serverOptions.CertDir != "/tls" || serverOptions.CertName != "tls.crt" || serverOptions.KeyName != "tls.key" {
		t.Fatalf("unexpected TLS server options: %+v", serverOptions)
	}
	if !injectionConfig.Enabled || injectionConfig.DefaultResource != "hisi_hpre" || injectionConfig.DefaultCount != 1 {
		t.Fatalf("unexpected injection config: %+v", injectionConfig)
	}
	if injectionConfig.TargetContainer != 0 {
		t.Fatalf("TargetContainer = %d, want 0", injectionConfig.TargetContainer)
	}
	wantNamespaces := []string{"kube-system", "kube-public", "kube-node-lease", "kae-system"}
	if !reflect.DeepEqual(injectionConfig.ExcludedNamespaces, wantNamespaces) {
		t.Fatalf("ExcludedNamespaces = %#v, want %#v", injectionConfig.ExcludedNamespaces, wantNamespaces)
	}
	wantIncludedNamespaces := []string{"tenant-a", "tenant-b"}
	if !reflect.DeepEqual(injectionConfig.IncludedNamespaces, wantIncludedNamespaces) {
		t.Fatalf("IncludedNamespaces = %#v, want %#v", injectionConfig.IncludedNamespaces, wantIncludedNamespaces)
	}
	if len(injectionConfig.EnvVars) != 2 || injectionConfig.EnvVars[1].Value != "a=b" {
		t.Fatalf("unexpected environment variables: %#v", injectionConfig.EnvVars)
	}
}

func TestOptionsBuildDisabled(t *testing.T) {
	options := NewOptions()
	options.ListenAddr = "invalid"
	options.DefaultCount = 0

	serverOptions, injectionConfig, err := options.Build("")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if serverOptions.Port != 0 || injectionConfig.Enabled {
		t.Fatalf("disabled Build() returned active options: server=%+v injection=%+v", serverOptions, injectionConfig)
	}
}

func TestOptionsBuildValidation(t *testing.T) {
	tests := []struct {
		name         string
		podNamespace string
		mutate       func(*Options)
	}{
		{name: "missing pod namespace", mutate: func(*Options) {}},
		{name: "empty resource", podNamespace: "kae-system", mutate: func(o *Options) { o.DefaultResource = "" }},
		{name: "invalid resource", podNamespace: "kae-system", mutate: func(o *Options) { o.DefaultResource = "bad/name/extra" }},
		{name: "non-positive count", podNamespace: "kae-system", mutate: func(o *Options) { o.DefaultCount = 0 }},
		{name: "negative target", podNamespace: "kae-system", mutate: func(o *Options) { o.TargetContainer = -1 }},
		{name: "invalid environment variable", podNamespace: "kae-system", mutate: func(o *Options) { o.InjectEnvs = "KAE_MODE" }},
		{name: "invalid listen address", podNamespace: "kae-system", mutate: func(o *Options) { o.ListenAddr = "9443" }},
		{name: "invalid listen port", podNamespace: "kae-system", mutate: func(o *Options) { o.ListenAddr = ":0" }},
		{name: "missing certificate", podNamespace: "kae-system", mutate: func(o *Options) { o.TLSCertFile = "" }},
		{name: "missing private key", podNamespace: "kae-system", mutate: func(o *Options) { o.TLSKeyFile = "" }},
		{name: "different TLS directories", podNamespace: "kae-system", mutate: func(o *Options) {
			o.TLSCertFile = "/certs/tls.crt"
			o.TLSKeyFile = "/keys/tls.key"
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewOptions()
			options.Enabled = true
			tt.mutate(&options)
			if _, _, err := options.Build(tt.podNamespace); err == nil {
				t.Fatal("Build() error = nil, want validation error")
			}
		})
	}
}

func TestOptionsBuildDoesNotDuplicateOwnNamespace(t *testing.T) {
	options := NewOptions()
	options.Enabled = true
	options.ExcludedNamespaces = "kube-system,kae-system"

	_, injectionConfig, err := options.Build("kae-system")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	want := []string{"kube-system", "kae-system"}
	if !reflect.DeepEqual(injectionConfig.ExcludedNamespaces, want) {
		t.Fatalf("ExcludedNamespaces = %#v, want %#v", injectionConfig.ExcludedNamespaces, want)
	}
}
