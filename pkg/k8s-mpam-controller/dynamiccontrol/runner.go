/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamiccontrol

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

const (
	defaultPublishInterval = 3 * time.Second
	defaultApplyInterval   = 5 * time.Second
	defaultTaskTimeout     = 2 * time.Second
)

// Runner executes dynamic-control periodic tasks under controller-runtime manager.
// It should be added via mgr.Add(runner).
type Runner struct {
	Coordinator *Coordinator

	PublishInterval time.Duration
	ApplyInterval   time.Duration
	TaskTimeout     time.Duration
}

// NewRunner creates a periodic runner with defaults.
func NewRunner(c *Coordinator) *Runner {
	return &Runner{
		Coordinator:     c,
		PublishInterval: defaultPublishInterval,
		ApplyInterval:   defaultApplyInterval,
		TaskTimeout:     defaultTaskTimeout,
	}
}

func (r *Runner) setDefaults() {
	if r.PublishInterval <= 0 {
		r.PublishInterval = defaultPublishInterval
	}
	if r.ApplyInterval <= 0 {
		r.ApplyInterval = defaultApplyInterval
	}
	if r.TaskTimeout <= 0 {
		r.TaskTimeout = defaultTaskTimeout
	}
}

func (r *Runner) validate() error {
	if r.Coordinator == nil {
		return fmt.Errorf("coordinator must not be nil")
	}
	if r.PublishInterval <= 0 {
		return fmt.Errorf("publish interval must be > 0")
	}
	if r.ApplyInterval <= 0 {
		return fmt.Errorf("apply interval must be > 0")
	}
	if r.TaskTimeout <= 0 {
		return fmt.Errorf("task timeout must be > 0")
	}
	return nil
}

// Start runs periodic loops until manager context is canceled.
func (r *Runner) Start(ctx context.Context) error {
	r.setDefaults()
	if err := r.validate(); err != nil {
		return err
	}

	klog.Infof(
		"starting dynamic-control runner: publishInterval=%s applyInterval=%s timeout=%s",
		r.PublishInterval, r.ApplyInterval, r.TaskTimeout,
	)

	go r.runLoop(ctx, "publish-online-pods", r.PublishInterval, r.Coordinator.PublishOnlinePodsOnce)
	go r.runLoop(ctx, "apply-interference", r.ApplyInterval, r.Coordinator.ApplyInterferenceOnce)

	<-ctx.Done()
	klog.Info("dynamic-control runner stopped")
	return nil
}

func (r *Runner) runLoop(
	ctx context.Context,
	name string,
	interval time.Duration,
	task func(context.Context) error,
) {
	r.runWithTimeout(ctx, name, task)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runWithTimeout(ctx, name, task)
		}
	}
}

func (r *Runner) runWithTimeout(
	ctx context.Context,
	name string,
	task func(context.Context) error,
) {
	taskCtx, cancel := context.WithTimeout(ctx, r.TaskTimeout)
	defer cancel()

	if err := task(taskCtx); err != nil {
		klog.Warningf("dynamic-control task %s failed: %v", name, err)
	}
}
