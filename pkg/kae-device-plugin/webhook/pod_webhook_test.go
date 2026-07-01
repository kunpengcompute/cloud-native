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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pod Webhook", func() {
	Context("When creating Pod under Defaulting Webhook", func() {
		It("Should apply defaults when a required field is empty", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx",
						},
					},
				},
			}
			err := k8sClient.Create(context.Background(), pod)
			Expect(err).ToNot(HaveOccurred())

			resourceName := corev1.ResourceName("kae.kunpeng.com/hisi_hpre")
			request, found := pod.Spec.Containers[0].Resources.Requests[resourceName]
			Expect(found).To(BeTrue())
			Expect(request.String()).To(Equal("1"))
			limit, found := pod.Spec.Containers[0].Resources.Limits[resourceName]
			Expect(found).To(BeTrue())
			Expect(limit.String()).To(Equal("1"))
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "KAE_MODE",
				Value: "auto",
			}))
		})
	})
})
