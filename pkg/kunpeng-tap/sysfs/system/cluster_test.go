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

var _ = Describe("Cluster", func() {
	Describe("cluster struct", func() {
		It("should return correct properties", func() {
			c := &cluster{
				id:     1,
				nodeID: 0,
				cpus:   MustParse("0-7"),
			}

			Expect(c.ID()).To(Equal(ID(1)))
			Expect(c.NodeID()).To(Equal(ID(0)))
			Expect(c.CPUSet().String()).To(Equal("0-7"))
			Expect(c.Size()).To(Equal(8))
		})
	})

	Describe("ClusterInfo", func() {
		var ci *ClusterInfo

		BeforeEach(func() {
			ci = NewClusterInfo()
		})

		Describe("NewClusterInfo", func() {
			It("should create empty ClusterInfo", func() {
				Expect(ci.Clusters).NotTo(BeNil())
				Expect(ci.NodeClusters).NotTo(BeNil())
				Expect(ci.CPUCluster).NotTo(BeNil())
				Expect(ci.HasClusters()).To(BeFalse())
			})
		})

		Describe("HasClusters", func() {
			It("should return false for empty ClusterInfo", func() {
				Expect(ci.HasClusters()).To(BeFalse())
			})

			It("should return true when clusters exist", func() {
				ci.Clusters[0] = &cluster{id: 0, cpus: MustParse("0-7")}
				Expect(ci.HasClusters()).To(BeTrue())
			})

			It("should return false for nil ClusterInfo", func() {
				var nilCI *ClusterInfo
				Expect(nilCI.HasClusters()).To(BeFalse())
			})
		})

		Describe("GetClusterByID", func() {
			It("should return cluster by ID", func() {
				c := &cluster{id: 5, cpus: MustParse("40-47")}
				ci.Clusters[5] = c

				result := ci.GetClusterByID(5)
				Expect(result).To(Equal(c))
			})

			It("should return nil for non-existent ID", func() {
				result := ci.GetClusterByID(999)
				Expect(result).To(BeNil())
			})

			It("should return nil for nil ClusterInfo", func() {
				var nilCI *ClusterInfo
				Expect(nilCI.GetClusterByID(0)).To(BeNil())
			})
		})

		Describe("GetClusterIDForCPU", func() {
			It("should return cluster ID for CPU", func() {
				ci.CPUCluster[5] = 1

				clusterID, ok := ci.GetClusterIDForCPU(5)
				Expect(ok).To(BeTrue())
				Expect(clusterID).To(Equal(ID(1)))
			})

			It("should return false for unknown CPU", func() {
				_, ok := ci.GetClusterIDForCPU(999)
				Expect(ok).To(BeFalse())
			})

			It("should return false for nil ClusterInfo", func() {
				var nilCI *ClusterInfo
				id, ok := nilCI.GetClusterIDForCPU(0)
				Expect(ok).To(BeFalse())
				Expect(id).To(Equal(ID(-1)))
			})
		})

		Describe("GetClustersForNode", func() {
			It("should return cluster IDs for node", func() {
				ci.NodeClusters[0] = []ID{0, 1, 2}

				clusters := ci.GetClustersForNode(0)
				Expect(clusters).To(Equal([]ID{0, 1, 2}))
			})

			It("should return nil for unknown node", func() {
				clusters := ci.GetClustersForNode(999)
				Expect(clusters).To(BeNil())
			})

			It("should return nil for nil ClusterInfo", func() {
				var nilCI *ClusterInfo
				Expect(nilCI.GetClustersForNode(0)).To(BeNil())
			})
		})

		Describe("AssociateWithNodes", func() {
			It("should associate clusters with nodes", func() {
				// Create clusters
				ci.Clusters[0] = &cluster{id: 0, nodeID: -1, cpus: MustParse("0-7")}
				ci.Clusters[1] = &cluster{id: 1, nodeID: -1, cpus: MustParse("8-15")}
				ci.Clusters[2] = &cluster{id: 2, nodeID: -1, cpus: MustParse("16-23")}

				// CPU to node mapping
				cpuNodeMap := make(map[ID]ID)
				for i := 0; i < 16; i++ {
					cpuNodeMap[i] = 0 // CPUs 0-15 belong to node 0
				}
				for i := 16; i < 24; i++ {
					cpuNodeMap[i] = 1 // CPUs 16-23 belong to node 1
				}

				ci.AssociateWithNodes(cpuNodeMap)

				// Check node assignments
				Expect(ci.Clusters[0].NodeID()).To(Equal(ID(0)))
				Expect(ci.Clusters[1].NodeID()).To(Equal(ID(0)))
				Expect(ci.Clusters[2].NodeID()).To(Equal(ID(1)))

				// Check NodeClusters mapping
				Expect(ci.NodeClusters[0]).To(Equal([]ID{0, 1}))
				Expect(ci.NodeClusters[1]).To(Equal([]ID{2}))
			})

			It("should handle nil ClusterInfo", func() {
				var nilCI *ClusterInfo
				// Should not panic
				nilCI.AssociateWithNodes(make(map[ID]ID))
			})
		})
	})

	Describe("DiscoverClusters", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "cluster-discover-test-")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(tempDir)
		})

		It("should discover clusters from sysfs", func() {
			// Create fake sysfs structure with 2 clusters (8 CPUs each)
			sysPath := filepath.Join(tempDir, "sys", "devices", "system", "cpu")
			err := os.MkdirAll(sysPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create CPU directories with cluster info
			for i := 0; i < 16; i++ {
				cpuDir := filepath.Join(sysPath, "cpu"+string(rune('0'+i/10))+string(rune('0'+i%10)))
				if i < 10 {
					cpuDir = filepath.Join(sysPath, "cpu"+string(rune('0'+i)))
				}
				topoDir := filepath.Join(cpuDir, "topology")
				err := os.MkdirAll(topoDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				var clusterCPUs string
				if i < 8 {
					clusterCPUs = "0-7"
				} else {
					clusterCPUs = "8-15"
				}
				err = os.WriteFile(filepath.Join(topoDir, "cluster_cpus_list"), []byte(clusterCPUs), 0644)
				Expect(err).NotTo(HaveOccurred())
			}

			ci, err := DiscoverClusters(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(ci).NotTo(BeNil())
			Expect(ci.HasClusters()).To(BeTrue())
			Expect(len(ci.Clusters)).To(Equal(2))
		})

		It("should return nil when cluster topology is not available", func() {
			// Create fake sysfs structure without cluster_cpus_list
			sysPath := filepath.Join(tempDir, "sys", "devices", "system", "cpu")
			cpuDir := filepath.Join(sysPath, "cpu0")
			topoDir := filepath.Join(cpuDir, "topology")
			err := os.MkdirAll(topoDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			// No cluster_cpus_list file

			ci, err := DiscoverClusters(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(ci).To(BeNil())
		})

		It("should return error when no CPU entries found", func() {
			// Create empty sysfs structure
			sysPath := filepath.Join(tempDir, "sys", "devices", "system", "cpu")
			err := os.MkdirAll(sysPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			ci, err := DiscoverClusters(tempDir)
			Expect(err).To(HaveOccurred())
			Expect(ci).To(BeNil())
		})
	})

	Describe("GetMaxClusterSize", func() {
		It("should return 0 for nil ClusterInfo", func() {
			var ci *ClusterInfo
			Expect(ci.GetMaxClusterSize()).To(Equal(0))
		})

		It("should return 0 for empty ClusterInfo", func() {
			ci := NewClusterInfo()
			Expect(ci.GetMaxClusterSize()).To(Equal(0))
		})

		It("should return the max cluster size", func() {
			ci := NewClusterInfo()
			// Add clusters with different sizes
			cpus0, _ := cpuset.Parse("0-7")   // size 8
			cpus1, _ := cpuset.Parse("8-23")  // size 16
			cpus2, _ := cpuset.Parse("24-35") // size 12
			ci.Clusters[0] = &cluster{id: 0, nodeID: 0, cpus: cpus0}
			ci.Clusters[1] = &cluster{id: 1, nodeID: 0, cpus: cpus1}
			ci.Clusters[2] = &cluster{id: 2, nodeID: 1, cpus: cpus2}

			Expect(ci.GetMaxClusterSize()).To(Equal(16))
		})

		It("should return single cluster size when only one cluster exists", func() {
			ci := NewClusterInfo()
			cpus, _ := cpuset.Parse("0-15") // size 16
			ci.Clusters[0] = &cluster{id: 0, nodeID: 0, cpus: cpus}
			Expect(ci.GetMaxClusterSize()).To(Equal(16))
		})
	})
})
