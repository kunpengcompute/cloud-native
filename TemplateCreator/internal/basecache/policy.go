// Package basecache 实现 TemplateCreator 单机持久化 base-layer cache 的稳定适配层。
package basecache

import (
	"fmt"

	"github.com/distribution/reference"
)

// ImageEligible 判断 ImageRef 是否允许读写共享 cache。
// 该函数只解析引用语法，不访问 registry，也不会把 mutable tag 自动解析为 digest。
func ImageEligible(imageRef string, allowMutableImageTags bool) (bool, error) {
	named, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return false, fmt.Errorf("parse OCI image reference: %w", err)
	}
	if _, pinned := named.(reference.Digested); pinned {
		return true, nil
	}

	return allowMutableImageTags, nil
}
