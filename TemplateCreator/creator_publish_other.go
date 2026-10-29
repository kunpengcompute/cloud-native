//go:build !linux

package templatecreator

import (
	"context"
	"errors"
)

// CreateTemplate 在非 Linux 平台稳定返回不支持错误，不会尝试创建任何 runtime 资源。
func (*Creator) CreateTemplate(context.Context, CreateRequest) (CreateResult, error) {
	return CreateResult{}, errors.New("TemplateCreator builds require Linux")
}
