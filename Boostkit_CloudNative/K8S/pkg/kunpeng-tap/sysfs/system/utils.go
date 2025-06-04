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

// parseValue parses a string value into the specified type.
func parseValue(str string, value interface{}) error {
	if value == nil {
		return fmt.Errorf("target value cannot be nil")
	}

	switch value.(type) {
	case *string:
		*value.(*string) = str
		return nil

	case *int:
		v, err := strconv.ParseInt(str, 0, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("failed to parse int value %q: %w", str, err)
		}
		*value.(*int) = int(v)
		return nil

	case *int8:
		v, err := strconv.ParseInt(str, 0, 8)
		if err != nil {
			return fmt.Errorf("failed to parse int8 value %q: %w", str, err)
		}
		*value.(*int8) = int8(v)
		return nil

	case *int16:
		v, err := strconv.ParseInt(str, 0, 16)
		if err != nil {
			return fmt.Errorf("failed to parse int16 value %q: %w", str, err)
		}
		*value.(*int16) = int16(v)
		return nil

	case *int32:
		v, err := strconv.ParseInt(str, 0, 32)
		if err != nil {
			return fmt.Errorf("failed to parse int32 value %q: %w", str, err)
		}
		*value.(*int32) = int32(v)
		return nil

	case *int64:
		v, err := strconv.ParseInt(str, 0, 64)
		if err != nil {
			return fmt.Errorf("failed to parse int64 value %q: %w", str, err)
		}
		*value.(*int64) = v
		return nil

	case *uint:
		v, err := strconv.ParseUint(str, 0, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %q: %w", str, err)
		}
		*value.(*uint) = uint(v)
		return nil

	case *uint8:
		v, err := strconv.ParseUint(str, 0, 8)
		if err != nil {
			return fmt.Errorf("failed to parse uint8 value %q: %w", str, err)
		}
		*value.(*uint8) = uint8(v)
		return nil

	case *uint16:
		v, err := strconv.ParseUint(str, 0, 16)
		if err != nil {
			return fmt.Errorf("failed to parse uint16 value %q: %w", str, err)
		}
		*value.(*uint16) = uint16(v)
		return nil

	case *uint32:
		v, err := strconv.ParseUint(str, 0, 32)
		if err != nil {
			return fmt.Errorf("failed to parse uint32 value %q: %w", str, err)
		}
		*value.(*uint32) = uint32(v)
		return nil

	case *uint64:
		v, err := strconv.ParseUint(str, 0, 64)
		if err != nil {
			return fmt.Errorf("failed to parse uint64 value %q: %w", str, err)
		}
		*value.(*uint64) = v
		return nil

	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
}

// parseValueList parses a list of values from a string into a slice or CPUSet.
func parseValueList(str, sep string, valuep interface{}) error {
	if str == "" {
		return nil
	}

	items := strings.Split(str, sep)

	// 处理 CPUSet 类型
	if cpuSet, ok := valuep.(*cpuset.CPUSet); ok {
		if sep != "," {
			return fmt.Errorf("invalid separator for CPUSet: %q", sep)
		}
		var err error
		if *cpuSet, err = cpuset.Parse(str); err != nil {
			klog.ErrorS(err, "Failed to parse CPUSet", "str", str)
			return err
		}

		klog.V(4).InfoS("parseValueList", "sep", sep, "items", items, "cpuset", valuep.(*cpuset.CPUSet).String())
		return nil
	}

	// 处理数值类型切片
	for _, s := range items {
		if s == "" {
			continue
		}

		v, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return fmt.Errorf("failed to parse number %q: %w", s, err)
		}

		switch ptr := valuep.(type) {
		case *[]int:
			*ptr = append(*ptr, int(v))
		case *[]uint:
			if v < 0 {
				return fmt.Errorf("negative value %d invalid for uint", v)
			}
			*ptr = append(*ptr, uint(v))
		case *[]int8:
			if v > math.MaxInt8 || v < math.MinInt8 {
				return fmt.Errorf("value %d out of range for int8", v)
			}
			*ptr = append(*ptr, int8(v))
		case *[]uint8:
			if v > math.MaxUint8 || v < 0 {
				return fmt.Errorf("value %d out of range for uint8", v)
			}
			*ptr = append(*ptr, uint8(v))
		case *[]int16:
			if v > math.MaxInt16 || v < math.MinInt16 {
				return fmt.Errorf("value %d out of range for int16", v)
			}
			*ptr = append(*ptr, int16(v))
		case *[]uint16:
			if v > math.MaxUint16 || v < 0 {
				return fmt.Errorf("value %d out of range for uint16", v)
			}
			*ptr = append(*ptr, uint16(v))
		case *[]int32:
			if v > math.MaxInt32 || v < math.MinInt32 {
				return fmt.Errorf("value %d out of range for int32", v)
			}
			*ptr = append(*ptr, int32(v))
		case *[]uint32:
			if v > math.MaxUint32 || v < 0 {
				return fmt.Errorf("value %d out of range for uint32", v)
			}
			*ptr = append(*ptr, uint32(v))
		case *[]int64:
			*ptr = append(*ptr, v)
		case *[]uint64:
			if v < 0 {
				return fmt.Errorf("negative value %d invalid for uint64", v)
			}
			*ptr = append(*ptr, uint64(v))
		default:
			return fmt.Errorf("unsupported slice type: %T", valuep)
		}
	}
	return nil
}
