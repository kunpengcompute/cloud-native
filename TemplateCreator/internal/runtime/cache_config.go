package runtime

import "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/basecache"

// BaseCacheConfig 是公共 CreatorConfig 到 Linux runtime 的私有配置副本。
// runtime 只消费归一化后的稳定值，不依赖 templatecreator 根 package，避免循环依赖。
type BaseCacheConfig struct {
	Enabled               bool
	Root                  string
	AllowMutableImageTags bool
	ABI                   basecache.RuntimeABI
}
