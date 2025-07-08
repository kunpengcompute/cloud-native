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

var _ = Describe("Utils", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "sysfs-test-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("readSysfsEntry", func() {
		It("should read string value correctly", func() {
			testFile := filepath.Join(tempDir, "test_string")
			err := os.WriteFile(testFile, []byte("test_value\n"), 0644)
			Expect(err).NotTo(HaveOccurred())

			var result string
			content, err := readSysfsEntry(tempDir, "test_string", &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("test_value"))
			Expect(content).To(Equal("test_value"))
		})

		It("should read integer value correctly", func() {
			testFile := filepath.Join(tempDir, "test_int")
			err := os.WriteFile(testFile, []byte("123\n"), 0644)
			Expect(err).NotTo(HaveOccurred())

			var result int
			content, err := readSysfsEntry(tempDir, "test_int", &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(123))
			Expect(content).To(Equal("123"))
		})

		It("should read CPUSet correctly", func() {
			testFile := filepath.Join(tempDir, "test_cpuset")
			err := os.WriteFile(testFile, []byte("0-2,4,6-8\n"), 0644)
			Expect(err).NotTo(HaveOccurred())

			result := cpuset.New()
			content, err := readSysfsEntry(tempDir, "test_cpuset", &result, ",")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.String()).To(Equal("0-2,4,6-8"))
			Expect(content).To(Equal("0-2,4,6-8"))
		})

		It("should handle non-existent files", func() {
			var result string
			_, err := readSysfsEntry(tempDir, "non_existent", &result)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid integer format", func() {
			testFile := filepath.Join(tempDir, "invalid_int")
			err := os.WriteFile(testFile, []byte("not_a_number\n"), 0644)
			Expect(err).NotTo(HaveOccurred())

			var result int
			_, err = readSysfsEntry(tempDir, "invalid_int", &result)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("parseValue", func() {
		Context("string parsing", func() {
			It("should parse string value correctly", func() {
				var result string
				err := parseValue("test_string", &result)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal("test_string"))
			})
		})

		Context("integer parsing", func() {
			DescribeTable("should parse valid integer values",
				func(input string, target interface{}, expected interface{}) {
					err := parseValue(input, target)
					Expect(err).NotTo(HaveOccurred())
					Expect(target).To(HaveValue(Equal(expected)))
				},
				Entry("int", "123", new(int), 123),
				Entry("int8 max", "127", new(int8), int8(127)),
				Entry("int8 min", "-128", new(int8), int8(-128)),
				Entry("int16", "32767", new(int16), int16(32767)),
				Entry("int32", "-12345", new(int32), int32(-12345)),
				Entry("int64", "9876543210", new(int64), int64(9876543210)),
				Entry("uint", "123", new(uint), uint(123)),
				Entry("uint8", "255", new(uint8), uint8(255)),
				Entry("uint16", "65535", new(uint16), uint16(65535)),
				Entry("uint32", "4294967295", new(uint32), uint32(4294967295)),
				Entry("uint64", "18446744073709551615", new(uint64), uint64(18446744073709551615)),
			)

			DescribeTable("should handle overflow cases",
				func(input string, target interface{}) {
					err := parseValue(input, target)
					Expect(err).To(HaveOccurred())
				},
				Entry("int8 overflow", "128", new(int8)),
				Entry("int8 underflow", "-129", new(int8)),
				Entry("uint8 overflow", "256", new(uint8)),
				Entry("uint8 negative", "-1", new(uint8)),
				Entry("int16 overflow", "32768", new(int16)),
				Entry("uint16 overflow", "65536", new(uint16)),
				Entry("int32 overflow", "2147483648", new(int32)),
				Entry("uint32 overflow", "4294967296", new(uint32)),
			)

			It("should handle invalid number format", func() {
				var result int
				err := parseValue("not_a_number", &result)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse value"))
			})
		})

		Context("error cases", func() {
			It("should handle nil target", func() {
				err := parseValue("123", nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("target value cannot be nil"))
			})

			It("should handle unsupported type", func() {
				var result float64
				err := parseValue("123.45", &result)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported value type"))
			})
		})
	})

	Context("parseValueList", func() {
		DescribeTable("should parse integer lists correctly",
			func(input string, sep string, target interface{}, expected interface{}) {
				err := parseValueList(input, sep, target)
				Expect(err).NotTo(HaveOccurred())
				Expect(target).To(HaveValue(Equal(expected)))
			},
			Entry("int slice space separated", "1 2 3", " ", &[]int{}, []int{1, 2, 3}),
			Entry("int32 slice comma separated", "1,2,3", ",", &[]int32{}, []int32{1, 2, 3}),
			Entry("int64 slice semicolon separated", "1;2;3", ";", &[]int64{}, []int64{1, 2, 3}),
		)

		It("should parse CPUSet correctly", func() {
			testCases := []struct {
				input    string
				expected string
			}{
				{"0-2,4,6-8", "0-2,4,6-8"},
				{"0-3", "0-3"},
				{"1,3,5,7", "1,3,5,7"},
			}

			for _, tc := range testCases {
				result := cpuset.New()
				err := parseValueList(tc.input, ",", &result)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.String()).To(Equal(tc.expected))
			}
		})

		It("should handle empty input", func() {
			var result []int
			err := parseValueList("", " ", &result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should handle invalid input type", func() {
			var result struct{}
			err := parseValueList("1,2,3", ",", &result)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported slice type"))
		})

		It("should handle invalid number format", func() {
			var result []int
			err := parseValueList("1,abc,3", ",", &result)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse number"))
		})
	})
})
