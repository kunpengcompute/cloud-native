/*
 * Copyright 2017-2021 Intel Corporation. All Rights Reserved.
 * Modifications Copyright (c) 2025 Huawei Technology corp.
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

package kaeplugin

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	dpapi "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/device-plugin"
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

// fakeNotifier implements Notifier interface.
type fakeNotifier struct {
	scanDone chan bool
	tree     dpapi.DeviceTree
}

// Notify stops plugin Scan.
func (n *fakeNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.tree = newDeviceTree
	n.scanDone <- true
}

func TestNewDevicePlugin(t *testing.T) {
	tcases := []struct {
		name            string
		kernelVfDrivers string
		expectedErr     bool
	}{
		{
			name:            "Wrong KAEDriver",
			kernelVfDrivers: "invalid",
			expectedErr:     true,
		},
		{
			name:            "No errors, one driver",
			kernelVfDrivers: "hisi_hpre",
			expectedErr:     false,
		},
		{
			name:            "No errors, all driver",
			kernelVfDrivers: "hisi_hpre,hisi_zip,hisi_sec2",
			expectedErr:     false,
		},
		{
			name:            "one driver is wrong driver",
			kernelVfDrivers: "hisi_hpre,hisi_zip,hisi_sec2,hisi_test",
			expectedErr:     true,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDevicePlugin(tt.kernelVfDrivers)

			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': expected error", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': expected success", tt.name)
			}
		})
	}
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name            string
		dirs            []string
		files           map[string][]byte
		symlinks        map[string]string
		kernelVfDrivers []string
		expectedErr     bool
	}{
		{
			name:            "test hisi_hpre driver success",
			kernelVfDrivers: []string{"hisi_hpre"},
			dirs: []string{
				"sys/bus/pci/drivers/hisi_hpre",
				"sys/bus/pci/devices/0000:3a:00.1/uacce/hisi_hpre-10",
				"sys/bus/pci/devices/0000:3a:00.1",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:3a:00.1/device": []byte("0xa259"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/hisi_hpre/0000:3a:00.1": "sys/bus/pci/devices/0000:3a:00.1",
			},
			expectedErr: false,
		},
		{
			name:            "test hisi_zip driver success",
			kernelVfDrivers: []string{"hisi_zip"},
			dirs: []string{
				"sys/bus/pci/drivers/hisi_zip",
				"sys/bus/pci/devices/0000:3a:00.1/uacce/hisi_zip-10",
				"sys/bus/pci/devices/0000:3a:00.1",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:3a:00.1/device": []byte("0xa251"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/hisi_zip/0000:3a:00.1": "sys/bus/pci/devices/0000:3a:00.1",
			},
			expectedErr: false,
		},
		{
			name:            "test hisi_sec2 driver success",
			kernelVfDrivers: []string{"hisi_sec2"},
			dirs: []string{
				"sys/bus/pci/drivers/hisi_sec2",
				"sys/bus/pci/devices/0000:3a:00.1/uacce/hisi_sec2-10",
				"sys/bus/pci/devices/0000:3a:00.1",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:3a:00.1/device": []byte("0xa256"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/hisi_sec2/0000:3a:00.1": "sys/bus/pci/devices/0000:3a:00.1",
			},
			expectedErr: false,
		},
		{
			name:            "test no uacce dir failed",
			kernelVfDrivers: []string{"hisi_sec2"},
			dirs: []string{
				"sys/bus/pci/drivers/hisi_sec2",
				"sys/bus/pci/devices/0000:3a:00.1",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:3a:00.1/device": []byte("0xa256"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/hisi_sec2/0000:3a:00.1": "sys/bus/pci/devices/0000:3a:00.1",
			},
			expectedErr: true,
		},
		{
			name:            "test hisi_xx dir in uacce dir failed",
			kernelVfDrivers: []string{"hisi_sec2"},
			dirs: []string{
				"sys/bus/pci/drivers/hisi_sec2",
				"sys/bus/pci/devices/0000:3a:00.1",
				"sys/bus/pci/devices/0000:3a:00.1/uacce",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:3a:00.1/device": []byte("0xa256"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/hisi_sec2/0000:3a:00.1": "sys/bus/pci/devices/0000:3a:00.1",
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

			dp := newDevicePlugin(
				filepath.Join(tmpdir, pciDriverDirectory),
				filepath.Join(tmpdir, pciDeviceDirectory),
				filepath.Join(tmpdir, uacceDirectory),
				tt.kernelVfDrivers,
			)

			fN := fakeNotifier{
				scanDone: dp.scanDone,
			}

			err = dp.Scan(&fN)

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
