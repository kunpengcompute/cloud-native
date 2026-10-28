// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package sandbox

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/clickhouse/pkg/hoststats"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/fc"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// initializeHostStatsCollector initializes the host stats collector for a sandbox.
// This is a best-effort operation - errors are logged but do not fail the sandbox initialization.
func initializeHostStatsCollector(
	ctx context.Context,
	sbx *Sandbox,
	fcHandle *fc.Process,
	buildID string,
	runtime RuntimeMetadata,
	config Config,
	hostStatsDelivery hoststats.Delivery,
	samplingInterval time.Duration,
) {
	if hostStatsDelivery == nil {
		return
	}

	firecrackerPID, err := fcHandle.Pid()
	if err != nil {
		logger.L().Error(ctx, "failed to get firecracker PID for host stats",
			logger.WithSandboxID(runtime.SandboxID),
			zap.Error(err))

		return
	}

	teamID, err := uuid.Parse(runtime.TeamID)
	if err != nil {
		logger.L().Error(ctx, "error parsing team ID", logger.WithTeamID(runtime.TeamID), zap.Error(err))
	}

	var cgroupStats CgroupStatsFunc
	if sbx.cgroupHandle != nil {
		cgroupStats = sbx.cgroupHandle.GetStats
	}

	collector, err := NewHostStatsCollector(
		HostStatsMetadata{
			SandboxID:   runtime.SandboxID,
			ExecutionID: runtime.ExecutionID,
			TemplateID:  runtime.TemplateID,
			BuildID:     buildID,
			TeamID:      teamID,
			VCPUCount:   config.Vcpu,
			MemoryMB:    config.RamMB,
		},
		int32(firecrackerPID),
		hostStatsDelivery,
		samplingInterval,
		cgroupStats,
	)
	if err != nil {
		logger.L().Error(ctx, "failed to create host stats collector",
			logger.WithSandboxID(runtime.SandboxID),
			zap.Error(err))

		return
	}

	sbx.hostStatsCollector = collector

	go collector.Start(ctx)
}
