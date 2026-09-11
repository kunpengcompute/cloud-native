/*
Copyright 2026 Huawei Technology corp.

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

package v1alpha1

import (
	"testing"

	"google.golang.org/protobuf/runtime/protoimpl"
)

func TestMessageExportersRejectUnexpectedTypes(t *testing.T) {
	if protoimpl.UnsafeEnabled {
		t.Skip("exporters are only installed by the pure Go protobuf runtime")
	}

	file_api_proto_init()
	messages := []interface{}{
		&PodSandboxMetadata{},
		&PodSandboxHookRequest{},
		&PodSandboxHookResponse{},
		&LinuxContainerResources{},
		&HugepageLimit{},
		&ContainerMetadata{},
		&ContainerResourceHookRequest{},
		&ContainerResourceHookResponse{},
	}
	for i, message := range messages {
		exporter := file_api_proto_msgTypes[i].Exporter
		if exporter == nil {
			t.Fatalf("message exporter %d is not installed", i)
		}
		if got := exporter(struct{}{}, 0); got != nil {
			t.Fatalf("message exporter %d accepted an unexpected type", i)
		}
		if got := exporter(message, 0); got == nil {
			t.Fatalf("message exporter %d rejected its message type", i)
		}
	}
}
