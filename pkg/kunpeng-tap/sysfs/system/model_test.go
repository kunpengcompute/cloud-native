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
)

var _ = Describe("Model Detection", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "model-test-")
		Expect(err).NotTo(HaveOccurred())
		ResetSupportsClusterFeatureCache()
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		ResetSupportsClusterFeatureCache()
	})

	Describe("checkCPUInfoFileForCluster", func() {
		Context("when CPU part is 0xd06 (950 model with cluster support)", func() {
			It("should return true", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
BogoMIPS	: 200.00
Features	: fp asimd evtstrm aes pmull sha1 sha2 crc32 atomics
CPU implementer	: 0x48
CPU architecture: 8
CPU variant	: 0x1
CPU part	: 0xd06
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeTrue())
			})
		})

		Context("when CPU part is 0xd02 (not 950 model)", func() {
			It("should return false", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
CPU part	: 0xd02
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})
		})

		Context("when CPU part is not 950 model", func() {
			It("should return false for other model", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
CPU part	: 0xd01
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})

			It("should return false for empty file", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				err := os.WriteFile(cpuinfoPath, []byte(""), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})

			It("should return false when no CPU part line exists", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
BogoMIPS	: 200.00
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})
		})

		Context("when cpuinfo file is not readable", func() {
			It("should return false when file does not exist", func() {
				cpuinfoPath := filepath.Join(tempDir, "nonexistent_file")
				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})
		})

		Context("with various CPU part formats", func() {
			It("should handle leading/trailing whitespace", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `CPU part	:   0xd06
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeTrue())
			})

			It("should handle multiple processors and find 950 model", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
CPU part	: 0xd06
CPU revision	: 0

processor	: 1
CPU part	: 0xd06
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("SupportsClusterFeature", func() {
		Context("when CPU is 950 model with cluster support", func() {
			It("should return true for cluster feature support", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
BogoMIPS	: 200.00
Features	: fp asimd evtstrm aes pmull sha1 sha2 crc32 atomics
CPU implementer	: 0x48
CPU architecture: 8
CPU variant	: 0x1
CPU part	: 0xd06
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeTrue())
			})
		})

		Context("when CPU is not 950 model", func() {
			It("should return false for cluster feature support", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
BogoMIPS	: 200.00
Features	: fp asimd evtstrm aes pmull sha1 sha2 crc32 atomics
CPU implementer	: 0x48
CPU architecture: 8
CPU variant	: 0x1
CPU part	: 0xd08
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeFalse())
			})
		})

		Context("checkCPUInfoFileForCluster function", func() {
			It("should correctly detect cluster support via cpuinfo", func() {
				cpuinfoPath := filepath.Join(tempDir, "cpuinfo")
				cpuinfoContent := `processor	: 0
CPU part	: 0xd06
CPU revision	: 0
`
				err := os.WriteFile(cpuinfoPath, []byte(cpuinfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())

				result := checkCPUInfoFileForCluster(cpuinfoPath)
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("ResetSupportsClusterFeatureCache", func() {
		It("should reset the cluster feature cache", func() {
			// Set up a cached value
			result := true
			supportsClusterFeatureCached = &result

			// Reset the cache
			ResetSupportsClusterFeatureCache()

			// Verify cache is nil
			Expect(supportsClusterFeatureCached).To(BeNil())
		})
	})
})
