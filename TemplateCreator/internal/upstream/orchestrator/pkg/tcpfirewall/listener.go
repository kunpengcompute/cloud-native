// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package tcpfirewall

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"go.uber.org/zap"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

const (
	acceptRetryDelay = 100 * time.Millisecond
)

// resilientListener wraps a net.Listener to handle transient Accept errors.
type resilientListener struct {
	net.Listener

	ctx    context.Context //nolint:containedctx // needed for cancellation in Accept retry loop
	logger logger.Logger
}

func (l *resilientListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err == nil {
			return conn, nil
		}

		if !isTransientAcceptError(err) {
			return nil, err
		}

		l.logger.Error(l.ctx, "tcpfirewall listener accept error, retrying", zap.Error(err))

		select {
		case <-l.ctx.Done():
			return nil, l.ctx.Err()
		case <-time.After(acceptRetryDelay):
		}
	}
}

func isTransientAcceptError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, syscall.EMFILE) || // too many open files (per-process)
		errors.Is(err, syscall.ENFILE) || // too many open files (system-wide)
		errors.Is(err, syscall.EAGAIN) || // resource temporarily unavailable
		errors.Is(err, syscall.ECONNABORTED) // client closed before accept
}
