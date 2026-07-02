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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

const kaeResourcePrefix = "kae.kunpeng.com/"

// InjectionConfig controls automatic KAE resource and environment variable injection.
type InjectionConfig struct {
	Enabled            bool
	DefaultResource    string
	DefaultCount       int64
	TargetContainer    int
	IncludedNamespaces []string
	ExcludedNamespaces []string
	EnvVars            []corev1.EnvVar
}

// NormalizeResourceName converts a short KAE device type into an extended resource name.
func NormalizeResourceName(name string) corev1.ResourceName {
	name = strings.TrimSpace(name)
	if strings.Contains(name, "/") {
		return corev1.ResourceName(name)
	}
	return corev1.ResourceName(kaeResourcePrefix + name)
}

// MutatePodForKAEInjection injects configured KAE resources and environment variables into a Pod.
func MutatePodForKAEInjection(pod *corev1.Pod, config InjectionConfig) (bool, error) {
	if pod == nil {
		return false, fmt.Errorf("pod is nil")
	}
	if !config.Enabled || !shouldInjectNamespace(pod.Namespace, config.IncludedNamespaces, config.ExcludedNamespaces) {
		return false, nil
	}
	if len(pod.Spec.Containers) == 0 {
		return false, fmt.Errorf("pod %s/%s has no containers", pod.Namespace, pod.Name)
	}
	if config.TargetContainer < 0 || config.TargetContainer >= len(pod.Spec.Containers) {
		return false, fmt.Errorf("target container index %d out of range", config.TargetContainer)
	}
	if config.DefaultCount <= 0 {
		return false, fmt.Errorf("default KAE count must be greater than zero")
	}

	resourceName := NormalizeResourceName(config.DefaultResource)
	if strings.TrimSpace(config.DefaultResource) == "" {
		return false, fmt.Errorf("default KAE resource must not be empty")
	}
	if errors := validation.IsQualifiedName(string(resourceName)); len(errors) != 0 {
		return false, fmt.Errorf("invalid KAE resource name %q: %s", resourceName, strings.Join(errors, ", "))
	}

	changed := false
	if !hasKAEResource(pod) {
		container := &pod.Spec.Containers[config.TargetContainer]
		if container.Resources.Requests == nil {
			container.Resources.Requests = corev1.ResourceList{}
		}
		if container.Resources.Limits == nil {
			container.Resources.Limits = corev1.ResourceList{}
		}
		quantity := *resource.NewQuantity(config.DefaultCount, resource.DecimalSI)
		container.Resources.Requests[resourceName] = quantity
		container.Resources.Limits[resourceName] = quantity
		changed = true
	}

	if appendMissingEnvVars(&pod.Spec.Containers[config.TargetContainer], config.EnvVars) {
		changed = true
	}
	return changed, nil
}

// ParseEnvVars parses comma-separated KEY=VALUE pairs into Kubernetes environment variables.
func ParseEnvVars(value string) ([]corev1.EnvVar, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	entries := strings.Split(value, ",")
	envVars := make([]corev1.EnvVar, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid environment variable %q, expected KEY=VALUE", entry)
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("environment variable name must not be empty")
		}
		envVars = append(envVars, corev1.EnvVar{Name: name, Value: parts[1]})
	}
	return envVars, nil
}

func appendMissingEnvVars(container *corev1.Container, envVars []corev1.EnvVar) bool {
	existing := make(map[string]struct{}, len(container.Env))
	for _, env := range container.Env {
		existing[env.Name] = struct{}{}
	}

	changed := false
	for _, env := range envVars {
		if _, found := existing[env.Name]; found {
			continue
		}
		container.Env = append(container.Env, env)
		existing[env.Name] = struct{}{}
		changed = true
	}
	return changed
}

func shouldInjectNamespace(namespace string, includedNamespaces, excludedNamespaces []string) bool {
	if len(includedNamespaces) == 0 {
		return !namespaceInList(namespace, excludedNamespaces)
	}
	return namespaceInList(namespace, includedNamespaces)
}

func namespaceInList(namespace string, namespaces []string) bool {
	for _, configuredNamespace := range namespaces {
		if namespace == strings.TrimSpace(configuredNamespace) {
			return true
		}
	}
	return false
}

func hasKAEResource(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if resourceListHasKAE(container.Resources.Requests) || resourceListHasKAE(container.Resources.Limits) {
			return true
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if resourceListHasKAE(container.Resources.Requests) || resourceListHasKAE(container.Resources.Limits) {
			return true
		}
	}
	return false
}

func resourceListHasKAE(resources corev1.ResourceList) bool {
	for name := range resources {
		if strings.HasPrefix(string(name), kaeResourcePrefix) {
			return true
		}
	}
	return false
}
