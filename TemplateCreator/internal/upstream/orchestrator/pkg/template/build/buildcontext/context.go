// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package buildcontext

import (
	"golang.org/x/sync/errgroup"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/cfg"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type BuildContext struct {
	BuilderConfig  cfg.BuilderConfig
	Config         config.TemplateConfig
	Template       storage.TemplateFiles
	UploadErrGroup *errgroup.Group
	EnvdVersion    string
	CacheScope     string
	IsV1Build      bool
	Version        string
}
