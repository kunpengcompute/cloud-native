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

package system

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/cpuset"
)

// TestHelper provides utilities for system tests
type TestHelper struct {
	tempDir string
}

func NewTestHelper() *TestHelper {
	tempDir, err := os.MkdirTemp("", "system-test-")
	Expect(err).NotTo(HaveOccurred())
	return &TestHelper{
		tempDir: tempDir,
	}
}

func (h *TestHelper) Cleanup() {
	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
	}
}

func (h *TestHelper) TempDir() string {
	return h.tempDir
}

func (h *TestHelper) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(filepath.Join(h.tempDir, path), perm)
}

func (h *TestHelper) WriteFileContents(path, content string) error {
	fullPath := filepath.Join(h.tempDir, path)
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

var _ = Describe("System", func() {
	var (
		helper *TestHelper
		// testPath string
		sys *system
	)

	BeforeEach(func() {
		helper = NewTestHelper()

		sys = &system{
			path:     helper.TempDir(),
			cpus:     make(map[ID]CPU),
			isolated: cpuset.New(),
		}
	})

	AfterEach(func() {
		helper.Cleanup()
	})

	Context("discoverCPU", func() {
		type testCase struct {
			cpuID    string
			files    map[string]string
			expected *cpu
			wantErr  bool
		}

		DescribeTable("discovering CPU information",
			func(tc testCase) {
				// 创建基本的CPU目录结构
				cpuPath := filepath.Join("devices/system/cpu", tc.cpuID)
				err := helper.MkdirAll(cpuPath, 0755)
				Expect(err).NotTo(HaveOccurred())

				// 创建topology目录
				topologyPath := filepath.Join(cpuPath, "topology")
				err = helper.MkdirAll(topologyPath, 0755)
				Expect(err).NotTo(HaveOccurred())

				// 创建node0目录
				nodePath := filepath.Join(cpuPath, "node0")
				err = helper.MkdirAll(nodePath, 0755)
				Expect(err).NotTo(HaveOccurred())

				// 写入测试文件
				for filename, content := range tc.files {
					err = helper.WriteFileContents(filepath.Join(cpuPath, filename), content)
					Expect(err).NotTo(HaveOccurred())
				}

				fullPath := filepath.Join(helper.TempDir(), cpuPath)
				err = sys.discoverCPU(fullPath)

				if tc.wantErr {
					Expect(err).To(HaveOccurred())
					return
				}

				Expect(err).NotTo(HaveOccurred())
				Expect(sys.cpus).To(HaveKey(tc.expected.id))

				actualCPU := sys.cpus[tc.expected.id].(*cpu)
				Expect(actualCPU.id).To(Equal(tc.expected.id))
				Expect(actualCPU.pkg).To(Equal(tc.expected.pkg))
				Expect(actualCPU.die).To(Equal(tc.expected.die))
				Expect(actualCPU.core).To(Equal(tc.expected.core))
				Expect(actualCPU.node).To(Equal(tc.expected.node))
				Expect(actualCPU.threads).To(Equal(tc.expected.threads))
				Expect(actualCPU.online).To(Equal(tc.expected.online))
				Expect(actualCPU.isolated).To(Equal(tc.expected.isolated))
			},
			Entry("valid cpu with all attributes", testCase{
				cpuID: "cpu0",
				files: map[string]string{
					"online":                        "1",
					"topology/physical_package_id":  "0",
					"topology/die_id":               "0",
					"topology/core_id":              "0",
					"topology/thread_siblings_list": "0,4",
				},
				expected: &cpu{
					id:       0,
					pkg:      0,
					die:      0,
					core:     0,
					node:     0,
					threads:  cpuset.New(0, 4),
					online:   true,
					isolated: false,
				},
				wantErr: false,
			}),
			Entry("offline cpu", testCase{
				cpuID: "cpu1",
				files: map[string]string{
					"online": "0",
				},
				expected: &cpu{
					id:       1,
					online:   false,
					isolated: false,
				},
				wantErr: false,
			}),
			Entry("missing physical_package_id", testCase{
				cpuID: "cpu2",
				files: map[string]string{
					"online":                        "1",
					"topology/core_id":              "2",
					"topology/thread_siblings_list": "2",
				},
				expected: &cpu{
					id:       2,
					pkg:      -1,
					die:      -1,
					core:     2,
					node:     0,
					threads:  cpuset.New(2),
					online:   true,
					isolated: false,
				},
				wantErr: false,
			}),
			Entry("missing core_id", testCase{
				cpuID: "cpu3",
				files: map[string]string{
					"online":                        "1",
					"topology/physical_package_id":  "0",
					"topology/thread_siblings_list": "3",
				},
				expected: &cpu{
					id:       3,
					pkg:      0,
					die:      -1,
					core:     -1,
					node:     0,
					threads:  cpuset.New(3),
					online:   true,
					isolated: false,
				},
				wantErr: false,
			}),
		)

		Context("when CPU is isolated", func() {
			It("should mark CPU as isolated", func() {
				sys.isolated = cpuset.New(0)

				cpuPath := filepath.Join("devices/system/cpu/cpu0")
				topologyPath := filepath.Join(cpuPath, "topology")

				// 创建所需的目录结构
				err := helper.MkdirAll(cpuPath, 0755)
				Expect(err).NotTo(HaveOccurred())
				err = helper.MkdirAll(topologyPath, 0755)
				Expect(err).NotTo(HaveOccurred())
				err = helper.MkdirAll(filepath.Join(cpuPath, "node0"), 0755)
				Expect(err).NotTo(HaveOccurred())

				files := map[string]string{
					"online":                        "1",
					"topology/physical_package_id":  "0",
					"topology/core_id":              "0",
					"topology/thread_siblings_list": "0",
				}
				for filename, content := range files {
					err = helper.WriteFileContents(filepath.Join(cpuPath, filename), content)
					Expect(err).NotTo(HaveOccurred())
				}

				fullPath := filepath.Join(helper.TempDir(), cpuPath)
				err = sys.discoverCPU(fullPath)
				Expect(err).NotTo(HaveOccurred())

				actualCPU := sys.cpus[0].(*cpu)
				Expect(actualCPU.isolated).To(BeTrue())
			})
		})

		Context("when thread_siblings_list affects threadsPerCore", func() {
			It("should update threadsPerCore correctly", func() {
				cpuPath := filepath.Join("devices/system/cpu/cpu0")
				topologyPath := filepath.Join(cpuPath, "topology")

				// 创建所需的目录结构
				err := helper.MkdirAll(cpuPath, 0755)
				Expect(err).NotTo(HaveOccurred())
				err = helper.MkdirAll(topologyPath, 0755)
				Expect(err).NotTo(HaveOccurred())
				err = helper.MkdirAll(filepath.Join(cpuPath, "node0"), 0755)
				Expect(err).NotTo(HaveOccurred())

				files := map[string]string{
					"online":                        "1",
					"topology/physical_package_id":  "0",
					"topology/core_id":              "0",
					"topology/thread_siblings_list": "0,4,8",
				}
				for filename, content := range files {
					err = helper.WriteFileContents(filepath.Join(cpuPath, filename), content)
					Expect(err).NotTo(HaveOccurred())
				}

				fullPath := filepath.Join(helper.TempDir(), cpuPath)
				err = sys.discoverCPU(fullPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(sys.threadsPerCore).To(Equal(3))
			})
		})
	})

	Context("node discovery", func() {
		Context("discoverNode", func() {
			BeforeEach(func() {
				sys.nodes = make(map[ID]Node) // 确保每个测试前重置 nodes map
			})
			type testCase struct {
				nodeID   string
				files    map[string]string
				expected *node
				wantErr  bool
			}

			DescribeTable("discovering single node information",
				func(tc testCase) {
					nodePath := filepath.Join("devices/system/node", tc.nodeID)
					err := helper.MkdirAll(nodePath, 0755)
					Expect(err).NotTo(HaveOccurred())

					for filename, content := range tc.files {
						err = helper.WriteFileContents(filepath.Join(nodePath, filename), content)
						Expect(err).NotTo(HaveOccurred())
					}

					fullPath := filepath.Join(helper.TempDir(), nodePath)
					err = sys.discoverNode(fullPath)

					if tc.wantErr {
						Expect(err).To(HaveOccurred())
						return
					}

					Expect(err).NotTo(HaveOccurred())
					actualNode := sys.nodes[tc.expected.id].(*node)
					Expect(actualNode.id).To(Equal(tc.expected.id))
					Expect(actualNode.cpus).To(Equal(tc.expected.cpus))
					Expect(actualNode.distance).To(Equal(tc.expected.distance))
				},
				Entry("valid node with all attributes", testCase{
					nodeID: "node0",
					files: map[string]string{
						"cpulist":  "0-3",
						"distance": "10 20 30 40",
					},
					expected: &node{
						id:       0,
						cpus:     cpuset.New(0, 1, 2, 3),
						distance: []int{10, 20, 30, 40},
					},
				}),
				Entry("node with non-continuous CPU list", testCase{
					nodeID: "node0",
					files: map[string]string{
						"cpulist":  "0,2,4,6",
						"distance": "10",
					},
					expected: &node{
						id:       0,
						cpus:     cpuset.New(0, 2, 4, 6),
						distance: []int{10},
					},
				}),
				Entry("missing cpulist file", testCase{
					nodeID: "node0",
					files: map[string]string{
						"distance": "10",
					},
					wantErr: true,
				}),
				Entry("missing distance file", testCase{
					nodeID: "node0",
					files: map[string]string{
						"cpulist": "0-3",
					},
					wantErr: true,
				}),
			)
		})

		Context("discoverNodes", func() {

			It("should correctly discover multiple nodes", func() {
				// 创建基本系统路径
				sysNodePath := filepath.Join("devices", "system", "node")
				err := helper.MkdirAll(sysNodePath, 0755)
				Expect(err).NotTo(HaveOccurred())

				// 创建必需的系统文件
				err = helper.WriteFileContents(filepath.Join(sysNodePath, "has_normal_memory"), "0-1")
				Expect(err).NotTo(HaveOccurred())
				err = helper.WriteFileContents(filepath.Join(sysNodePath, "has_memory"), "0-1")
				Expect(err).NotTo(HaveOccurred())

				// 创建测试节点数据
				nodes := []struct {
					id       string
					cpulist  string
					distance string
				}{
					{"node0", "0-1", "10 20"},
					{"node1", "2-3", "20 10"},
				}

				// 为每个节点创建目录和文件
				for _, n := range nodes {
					nodePath := filepath.Join(sysNodePath, n.id)
					err := helper.MkdirAll(nodePath, 0755)
					Expect(err).NotTo(HaveOccurred())

					files := map[string]string{
						"cpulist":  n.cpulist,
						"distance": n.distance,
					}
					for filename, content := range files {
						err = helper.WriteFileContents(filepath.Join(nodePath, filename), content)
						Expect(err).NotTo(HaveOccurred())
					}
				}

				// 执行节点发现
				sys.path = helper.TempDir() // 设置正确的系统路径

				err = sys.discoverNodes()
				Expect(err).NotTo(HaveOccurred())

				// 验证结果
				Expect(len(sys.nodes)).To(Equal(2))

				// 验证 node0
				node0, ok := sys.nodes[0]
				Expect(ok).To(BeTrue())
				Expect(node0.(*node).cpus).To(Equal(cpuset.New(0, 1)))
				Expect(node0.(*node).distance).To(Equal([]int{10, 20}))

				// 验证 node1
				node1, ok := sys.nodes[1]
				Expect(ok).To(BeTrue())
				Expect(node1.(*node).cpus).To(Equal(cpuset.New(2, 3)))
				Expect(node1.(*node).distance).To(Equal([]int{20, 10}))
			})

			It("should handle empty node directory", func() {
				sysNodePath := filepath.Join("devices", "system", "node")
				err := helper.MkdirAll(sysNodePath, 0755)
				Expect(err).NotTo(HaveOccurred())

				sys.path = helper.TempDir() // 设置正确的系统路径
				err = sys.discoverNodes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(sys.nodes)).To(Equal(0))
			})

			It("should handle missing required files", func() {
				sysNodePath := filepath.Join("devices", "system", "node")
				err := helper.MkdirAll(sysNodePath, 0755)
				Expect(err).NotTo(HaveOccurred())

				// 创建节点目录但不创建必需的文件
				nodePath := filepath.Join(sysNodePath, "node0")
				err = helper.MkdirAll(nodePath, 0755)
				Expect(err).NotTo(HaveOccurred())

				sys.path = helper.TempDir() // 设置正确的系统路径
				err = sys.discoverNodes()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("discoverPackages", func() {
		BeforeEach(func() {
			sys.cpus = make(map[ID]CPU)
		})

		It("should handle nil CPU map", func() {
			sys.cpus = nil
			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())
			Expect(sys.packages).To(HaveLen(0))
		})

		It("should handle empty CPU map", func() {
			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())
			Expect(sys.packages).To(HaveLen(0))
		})

		It("should discover single package with one CPU", func() {
			// 准备测试数据
			sys.cpus[0] = &cpu{
				id:      0,
				pkg:     0,
				die:     0,
				node:    0,
				core:    0,
				online:  true,
				threads: cpuset.New(0),
			}

			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())

			// 验证结果
			Expect(sys.packages).To(HaveLen(1))
			Expect(sys.packages).To(HaveKey(ID(0)))

			pkg := sys.packages[0]
			Expect(pkg.CPUSet().List()).To(Equal([]int{0}))
			Expect(pkg.NodeIDs()).To(Equal([]ID{0}))
			Expect(pkg.DieIDs()).To(Equal([]ID{0}))
			Expect(pkg.DieCPUSet(0).List()).To(Equal([]int{0}))
			Expect(pkg.DieNodeIDs(0)).To(Equal([]ID{0}))
		})

		It("should discover single package with multiple CPUs and dies", func() {
			// 准备测试数据
			sys.cpus[0] = &cpu{
				id:      0,
				pkg:     0,
				die:     0,
				node:    0,
				core:    0,
				online:  true,
				threads: cpuset.New(0),
			}
			sys.cpus[1] = &cpu{
				id:      1,
				pkg:     0,
				die:     0,
				node:    0,
				core:    1,
				online:  true,
				threads: cpuset.New(1),
			}
			sys.cpus[2] = &cpu{
				id:      2,
				pkg:     0,
				die:     1,
				node:    1,
				core:    2,
				online:  true,
				threads: cpuset.New(2),
			}

			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())

			// 验证结果
			Expect(sys.packages).To(HaveLen(1))
			Expect(sys.packages).To(HaveKey(ID(0)))

			pkg := sys.packages[0]
			Expect(pkg.CPUSet().List()).To(Equal([]int{0, 1, 2}))
			Expect(pkg.NodeIDs()).To(ConsistOf(ID(0), ID(1)))
			Expect(pkg.DieIDs()).To(ConsistOf(ID(0), ID(1)))
			Expect(pkg.DieCPUSet(0).List()).To(Equal([]int{0, 1}))
			Expect(pkg.DieCPUSet(1).List()).To(Equal([]int{2}))
			Expect(pkg.DieNodeIDs(0)).To(Equal([]ID{0}))
			Expect(pkg.DieNodeIDs(1)).To(Equal([]ID{1}))
		})

		It("should discover multiple packages", func() {
			// 准备测试数据
			sys.cpus[0] = &cpu{
				id:      0,
				pkg:     0,
				die:     0,
				node:    0,
				core:    0,
				online:  true,
				threads: cpuset.New(0),
			}
			sys.cpus[1] = &cpu{
				id:      1,
				pkg:     0,
				die:     0,
				node:    0,
				core:    1,
				online:  true,
				threads: cpuset.New(1),
			}
			sys.cpus[2] = &cpu{
				id:      2,
				pkg:     1,
				die:     0,
				node:    1,
				core:    0,
				online:  true,
				threads: cpuset.New(2),
			}
			sys.cpus[3] = &cpu{
				id:      3,
				pkg:     1,
				die:     1,
				node:    2,
				core:    1,
				online:  true,
				threads: cpuset.New(3),
			}

			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())

			// 验证结果
			Expect(sys.packages).To(HaveLen(2))
			Expect(sys.packages).To(HaveKey(ID(0)))
			Expect(sys.packages).To(HaveKey(ID(1)))

			// 验证第一个包
			pkg0 := sys.packages[0]
			Expect(pkg0.CPUSet().List()).To(Equal([]int{0, 1}))
			Expect(pkg0.NodeIDs()).To(ConsistOf(ID(0)))
			Expect(pkg0.DieIDs()).To(ConsistOf(ID(0)))
			Expect(pkg0.DieCPUSet(0).List()).To(Equal([]int{0, 1}))
			Expect(pkg0.DieNodeIDs(0)).To(Equal([]ID{0}))

			// 验证第二个包
			pkg1 := sys.packages[1]
			Expect(pkg1.CPUSet().List()).To(Equal([]int{2, 3}))
			Expect(pkg1.NodeIDs()).To(ConsistOf(ID(1), ID(2)))
			Expect(pkg1.DieIDs()).To(ConsistOf(ID(0), ID(1)))
			Expect(pkg1.DieCPUSet(0).List()).To(Equal([]int{2}))
			Expect(pkg1.DieCPUSet(1).List()).To(Equal([]int{3}))
			Expect(pkg1.DieNodeIDs(0)).To(Equal([]ID{1}))
			Expect(pkg1.DieNodeIDs(1)).To(Equal([]ID{2}))
		})

		It("should not ignore offline CPUs", func() {
			// 准备测试数据
			sys.cpus[0] = &cpu{
				id:      0,
				pkg:     0,
				die:     0,
				node:    0,
				core:    0,
				online:  true,
				threads: cpuset.New(0),
			}
			sys.cpus[1] = &cpu{
				id:      1,
				pkg:     0,
				die:     0,
				node:    0,
				core:    1,
				online:  false,
				threads: cpuset.New(1),
			}

			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())

			// 验证结果
			Expect(sys.packages).To(HaveLen(1))
			pkg := sys.packages[0]
			// 应该包含所有 CPU，包括离线的
			Expect(pkg.CPUSet().List()).To(Equal([]int{0, 1}))
			Expect(pkg.NodeIDs()).To(Equal([]ID{0}))
			Expect(pkg.DieIDs()).To(Equal([]ID{0}))
			Expect(pkg.DieCPUSet(0).List()).To(Equal([]int{0, 1}))
			Expect(pkg.DieNodeIDs(0)).To(Equal([]ID{0}))
		})

		It("should set die information on nodes during package discovery", func() {
			// 准备测试数据
			sys.cpus[0] = &cpu{
				id:      0,
				pkg:     0,
				die:     0,
				node:    0,
				core:    0,
				online:  true,
				threads: cpuset.New(0),
			}
			sys.cpus[1] = &cpu{
				id:      1,
				pkg:     0,
				die:     1,
				node:    1,
				core:    1,
				online:  true,
				threads: cpuset.New(1),
			}

			// 创建 test nodes
			sys.nodes = make(map[ID]Node)
			sys.nodes[0] = &node{id: 0}
			sys.nodes[1] = &node{id: 1}

			err := sys.discoverPackages()
			Expect(err).NotTo(HaveOccurred())

			// 验证 die 信息已正确设置到节点上
			Expect(sys.nodes[0].DieID()).To(Equal(ID(0)))
			Expect(sys.nodes[1].DieID()).To(Equal(ID(1)))
		})
	})

	Context("Discover", func() {
		var (
			sys    *system
			tmpDir string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "system-test-*")
			Expect(err).NotTo(HaveOccurred())

			sys = &system{
				path: tmpDir,
			}

			// 创建基础目录结构和通用文件
			err = os.MkdirAll(filepath.Join(tmpDir, "devices/system/cpu"), 0755)
			Expect(err).NotTo(HaveOccurred())

			// 创建 isolated CPUs 文件
			err = os.WriteFile(filepath.Join(tmpDir, "devices/system/cpu/isolated"), []byte(""), 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if tmpDir != "" {
				os.RemoveAll(tmpDir)
			}
		})

		It("should discover system topology successfully", func() {
			// 准备测试数据
			// 创建 CPU 相关文件和目录
			cpuPath := filepath.Join(tmpDir, "devices/system/cpu")
			files := map[string]string{
				"cpu0/topology/physical_package_id":  "0",
				"cpu0/topology/die_id":               "0",
				"cpu0/topology/core_id":              "0",
				"cpu0/topology/thread_siblings_list": "0",
				"cpu0/online":                        "1",
				"cpu1/topology/physical_package_id":  "0",
				"cpu1/topology/die_id":               "0",
				"cpu1/topology/core_id":              "1",
				"cpu1/topology/thread_siblings_list": "1",
				"cpu1/online":                        "1",
			}

			for filename, content := range files {
				fullPath := filepath.Join(cpuPath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建 NUMA 节点相关文件和目录
			nodePath := filepath.Join(tmpDir, "devices/system/node")
			nodeFiles := map[string]string{
				"node0/cpulist":  "0-1",
				"node0/distance": "10",
			}

			for filename, content := range nodeFiles {
				fullPath := filepath.Join(nodePath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建 CPU 到 Node 的链接
			err := os.MkdirAll(filepath.Join(cpuPath, "cpu0/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.MkdirAll(filepath.Join(cpuPath, "cpu1/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())

			// 执行发现
			err = sys.Discover()
			Expect(err).NotTo(HaveOccurred())

			// 验证发现结果
			// 验证 CPU
			Expect(sys.cpus).To(HaveLen(2))
			Expect(sys.cpus[0].PackageID()).To(Equal(ID(0)))
			Expect(sys.cpus[0].DieID()).To(Equal(ID(0)))
			Expect(sys.cpus[0].NodeID()).To(Equal(ID(0)))
			Expect(sys.cpus[0].CoreID()).To(Equal(ID(0)))
			Expect(sys.cpus[0].Online()).To(BeTrue())

			// 验证 Node
			Expect(sys.nodes).To(HaveLen(1))
			node0 := sys.nodes[0]
			Expect(node0.CPUSet().String()).To(Equal("0-1"))
			Expect(node0.PackageID()).To(Equal(ID(0)))
			Expect(node0.DieID()).To(Equal(ID(0)))
			Expect(node0.Distance()).To(Equal([]int{10}))

			// 验证 Package
			Expect(sys.packages).To(HaveLen(1))
			pkg0 := sys.packages[0]
			Expect(pkg0.CPUSet().String()).To(Equal("0-1"))
			Expect(pkg0.NodeIDs()).To(Equal([]ID{0}))
			Expect(pkg0.DieIDs()).To(Equal([]ID{0}))
			Expect(pkg0.DieCPUSet(0).String()).To(Equal("0-1"))
		})

		It("should handle missing CPU topology files", func() {
			cpuPath := filepath.Join(tmpDir, "devices/system/cpu")
			err := os.MkdirAll(filepath.Join(cpuPath, "cpu0"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = sys.Discover()
			Expect(err).To(HaveOccurred())
		})

		It("should handle missing NUMA node files", func() {
			// 创建基本的 CPU 文件
			cpuPath := filepath.Join(tmpDir, "devices/system/cpu")
			files := map[string]string{
				"cpu0/topology/physical_package_id":  "0",
				"cpu0/topology/die_id":               "0",
				"cpu0/topology/core_id":              "0",
				"cpu0/topology/thread_siblings_list": "0",
				"cpu0/online":                        "1",
			}

			for filename, content := range files {
				fullPath := filepath.Join(cpuPath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建 node 目录但不创建 cpulist 文件
			nodePath := filepath.Join(tmpDir, "devices/system/node/node0")
			err := os.MkdirAll(nodePath, 0755)
			Expect(err).NotTo(HaveOccurred())

			// 创建 CPU 到 Node 的链接
			err = os.MkdirAll(filepath.Join(cpuPath, "cpu0/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = sys.Discover()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cpulist"))
		})

		It("should handle invalid node-cpu mapping", func() {
			// 创建 CPU 文件但设置错误的 node 映射
			cpuPath := filepath.Join(tmpDir, "devices/system/cpu")
			files := map[string]string{
				"cpu0/topology/physical_package_id":  "0",
				"cpu0/topology/die_id":               "0",
				"cpu0/topology/core_id":              "0",
				"cpu0/topology/thread_siblings_list": "0",
				"cpu0/online":                        "1",
			}

			for filename, content := range files {
				fullPath := filepath.Join(cpuPath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建多个 node 目录，这会导致错误
			err := os.MkdirAll(filepath.Join(cpuPath, "cpu0/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.MkdirAll(filepath.Join(cpuPath, "cpu0/node1"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = sys.Discover()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one node per cpu allowed"))
		})

		It("should handle offline CPUs correctly", func() {
			// 创建包含离线 CPU 的测试数据
			cpuPath := filepath.Join(tmpDir, "devices/system/cpu")
			files := map[string]string{
				"cpu0/topology/physical_package_id":  "0",
				"cpu0/topology/die_id":               "0",
				"cpu0/topology/core_id":              "0",
				"cpu0/topology/thread_siblings_list": "0",
				"cpu0/online":                        "1",
				"cpu1/topology/physical_package_id":  "0",
				"cpu1/topology/die_id":               "0",
				"cpu1/topology/core_id":              "1",
				"cpu1/topology/thread_siblings_list": "1",
				"cpu1/online":                        "0",
			}

			for filename, content := range files {
				fullPath := filepath.Join(cpuPath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建 NUMA 节点文件
			nodePath := filepath.Join(tmpDir, "devices/system/node")
			nodeFiles := map[string]string{
				"node0/cpulist":  "0-1",
				"node0/distance": "10",
			}

			for filename, content := range nodeFiles {
				fullPath := filepath.Join(nodePath, filename)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				Expect(err).NotTo(HaveOccurred())
				err = os.WriteFile(fullPath, []byte(content), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			// 创建 CPU 到 Node 的链接
			err := os.MkdirAll(filepath.Join(cpuPath, "cpu0/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.MkdirAll(filepath.Join(cpuPath, "cpu1/node0"), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = sys.Discover()
			Expect(err).NotTo(HaveOccurred())

			// 验证结果
			Expect(sys.cpus).To(HaveLen(2))
			Expect(sys.cpus[0].Online()).To(BeTrue())
			Expect(sys.cpus[1].Online()).To(BeFalse())
			Expect(sys.offline.String()).To(Equal("1")) // 改用 String() 方法验证
		})
	})

	Describe("discoverNodeMemory", func() {
		BeforeEach(func() {
			// Create NUMA node directories
			node0Dir := filepath.Join(helper.TempDir(), "devices/system/node/node0")
			node1Dir := filepath.Join(helper.TempDir(), "devices/system/node/node1")
			err := os.MkdirAll(node0Dir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.MkdirAll(node1Dir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create CPU list files for each node
			err = os.WriteFile(filepath.Join(node0Dir, "cpulist"), []byte("0-3"), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(node1Dir, "cpulist"), []byte("4-7"), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create distance files
			err = os.WriteFile(filepath.Join(node0Dir, "distance"), []byte("10 20"), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(node1Dir, "distance"), []byte("20 10"), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create has_memory and has_normal_memory files
			err = os.MkdirAll(filepath.Join(helper.TempDir(), "devices/system/node"), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(helper.TempDir(), "devices/system/node/has_memory"), []byte("0,1"), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(helper.TempDir(), "devices/system/node/has_normal_memory"), []byte("0"), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create meminfo files for each node with different units
			meminfoContent0 := `Node 0 MemTotal:       16384 MB
Node 0 MemFree:        8 GB
Node 0 MemUsed:        8388608 kB
`
			err = os.WriteFile(filepath.Join(node0Dir, "meminfo"), []byte(meminfoContent0), 0644)
			Expect(err).NotTo(HaveOccurred())

			meminfoContent1 := `Node 1 MemTotal:       8388608 kB
Node 1 MemFree:        4194304 kB
Node 1 MemUsed:        4194304 kB
`
			err = os.WriteFile(filepath.Join(node1Dir, "meminfo"), []byte(meminfoContent1), 0644)
			Expect(err).NotTo(HaveOccurred())

			// First discover the nodes
			err = sys.discoverNodes()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should discover memory information for nodes with different units and convert to KB", func() {
			// Check node0 memory properties
			node0 := sys.nodes[0]
			Expect(node0.HasMemory()).To(BeTrue())
			Expect(node0.HasNormalMemory()).To(BeTrue())
			Expect(node0.GetMemoryType()).To(Equal(MemoryTypeDRAM))

			// Check node0 memory info with MB and GB units converted to KB
			memInfo0, err := node0.MemoryInfo()
			Expect(err).NotTo(HaveOccurred())
			Expect(memInfo0).NotTo(BeNil())
			Expect(memInfo0.MemTotal).To(Equal(uint64(16384 * 1024)))   // MB -> KB
			Expect(memInfo0.MemFree).To(Equal(uint64(8 * 1024 * 1024))) // GB -> KB
			Expect(memInfo0.MemUsed).To(Equal(uint64(8388608)))         // Already in KB

			// Check node1 memory properties
			node1 := sys.nodes[1]
			Expect(node1.HasMemory()).To(BeTrue())
			Expect(node1.HasNormalMemory()).To(BeFalse())
			Expect(node1.GetMemoryType()).To(Equal(MemoryTypeDRAM))

			// Check node1 memory info with kB units (should remain as is)
			memInfo1, err := node1.MemoryInfo()
			Expect(err).NotTo(HaveOccurred())
			Expect(memInfo1).NotTo(BeNil())
			Expect(memInfo1.MemTotal).To(Equal(uint64(8388608)))
			Expect(memInfo1.MemFree).To(Equal(uint64(4194304)))
			Expect(memInfo1.MemUsed).To(Equal(uint64(4194304)))
		})

		It("should handle missing memory information files", func() {
			// Create a new system instance
			newSys := &system{
				path: helper.TempDir(),
			}

			// Remove the has_memory and has_normal_memory files
			os.Remove(filepath.Join(helper.TempDir(), "devices/system/node/has_memory"))
			os.Remove(filepath.Join(helper.TempDir(), "devices/system/node/has_normal_memory"))

			// Discover nodes first
			err := newSys.discoverNodes()
			Expect(err).NotTo(HaveOccurred())

			// Nodes should not have memory flags set
			node0 := newSys.nodes[0]
			Expect(node0.HasMemory()).To(BeFalse())
			Expect(node0.HasNormalMemory()).To(BeFalse())

			node1 := newSys.nodes[1]
			Expect(node1.HasMemory()).To(BeFalse())
			Expect(node1.HasNormalMemory()).To(BeFalse())
		})
	})
})
