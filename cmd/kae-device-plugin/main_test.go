/*
Copyright 2026.

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

package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunComponentsReturnsControllerError(t *testing.T) {
	wantErr := errors.New("webhook failed")
	pluginStarted := make(chan struct{})
	releasePlugin := make(chan struct{})
	t.Cleanup(func() { close(releasePlugin) })

	err := runComponents(context.Background(), func() {
		close(pluginStarted)
		<-releasePlugin
	}, func(context.Context) error {
		<-pluginStarted
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("runComponents() error = %v, want %v", err, wantErr)
	}
}

func TestRunComponentsCancelsControllerWhenDevicePluginStops(t *testing.T) {
	controllerStopped := make(chan struct{})

	err := runComponents(context.Background(), func() {}, func(ctx context.Context) error {
		<-ctx.Done()
		close(controllerStopped)
		return nil
	})
	if err != nil {
		t.Fatalf("runComponents() error = %v", err)
	}

	select {
	case <-controllerStopped:
	case <-time.After(time.Second):
		t.Fatal("controller was not canceled after device plugin stopped")
	}
}

func TestRunComponentsReturnsWhenControllerStops(t *testing.T) {
	releasePlugin := make(chan struct{})
	t.Cleanup(func() { close(releasePlugin) })

	err := runComponents(context.Background(), func() {
		<-releasePlugin
	}, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("runComponents() error = %v", err)
	}
}

func TestRunComponentsTreatsSignalCancellationAsCleanStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	releasePlugin := make(chan struct{})
	t.Cleanup(func() { close(releasePlugin) })

	err := runComponents(ctx, func() {
		<-releasePlugin
	}, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("runComponents() error = %v, want clean cancellation", err)
	}
}
