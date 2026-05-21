package system

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/cpuset"
)

var _ = Describe("Node", func() {
	var (
		tempDir  string
		testNode *node
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "node-test-")
		Expect(err).NotTo(HaveOccurred())

		// Create a test node
		testNode = &node{
			path:       tempDir,
			id:         0,
			pkg:        1,
			die:        2,
			cpus:       MustParse("0-3"),
			distance:   []int{10, 20, 30},
			hasMemory:  true,
			normalMem:  true,
			memoryType: MemoryTypeDRAM,
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("Basic node methods", func() {
		It("should return correct node properties", func() {
			Expect(testNode.ID()).To(Equal(ID(0)))
			Expect(testNode.PackageID()).To(Equal(ID(1)))
			Expect(testNode.DieID()).To(Equal(ID(2)))
			Expect(testNode.CPUSet().String()).To(Equal("0-3"))
			Expect(testNode.Distance()).To(Equal([]int{10, 20, 30}))
			Expect(testNode.DistanceFrom(0)).To(Equal(10))
			Expect(testNode.DistanceFrom(1)).To(Equal(20))
			Expect(testNode.DistanceFrom(2)).To(Equal(30))
			Expect(testNode.DistanceFrom(3)).To(Equal(-1)) // Out of range
			Expect(testNode.HasMemory()).To(BeTrue())
			Expect(testNode.HasNormalMemory()).To(BeTrue())
			Expect(testNode.GetMemoryType()).To(Equal(MemoryTypeDRAM))
		})

		It("should set node properties correctly", func() {
			testNode.SetPackageID(10)
			Expect(testNode.PackageID()).To(Equal(ID(10)))

			testNode.SetDieID(20)
			Expect(testNode.DieID()).To(Equal(ID(20)))

			testNode.SetMemory(false)
			Expect(testNode.HasMemory()).To(BeFalse())

			testNode.SetNormalMemory(false)
			Expect(testNode.HasNormalMemory()).To(BeFalse())

			testNode.SetMemoryType(MemoryTypeDRAM)
			Expect(testNode.GetMemoryType()).To(Equal(MemoryTypeDRAM))
		})
	})

	Describe("MemoryInfo", func() {
		Context("with valid meminfo file with kB units", func() {
			BeforeEach(func() {
				meminfoContent := `Node 0 MemTotal:       16777216 kB
Node 0 MemFree:        8388608 kB
Node 0 MemUsed:        8388608 kB
Node 0 Active:          524288 kB
Node 0 Inactive:        262144 kB
Node 0 Active(anon):    131072 kB
Node 0 Inactive(anon):   65536 kB
Node 0 Active(file):    393216 kB
Node 0 Inactive(file):  196608 kB
Node 0 Unevictable:          0 kB
Node 0 Mlocked:              0 kB
`
				err := os.WriteFile(filepath.Join(tempDir, "meminfo"), []byte(meminfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse memory information correctly in KB", func() {
				memInfo, err := testNode.MemoryInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(memInfo).NotTo(BeNil())

				// Values should remain in KB
				Expect(memInfo.MemTotal).To(Equal(uint64(16777216)))
				Expect(memInfo.MemFree).To(Equal(uint64(8388608)))
				Expect(memInfo.MemUsed).To(Equal(uint64(8388608)))
			})
		})

		Context("with meminfo file containing different units", func() {
			BeforeEach(func() {
				meminfoContent := `Node 0 MemTotal:       16384 MB
Node 0 MemFree:        8 GB
Node 0 MemUsed:        8388608 kB
`
				err := os.WriteFile(filepath.Join(tempDir, "meminfo"), []byte(meminfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert all units to KB", func() {
				memInfo, err := testNode.MemoryInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(memInfo).NotTo(BeNil())

				// MemTotal: MB -> KB (×1024)
				Expect(memInfo.MemTotal).To(Equal(uint64(16384 * 1024)))

				// MemFree: GB -> KB (×1024×1024)
				Expect(memInfo.MemFree).To(Equal(uint64(8 * 1024 * 1024)))

				// MemUsed: kB -> KB (no conversion)
				Expect(memInfo.MemUsed).To(Equal(uint64(8388608)))
			})
		})

		Context("with meminfo file containing bytes unit", func() {
			BeforeEach(func() {
				meminfoContent := `Node 0 MemTotal:       16777216 B
Node 0 MemFree:        8388608 B
Node 0 MemUsed:        8388608 B
`
				err := os.WriteFile(filepath.Join(tempDir, "meminfo"), []byte(meminfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert bytes to KB", func() {
				memInfo, err := testNode.MemoryInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(memInfo).NotTo(BeNil())

				// B -> KB (÷1024)
				Expect(memInfo.MemTotal).To(Equal(uint64(16777216 / 1024)))
				Expect(memInfo.MemFree).To(Equal(uint64(8388608 / 1024)))
				Expect(memInfo.MemUsed).To(Equal(uint64(8388608 / 1024)))
			})
		})

		Context("with meminfo file containing no units", func() {
			BeforeEach(func() {
				meminfoContent := `Node 0 MemTotal:       16777216
Node 0 MemFree:        8388608
Node 0 MemUsed:        8388608
`
				err := os.WriteFile(filepath.Join(tempDir, "meminfo"), []byte(meminfoContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should assume values are already in KB", func() {
				memInfo, err := testNode.MemoryInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(memInfo).NotTo(BeNil())

				// Values without units are assumed to be in KB
				Expect(memInfo.MemTotal).To(Equal(uint64(16777216)))
				Expect(memInfo.MemFree).To(Equal(uint64(8388608)))
				Expect(memInfo.MemUsed).To(Equal(uint64(8388608)))
			})
		})

		Context("with invalid meminfo file format", func() {
			BeforeEach(func() {
				invalidContent := `Invalid format
No proper key-value pairs
`
				err := os.WriteFile(filepath.Join(tempDir, "meminfo"), []byte(invalidContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty memory info without error", func() {
				memInfo, err := testNode.MemoryInfo()
				Expect(err).NotTo(HaveOccurred())
				Expect(memInfo).NotTo(BeNil())
				Expect(memInfo.MemTotal).To(Equal(uint64(0)))
				Expect(memInfo.MemFree).To(Equal(uint64(0)))
				Expect(memInfo.MemUsed).To(Equal(uint64(0)))
			})
		})

		Context("with missing meminfo file", func() {
			It("should return an error", func() {
				// Ensure the meminfo file doesn't exist
				_ = os.Remove(filepath.Join(tempDir, "meminfo"))

				memInfo, err := testNode.MemoryInfo()
				Expect(err).To(HaveOccurred())
				Expect(memInfo).To(BeNil())
			})
		})
	})
})

// Helper function to create CPUSet from string
func MustParse(s string) cpuset.CPUSet {
	result, err := cpuset.Parse(s)
	if err != nil {
		panic(err)
	}
	return result
}
