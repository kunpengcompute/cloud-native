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
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	controllerwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

const defaultExcludedNamespaces = "kube-system,kube-public,kube-node-lease"

// Options contains command-line configuration for the KAE admission webhook.
type Options struct {
	Enabled            bool
	ListenAddr         string
	TLSCertFile        string
	TLSKeyFile         string
	DefaultResource    string
	DefaultCount       int64
	TargetContainer    int
	InjectEnvs         string
	ExcludedNamespaces string
}

// NewOptions returns disabled webhook options with production defaults.
func NewOptions() Options {
	return Options{
		ListenAddr:         ":9443",
		TLSCertFile:        "/tls/tls.crt",
		TLSKeyFile:         "/tls/tls.key",
		DefaultResource:    "hisi_hpre",
		DefaultCount:       1,
		ExcludedNamespaces: defaultExcludedNamespaces,
	}
}

// AddFlags registers KAE admission webhook command-line flags.
func (o *Options) AddFlags(flags *flag.FlagSet) {
	flags.BoolVar(&o.Enabled, "enable-admission-webhook", o.Enabled, "Enable the KAE admission webhook")
	flags.StringVar(&o.ListenAddr, "webhook-listen-addr", o.ListenAddr, "HTTPS listen address for the KAE admission webhook")
	flags.StringVar(&o.TLSCertFile, "webhook-tls-cert-file", o.TLSCertFile, "TLS certificate file for the KAE admission webhook")
	flags.StringVar(&o.TLSKeyFile, "webhook-tls-key-file", o.TLSKeyFile, "TLS private key file for the KAE admission webhook")
	flags.StringVar(&o.DefaultResource, "webhook-default-kae-resource", o.DefaultResource, "Default KAE device resource injected by the admission webhook")
	flags.Int64Var(&o.DefaultCount, "webhook-default-kae-count", o.DefaultCount, "Default KAE device count injected by the admission webhook")
	flags.IntVar(&o.TargetContainer, "webhook-target-container-index", o.TargetContainer, "Container index targeted by the admission webhook")
	flags.StringVar(&o.InjectEnvs, "webhook-inject-envs", o.InjectEnvs, "Comma-separated environment variables injected in KEY=VALUE format")
	flags.StringVar(&o.ExcludedNamespaces, "webhook-excluded-namespaces", o.ExcludedNamespaces, "Comma-separated namespaces excluded from KAE injection")
}

// Build validates enabled options and converts them to controller-runtime and injection configurations.
func (o Options) Build(podNamespace string) (controllerwebhook.Options, InjectionConfig, error) {
	if !o.Enabled {
		return controllerwebhook.Options{}, InjectionConfig{}, nil
	}

	podNamespace = strings.TrimSpace(podNamespace)
	if podNamespace == "" {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("POD_NAMESPACE must not be empty when the admission webhook is enabled")
	}
	if strings.TrimSpace(o.DefaultResource) == "" {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("default KAE resource must not be empty")
	}
	resourceName := NormalizeResourceName(o.DefaultResource)
	if errors := validation.IsQualifiedName(string(resourceName)); len(errors) != 0 {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("invalid KAE resource name %q: %s", resourceName, strings.Join(errors, ", "))
	}
	if o.DefaultCount <= 0 {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("default KAE count must be greater than zero")
	}
	if o.TargetContainer < 0 {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("target container index must not be negative")
	}

	host, portValue, err := net.SplitHostPort(strings.TrimSpace(o.ListenAddr))
	if err != nil {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("invalid webhook listen address %q: %w", o.ListenAddr, err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("invalid webhook listen port %q", portValue)
	}

	certFile := strings.TrimSpace(o.TLSCertFile)
	keyFile := strings.TrimSpace(o.TLSKeyFile)
	if certFile == "" {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("webhook TLS certificate file must not be empty")
	}
	if keyFile == "" {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("webhook TLS private key file must not be empty")
	}
	certDir := filepath.Clean(filepath.Dir(certFile))
	if keyDir := filepath.Clean(filepath.Dir(keyFile)); keyDir != certDir {
		return controllerwebhook.Options{}, InjectionConfig{}, fmt.Errorf("webhook TLS certificate and private key must be in the same directory")
	}

	envVars, err := ParseEnvVars(o.InjectEnvs)
	if err != nil {
		return controllerwebhook.Options{}, InjectionConfig{}, err
	}
	excludedNamespaces := appendUnique(splitCSV(o.ExcludedNamespaces), podNamespace)

	return controllerwebhook.Options{
			Host:     host,
			Port:     port,
			CertDir:  certDir,
			CertName: filepath.Base(certFile),
			KeyName:  filepath.Base(keyFile),
		}, InjectionConfig{
			Enabled:            true,
			DefaultResource:    strings.TrimSpace(o.DefaultResource),
			DefaultCount:       o.DefaultCount,
			TargetContainer:    o.TargetContainer,
			ExcludedNamespaces: excludedNamespaces,
			EnvVars:            envVars,
		}, nil
}

func splitCSV(value string) []string {
	values := strings.Split(value, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}

func appendUnique(values []string, additions ...string) []string {
	existing := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		existing[value] = struct{}{}
	}
	for _, addition := range additions {
		if _, found := existing[addition]; found {
			continue
		}
		values = append(values, addition)
		existing[addition] = struct{}{}
	}
	return values
}
