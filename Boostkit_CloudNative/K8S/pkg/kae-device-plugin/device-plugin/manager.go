// Copyright 2018 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deviceplugin

// updateInfo contains info for added, updated and deleted devices.
type updateInfo struct {
	Added   DeviceTree
	Updated DeviceTree
	Removed DeviceTree
}

// notifier implements Notifier interface.
type notifier struct {
	deviceTree DeviceTree
	updatesCh  chan<- updateInfo
}

func newNotifier(updatesCh chan<- updateInfo) *notifier {
	return &notifier{
		updatesCh: updatesCh,
	}
}

func (n *notifier) Notify(newDeviceTree DeviceTree) {

}

// Manager manages life cycle of device plugins and handles the scan results
// received from them.
type Manager struct {
	devicePlugin Scanner
	servers      map[string]server
	namespace    string
}

// NewManager creates a new instance of Manager.
func NewManager(namespace string, devicePlugin Scanner) *Manager {
	return &Manager{
		devicePlugin: devicePlugin,
		namespace:    namespace,
		servers:      make(map[string]server),
	}
}

// Run prepares and launches event loop for updates from Scanner.
func (m *Manager) Run() {

}

func (m *Manager) handleUpdate(update updateInfo) {

}
