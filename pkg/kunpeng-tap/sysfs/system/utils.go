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
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
)

const unsupportedValueTypeError = "unsupported value type: %T"

// Get the trailing enumeration part of a name.
func getEnumeratedID(name string) ID {
	id := 0
	base := 1
	for idx := len(name) - 1; idx > 0; idx-- {
		d := name[idx]

		if '0' <= d && d <= '9' {
			id += base * (int(d) - '0')
			base *= 10
		} else {
			if base > 1 {
				return ID(id)
			}

			return ID(-1)
		}
	}

	return ID(-1)
}

// Read content of a sysfs entry and convert it according to the type of a given pointer.
func readSysfsEntry(dir, file string, target interface{}, seps ...string) (string, error) {
	var buf string

	path := filepath.Join(dir, file)

	blob, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	buf = strings.Trim(string(blob), "\n")

	if target == interface{}(nil) {
		return buf, nil
	}

	switch target.(type) {
	case *string, *int, *uint, *int8, *uint8, *int16, *uint16, *int32, *uint32, *int64, *uint64:
		err := parseValue(buf, target)
		if err != nil {
			klog.ErrorS(err, "Failed to parse value", "path", path)
			return "", err
		}
		return buf, nil

	case *cpuset.CPUSet, *[]int, *[]uint, *[]int8, *[]uint8, *[]int16, *[]uint16, *[]int32, *[]uint32, *[]int64, *[]uint64:
		sep, err := getSeparator(" ", seps)
		if err != nil {
			return "", err
		}

		err = parseValueList(buf, sep, target)
		if err != nil {
			klog.ErrorS(err, "Failed to parse value list", "path", path)
			return "", err
		}
		return buf, nil
	}

	return "", fmt.Errorf("unsupported sysfs entry type %T", target)
}

// Determine list separator string, given an optional separator variadic argument.
func getSeparator(defaultVal string, args []string) (string, error) {
	switch len(args) {
	case 0:
		return defaultVal, nil
	case 1:
		return args[0], nil
	}

	return "", fmt.Errorf("invalid separator (%v), 1 expected, %d given", args, len(args))
}

// Helper function to parse signed integer types
func parseSignedInt(str string, bitSize int, target interface{}) error {
	v, err := strconv.ParseInt(str, 0, bitSize)
	if err != nil {
		return fmt.Errorf("failed to parse value of int%d %q: %w", bitSize, str, err)
	}

	switch t := target.(type) {
	case *int:
		*t = int(v)
	case *int8:
		*t = int8(v)
	case *int16:
		*t = int16(v)
	case *int32:
		*t = int32(v)
	case *int64:
		*t = v
	default:
		return fmt.Errorf(unsupportedValueTypeError, target)
	}
	return nil
}

// Helper function to parse unsigned integer types
func parseUnsignedInt(str string, bitSize int, target interface{}) error {
	v, err := strconv.ParseUint(str, 0, bitSize)
	if err != nil {
		return fmt.Errorf("failed to parse value of uint%d %q: %w", bitSize, str, err)
	}

	switch t := target.(type) {
	case *uint:
		*t = uint(v)
	case *uint8:
		*t = uint8(v)
	case *uint16:
		*t = uint16(v)
	case *uint32:
		*t = uint32(v)
	case *uint64:
		*t = v
	default:
		return fmt.Errorf(unsupportedValueTypeError, target)
	}
	return nil
}

// parseValue parses a string value into the specified type.
func parseValue(str string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("target value cannot be nil")
	}

	switch v := value.(type) {
	case *string:
		*v = str
		return nil
	case *int:
		return parseSignedInt(str, strconv.IntSize, value)
	case *int8:
		return parseSignedInt(str, 8, value)
	case *int16:
		return parseSignedInt(str, 16, value)
	case *int32:
		return parseSignedInt(str, 32, value)
	case *int64:
		return parseSignedInt(str, 64, value)
	case *uint:
		return parseUnsignedInt(str, strconv.IntSize, value)
	case *uint8:
		return parseUnsignedInt(str, 8, value)
	case *uint16:
		return parseUnsignedInt(str, 16, value)
	case *uint32:
		return parseUnsignedInt(str, 32, value)
	case *uint64:
		return parseUnsignedInt(str, 64, value)
	default:
		return fmt.Errorf(unsupportedValueTypeError, value)
	}
}

// Helper function to parse CPUSet from string
func parseCPUSet(str, sep string, cpuSet *cpuset.CPUSet) error {
	if sep != "," {
		return fmt.Errorf("invalid separator for CPUSet: %q", sep)
	}
	var err error
	if *cpuSet, err = cpuset.Parse(str); err != nil {
		klog.ErrorS(err, "Failed to parse CPUSet", "str", str)
		return err
	}
	klog.V(4).InfoS("parseValueList", "sep", sep, "cpuset", cpuSet.String())
	return nil
}

// Helper function to validate unsigned integer types
func validateUnsignedType(v int64, targetType string, maxValue int64) error {
	if v < 0 {
		return fmt.Errorf("negative value %d invalid for %s", v, targetType)
	}
	if maxValue > 0 && v > maxValue {
		return fmt.Errorf("value %d out of range for %s", v, targetType)
	}
	return nil
}

// Helper function to validate signed integer types
func validateSignedType(v int64, targetType string, minValue, maxValue int64) error {
	if v > maxValue || v < minValue {
		return fmt.Errorf("value %d out of range for %s", v, targetType)
	}
	return nil
}

// Helper function to validate and convert value for specific type
func validateAndConvertValue(v int64, targetType string) error {
	switch targetType {
	case "uint", "uint64":
		return validateUnsignedType(v, targetType, 0)
	case "uint8":
		return validateUnsignedType(v, targetType, math.MaxUint8)
	case "uint16":
		return validateUnsignedType(v, targetType, math.MaxUint16)
	case "uint32":
		return validateUnsignedType(v, targetType, math.MaxUint32)
	case "int8":
		return validateSignedType(v, targetType, math.MinInt8, math.MaxInt8)
	case "int16":
		return validateSignedType(v, targetType, math.MinInt16, math.MaxInt16)
	case "int32":
		return validateSignedType(v, targetType, math.MinInt32, math.MaxInt32)
	default:
		return fmt.Errorf(unsupportedValueTypeError, targetType)
	}
}

// Helper function to validate and append value to slice
func validateAndAppend(v int64, targetType string, appendFunc func()) error {
	if err := validateAndConvertValue(v, targetType); err != nil {
		return err
	}
	appendFunc()
	return nil
}

// Helper function to append value to appropriate slice type
func appendToSlice(valuep interface{}, v int64) error {
	switch ptr := valuep.(type) {
	case *[]int:
		*ptr = append(*ptr, int(v))
	case *[]int64:
		*ptr = append(*ptr, v)
	case *[]uint:
		return validateAndAppend(v, "uint", func() { *ptr = append(*ptr, uint(v)) })
	case *[]uint64:
		return validateAndAppend(v, "uint64", func() { *ptr = append(*ptr, uint64(v)) })
	case *[]int8:
		return validateAndAppend(v, "int8", func() { *ptr = append(*ptr, int8(v)) })
	case *[]int16:
		return validateAndAppend(v, "int16", func() { *ptr = append(*ptr, int16(v)) })
	case *[]int32:
		return validateAndAppend(v, "int32", func() { *ptr = append(*ptr, int32(v)) })
	case *[]uint8:
		return validateAndAppend(v, "uint8", func() { *ptr = append(*ptr, uint8(v)) })
	case *[]uint16:
		return validateAndAppend(v, "uint16", func() { *ptr = append(*ptr, uint16(v)) })
	case *[]uint32:
		return validateAndAppend(v, "uint32", func() { *ptr = append(*ptr, uint32(v)) })
	default:
		return fmt.Errorf("unsupported slice type: %T", valuep)
	}
	return nil
}

// parseValueList parses a list of values from a string into a slice or CPUSet.
func parseValueList(str, sep string, valuep interface{}) error {
	if str == "" {
		return nil
	}

	// Handle CPUSet type
	if cpuSet, ok := valuep.(*cpuset.CPUSet); ok {
		return parseCPUSet(str, sep, cpuSet)
	}

	// Handle numeric slice types
	items := strings.Split(str, sep)
	for _, s := range items {
		if s == "" {
			continue
		}

		v, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return fmt.Errorf("failed to parse number %q: %w", s, err)
		}

		if err := appendToSlice(valuep, v); err != nil {
			return err
		}
	}
	return nil
}
