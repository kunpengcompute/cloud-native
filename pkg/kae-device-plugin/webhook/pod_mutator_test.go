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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutatePodForKAEInjection(t *testing.T) {
	tests := []struct {
		name        string
		config      InjectionConfig
		pod         *corev1.Pod
		wantChanged bool
		wantErr     bool
		verify      func(*testing.T, *corev1.Pod)
	}{
		{
			name: "disabled injection leaves pod unchanged",
			config: InjectionConfig{
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
			},
			pod: newMutationTestPod("default", "app"),
		},
		{
			name: "excluded namespace is skipped",
			config: InjectionConfig{
				Enabled:            true,
				DefaultResource:    "hisi_hpre",
				DefaultCount:       1,
				ExcludedNamespaces: []string{"kube-system"},
			},
			pod: newMutationTestPod("kube-system", "app"),
		},
		{
			name: "included namespace overrides excluded namespace",
			config: InjectionConfig{
				Enabled:            true,
				DefaultResource:    "hisi_hpre",
				DefaultCount:       1,
				IncludedNamespaces: []string{"tenant-a"},
				ExcludedNamespaces: []string{"tenant-a"},
			},
			pod:         newMutationTestPod("tenant-a", "app"),
			wantChanged: true,
		},
		{
			name: "namespace outside non-empty include list is skipped",
			config: InjectionConfig{
				Enabled:            true,
				DefaultResource:    "hisi_hpre",
				DefaultCount:       1,
				IncludedNamespaces: []string{"tenant-a"},
			},
			pod: newMutationTestPod("tenant-b", "app"),
		},
		{
			name: "injects resource into selected container",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    2,
				TargetContainer: 1,
			},
			pod:         newMutationTestPod("default", "sidecar", "app"),
			wantChanged: true,
			verify: func(t *testing.T, pod *corev1.Pod) {
				assertResourceQuantity(t, pod.Spec.Containers[1], "kae.kunpeng.com/hisi_hpre", "2")
				if len(pod.Spec.Containers[0].Resources.Requests) != 0 {
					t.Fatal("non-target container was modified")
				}
			},
		},
		{
			name: "fully qualified resource name is preserved",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "kae.kunpeng.com/hisi_sec2",
				DefaultCount:    1,
			},
			pod:         newMutationTestPod("default", "app"),
			wantChanged: true,
			verify: func(t *testing.T, pod *corev1.Pod) {
				assertResourceQuantity(t, pod.Spec.Containers[0], "kae.kunpeng.com/hisi_sec2", "1")
			},
		},
		{
			name: "existing KAE resource prevents resource injection",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
			},
			pod: podWithExistingKAEResource("kae.kunpeng.com/hisi_zip"),
		},
		{
			name: "init container KAE resource prevents resource injection",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
			},
			pod: podWithInitContainerKAEResource("kae.kunpeng.com/hisi_zip"),
		},
		{
			name: "nil pod returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
			},
			wantErr: true,
		},
		{
			name: "missing containers returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
			},
			pod:     &corev1.Pod{},
			wantErr: true,
		},
		{
			name: "invalid target index returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
				TargetContainer: 1,
			},
			pod:     newMutationTestPod("default", "app"),
			wantErr: true,
		},
		{
			name: "negative target index returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
				TargetContainer: -1,
			},
			pod:     newMutationTestPod("default", "app"),
			wantErr: true,
		},
		{
			name: "non-positive count returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
			},
			pod:     newMutationTestPod("default", "app"),
			wantErr: true,
		},
		{
			name: "empty resource name returns error",
			config: InjectionConfig{
				Enabled:      true,
				DefaultCount: 1,
			},
			pod:     newMutationTestPod("default", "app"),
			wantErr: true,
		},
		{
			name: "invalid resource name returns error",
			config: InjectionConfig{
				Enabled:         true,
				DefaultResource: "bad/name/extra",
				DefaultCount:    1,
			},
			pod:     newMutationTestPod("default", "app"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, err := MutatePodForKAEInjection(tt.pod, tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MutatePodForKAEInjection() error = %v, wantErr %v", err, tt.wantErr)
			}
			if changed != tt.wantChanged {
				t.Fatalf("MutatePodForKAEInjection() changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.verify != nil {
				tt.verify(t, tt.pod)
			}
		})
	}
}

func TestMutatePodForKAEInjectionEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		wantValue       string
		resourceChanged bool
	}{
		{
			name:            "injects missing environment variable",
			pod:             newMutationTestPod("default", "app"),
			wantValue:       "auto",
			resourceChanged: true,
		},
		{
			name: "preserves existing environment variable",
			pod: func() *corev1.Pod {
				pod := newMutationTestPod("default", "app")
				pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "KAE_MODE", Value: "manual"}}
				return pod
			}(),
			wantValue:       "manual",
			resourceChanged: true,
		},
		{
			name:            "injects environment variable when KAE resource exists",
			pod:             podWithExistingKAEResource("kae.kunpeng.com/hisi_zip"),
			wantValue:       "auto",
			resourceChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, err := MutatePodForKAEInjection(tt.pod, InjectionConfig{
				Enabled:         true,
				DefaultResource: "hisi_hpre",
				DefaultCount:    1,
				EnvVars:         []corev1.EnvVar{{Name: "KAE_MODE", Value: "auto"}},
			})
			if err != nil {
				t.Fatalf("MutatePodForKAEInjection() error = %v", err)
			}
			if !changed {
				t.Fatal("MutatePodForKAEInjection() changed = false, want true")
			}
			if got := findMutationTestEnv(tt.pod.Spec.Containers[0].Env, "KAE_MODE"); got != tt.wantValue {
				t.Fatalf("KAE_MODE = %q, want %q", got, tt.wantValue)
			}
			_, resourceExists := tt.pod.Spec.Containers[0].Resources.Limits["kae.kunpeng.com/hisi_hpre"]
			if resourceExists != tt.resourceChanged {
				t.Fatalf("injected resource exists = %v, want %v", resourceExists, tt.resourceChanged)
			}
		})
	}
}

func TestMutatePodForKAEInjectionIsIdempotent(t *testing.T) {
	pod := newMutationTestPod("default", "app")
	config := InjectionConfig{
		Enabled:         true,
		DefaultResource: "hisi_hpre",
		DefaultCount:    1,
		EnvVars:         []corev1.EnvVar{{Name: "KAE_MODE", Value: "auto"}},
	}

	changed, err := MutatePodForKAEInjection(pod, config)
	if err != nil || !changed {
		t.Fatalf("first mutation changed = %v, error = %v", changed, err)
	}
	changed, err = MutatePodForKAEInjection(pod, config)
	if err != nil || changed {
		t.Fatalf("second mutation changed = %v, error = %v", changed, err)
	}
	if len(pod.Spec.Containers[0].Env) != 1 {
		t.Fatalf("environment variable count = %d, want 1", len(pod.Spec.Containers[0].Env))
	}
}

func TestParseEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []corev1.EnvVar
		wantErr bool
	}{
		{name: "empty input"},
		{
			name:  "multiple variables preserve equals in value",
			input: "KAE_MODE=auto,KAE_OPTIONS=a=b",
			want: []corev1.EnvVar{
				{Name: "KAE_MODE", Value: "auto"},
				{Name: "KAE_OPTIONS", Value: "a=b"},
			},
		},
		{name: "missing equals", input: "KAE_MODE", wantErr: true},
		{name: "empty name", input: "=auto", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEnvVars(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEnvVars() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseEnvVars() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func newMutationTestPod(namespace string, containerNames ...string) *corev1.Pod {
	containers := make([]corev1.Container, 0, len(containerNames))
	for _, name := range containerNames {
		containers = append(containers, corev1.Container{Name: name})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: namespace},
		Spec:       corev1.PodSpec{Containers: containers},
	}
}

func podWithExistingKAEResource(name corev1.ResourceName) *corev1.Pod {
	pod := newMutationTestPod("default", "app")
	pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{name: resource.MustParse("1")}
	return pod
}

func podWithInitContainerKAEResource(name corev1.ResourceName) *corev1.Pod {
	pod := newMutationTestPod("default", "app")
	pod.Spec.InitContainers = []corev1.Container{{
		Name: "init",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{name: resource.MustParse("1")},
		},
	}}
	return pod
}

func assertResourceQuantity(t *testing.T, container corev1.Container, name corev1.ResourceName, want string) {
	t.Helper()
	request, found := container.Resources.Requests[name]
	if !found || request.String() != want {
		t.Fatalf("request %s = %q, want %q", name, request.String(), want)
	}
	limit, found := container.Resources.Limits[name]
	if !found || limit.String() != want {
		t.Fatalf("limit %s = %q, want %q", name, limit.String(), want)
	}
}

func findMutationTestEnv(envVars []corev1.EnvVar, name string) string {
	for _, env := range envVars {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}
