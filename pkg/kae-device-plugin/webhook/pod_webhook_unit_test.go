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
	"context"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestPodCustomDefaulterDefault(t *testing.T) {
	defaulter := &PodCustomDefaulter{Config: InjectionConfig{
		Enabled:         true,
		DefaultResource: "hisi_hpre",
		DefaultCount:    1,
		EnvVars:         []corev1.EnvVar{{Name: "KAE_MODE", Value: "auto"}},
	}}
	pod := newMutationTestPod("default", "app")

	if err := defaulter.Default(context.Background(), pod); err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	assertResourceQuantity(t, pod.Spec.Containers[0], "kae.kunpeng.com/hisi_hpre", "1")
	if got := findMutationTestEnv(pod.Spec.Containers[0].Env, "KAE_MODE"); got != "auto" {
		t.Fatalf("KAE_MODE = %q, want %q", got, "auto")
	}
}

func TestPodCustomDefaulterDefaultReturnsMutationError(t *testing.T) {
	defaulter := &PodCustomDefaulter{Config: InjectionConfig{
		Enabled:         true,
		DefaultResource: "hisi_hpre",
		DefaultCount:    1,
	}}

	if err := defaulter.Default(context.Background(), &corev1.Pod{}); err == nil {
		t.Fatal("Default() error = nil, want missing container error")
	}
}

func TestPodCustomDefaulterUsesAdmissionRequestNamespace(t *testing.T) {
	defaulter := &PodCustomDefaulter{Config: InjectionConfig{
		Enabled:            true,
		DefaultResource:    "hisi_hpre",
		DefaultCount:       1,
		IncludedNamespaces: []string{"test-ns"},
	}}
	pod := newMutationTestPod("", "app")
	ctx := admission.NewContextWithRequest(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{Namespace: "test-ns"},
	})

	if err := defaulter.Default(ctx, pod); err != nil {
		t.Fatalf("Default() error = %v", err)
	}

	assertResourceQuantity(t, pod.Spec.Containers[0], "kae.kunpeng.com/hisi_hpre", "1")
	if pod.Namespace != "" {
		t.Fatalf("pod namespace = %q, want unchanged empty namespace", pod.Namespace)
	}
}
