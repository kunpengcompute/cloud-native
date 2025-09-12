/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package kaeqos

import (
	"flag"
	"os"
	"path"
	"testing"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

func createTestFiles(prefix string, dirs []string, files map[string][]byte, symlinks map[string]string) error {
	for _, dir := range dirs {
		err := os.MkdirAll(path.Join(prefix, dir), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for filename, body := range files {
		err := os.WriteFile(path.Join(prefix, filename), body, 0600)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	for link, target := range symlinks {
		err := os.MkdirAll(path.Join(prefix, target), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake symlink target directory")
		}

		err = os.Symlink(path.Join(prefix, target), path.Join(prefix, link))
		if err != nil {
			return errors.Wrap(err, "Failed to create fake symlink")
		}
	}

	return nil
}

func fakeNewPodResourceClient() *podResourceClient {
	return &podResourceClient{}
}

func TestUpdateQos(t *testing.T) {
	podResourceClient := fakeNewPodResourceClient()
	podResourceClient.podResources = map[string]ResourceInfo{
		"default/kae-test": {
			"kae.kunpeng.com/hisi_hpre": {"0000:3a:00.1"},
		},
	}
	qosManager := &QosManager{
		client: podResourceClient,
	}

	tcases := []struct {
		name        string
		dirs        []string
		files       map[string][]byte
		symlinks    map[string]string
		pod         *corev1.Pod
		expectedErr bool
	}{
		{
			name: "set Qos for hpre device success",
			dirs: []string{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0",
				"sys/bus/pci/devices/0000:3a:00.1",
				"sys/bus/pci/devices/0000:3a:00.0",
			},
			files: map[string][]byte{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0/alg_qos": []byte("1000"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:3a:00.1/physfn": "sys/bus/pci/devices/0000:3a:00.0",
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "kae-test",
					Annotations: map[string]string{
						"qos.kae.kunpeng.com/hisi_hpre": "500",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kae-test",
							Image: "nginx",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"kae.kunpeng.com/hisi_hpre": resource.Quantity{
										Format: "1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "set Qos for hpre device failed because invalid qos value",
			dirs: []string{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0",
				"sys/bus/pci/devices/0000:3a:00.1",
				"sys/bus/pci/devices/0000:3a:00.0",
			},
			files: map[string][]byte{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0/alg_qos": []byte("1000"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:3a:00.1/physfn": "sys/bus/pci/devices/0000:3a:00.0",
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "kae-test",
					Annotations: map[string]string{
						"qos.kae.kunpeng.com/hisi_hpre": "invalid",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kae-test",
							Image: "nginx",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"kae.kunpeng.com/hisi_hpre": resource.Quantity{
										Format: "1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "set Qos for hpre device failed, because qos value is invalid, use float value",
			dirs: []string{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0",
				"sys/bus/pci/devices/0000:3a:00.1",
				"sys/bus/pci/devices/0000:3a:00.0",
			},
			files: map[string][]byte{
				"sys/kernel/debug/hisi_hpre/0000:3a:00.0/alg_qos": []byte("1000"),
				"sys/bus/pci/devices/0000:3a:00.1/device":         []byte("0xa259"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:3a:00.1/physfn": "sys/bus/pci/devices/0000:3a:00.0",
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "kae-test",
					Annotations: map[string]string{
						"qos.kae.kunpeng.com/hisi_hpre": "123.456",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kae-test",
							Image: "nginx",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"kae.kunpeng.com/hisi_hpre": resource.Quantity{
										Format: "1",
									},
								},
							},
						},
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp("/tmp/", "kaeplugin-TestScanPrivate-*")
			if err != nil {
				t.Fatal(err)
			}

			if err = createTestFiles(tmpdir, tt.dirs, tt.files, tt.symlinks); err != nil {
				t.Fatalf("%+v", err)
			}

			kernelDebugPath = path.Join(tmpdir, "sys/kernel/debug")
			pciDeviceDir = path.Join(tmpdir, "sys/bus/pci/devices")

			err = qosManager.updateQos(tt.pod)

			if tt.expectedErr && err == nil {
				t.Errorf("expected error, but got success")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("got unexpected error: %+v", err)
			}

			if err = os.RemoveAll(tmpdir); err != nil {
				t.Fatal(err)
			}
		})
	}
}
