package util

import "strconv"

func ParseInt64(str string) (int64, error) {
	const (
		base    = 10
		bitSize = 64
	)
	return strconv.ParseInt(str, base, bitSize)
}
