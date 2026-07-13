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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pod Webhook", func() {
	Context("When creating Pod under Defaulting Webhook", func() {
		It("Should apply defaults when a required field is empty", func() {
			pod := newWebhookTestPod("default-injection", "default", "app")
			err := k8sClient.Create(context.Background(), pod)
			Expect(err).ToNot(HaveOccurred())

			expectInjectedKAEConfig(pod.Spec.Containers[0], "auto")
		})

		It("Should preserve an existing KAE resource and still inject environment variables", func() {
			pod := newWebhookTestPod("existing-resource", "default", "app")
			existingName := corev1.ResourceName("kae.kunpeng.com/hisi_zip")
			pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
				existingName: resource.MustParse("2"),
			}

			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())

			existing := pod.Spec.Containers[0].Resources.Limits[existingName]
			Expect(existing.String()).To(Equal("2"))
			_, found := pod.Spec.Containers[0].Resources.Limits["kae.kunpeng.com/hisi_hpre"]
			Expect(found).To(BeFalse())
			_, found = pod.Spec.Containers[0].Resources.Requests["kae.kunpeng.com/hisi_hpre"]
			Expect(found).To(BeFalse())
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: "KAE_MODE", Value: "auto",
			}))
		})

		It("Should preserve an existing environment variable", func() {
			pod := newWebhookTestPod("existing-env", "default", "app")
			pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "KAE_MODE", Value: "manual"}}

			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())

			expectInjectedKAEConfig(pod.Spec.Containers[0], "manual")
			Expect(pod.Spec.Containers[0].Env).To(HaveLen(1))
		})

		It("Should inject only the configured target container", func() {
			pod := newWebhookTestPod("multiple-containers", "default", "app", "sidecar")

			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())

			expectInjectedKAEConfig(pod.Spec.Containers[0], "auto")
			Expect(pod.Spec.Containers[1].Resources.Requests).To(BeEmpty())
			Expect(pod.Spec.Containers[1].Resources.Limits).To(BeEmpty())
			Expect(pod.Spec.Containers[1].Env).To(BeEmpty())
		})

		It("Should skip excluded namespaces", func() {
			pod := newWebhookTestPod("excluded-namespace", "kube-system", "app")

			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())

			Expect(pod.Spec.Containers[0].Resources.Requests).To(BeEmpty())
			Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
			Expect(pod.Spec.Containers[0].Env).To(BeEmpty())
		})

		It("Should remain idempotent when a Pod is updated", func() {
			pod := newWebhookTestPod("idempotent-update", "default", "app")
			Expect(k8sClient.Create(context.Background(), pod)).To(Succeed())

			pod.Labels = map[string]string{"updated": "true"}
			Expect(k8sClient.Update(context.Background(), pod)).To(Succeed())

			expectInjectedKAEConfig(pod.Spec.Containers[0], "auto")
			Expect(pod.Spec.Containers[0].Env).To(HaveLen(1))
		})
	})
})

func newWebhookTestPod(name, namespace string, containerNames ...string) *corev1.Pod {
	containers := make([]corev1.Container, 0, len(containerNames))
	for _, containerName := range containerNames {
		containers = append(containers, corev1.Container{Name: containerName, Image: "nginx"})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       corev1.PodSpec{Containers: containers},
	}
}

func expectInjectedKAEConfig(container corev1.Container, envValue string) {
	GinkgoHelper()
	resourceName := corev1.ResourceName("kae.kunpeng.com/hisi_hpre")
	request, found := container.Resources.Requests[resourceName]
	Expect(found).To(BeTrue())
	Expect(request.String()).To(Equal("1"))
	limit, found := container.Resources.Limits[resourceName]
	Expect(found).To(BeTrue())
	Expect(limit.String()).To(Equal("1"))
	Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "KAE_MODE", Value: envValue}))
}
