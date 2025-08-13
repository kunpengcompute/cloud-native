// Copyright 2018 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package deviceplugin

import (
	"context"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type serverState int

// Server state.
const (
	uninitialized serverState = iota
	serving
	terminating
)

type server struct {
	grpcServer *grpc.Server
	devType    string
	state      serverState
	stateMutex sync.Mutex
}

func (srv *server) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (srv *server) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (srv *server) GetPreferredAllocation(ctx context.Context, rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}

func (srv *server) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	return nil
}

func (srv *server) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return &pluginapi.AllocateResponse{}, nil
}

func (srv *server) Serve(namespace string) error {
	return srv.setupAndServe(namespace, pluginapi.DevicePluginPath, pluginapi.KubeletSocket)
}

func (srv *server) getState() serverState {
	srv.stateMutex.Lock()
	defer srv.stateMutex.Unlock()

	return srv.state
}

func (srv *server) setState(state serverState) {
	srv.stateMutex.Lock()
	defer srv.stateMutex.Unlock()

	srv.state = state
}

// setupAndServe binds given gRPC server to device manager, starts it and registers it with kubelet.
func (srv *server) setupAndServe(namespace string, devicePluginPath string, kubeletSocket string) error {
	resourceName := namespace + "/" + srv.devType
	pluginPrefix := namespace + "-" + srv.devType
	srv.setState(serving)

	for srv.getState() == serving {
		// pluginEndpoint is a sock file, name like kae.kunpeng.com-hpre.sock
		pluginEndpoint := pluginPrefix + ".sock"
		pluginSocket := path.Join(devicePluginPath, pluginEndpoint)

		if err := waitForServer(pluginSocket, time.Second); err == nil {
			return errors.Errorf("Socket %s is already in use", pluginSocket)
		}

		err := os.Remove(pluginSocket)
		if err != nil {
			klog.Errorf("Remove pluginSocket %s failed: %v", pluginSocket, err)
		}

		lis, err := net.Listen("unix", pluginSocket)
		if err != nil {
			return errors.Wrap(err, "Failed to listen to plugin socket")
		}

		srv.grpcServer = grpc.NewServer()
		pluginapi.RegisterDevicePluginServer(srv.grpcServer, srv)

		// Starts device plugin service.
		go func() {
			klog.V(1).Infof("Start server for %s at: %s", srv.devType, pluginSocket)

			if serveErr := srv.grpcServer.Serve(lis); serveErr != nil {
				klog.Errorf("unable to start gRPC server: %+v", serveErr)
			}
		}()

		// Wait for the server to start
		if err = waitForServer(pluginSocket, 10*time.Second); err != nil {
			return err
		}

		// Register with Kubelet.
		err = srv.registerWithKubelet(kubeletSocket, pluginEndpoint, resourceName)
		if err != nil {
			return err
		}

		klog.V(1).Infof("Device plugin for %s registered", srv.devType)

		// Kubelet removes plugin socket when it (re)starts
		// plugin must restart in this case
		// watch plugin socket or kubelet socket?
		if err = watchFile(pluginSocket); err != nil {
			return err
		}

		if srv.getState() == serving {
			srv.grpcServer.Stop()
			klog.V(1).Infof("Socket %s removed, restarting", pluginSocket)
		} else {
			klog.V(1).Infof("Socket %s shut down", pluginSocket)
		}
	}

	return nil
}

// waitForServer checks if grpc server is alive
// by making grpc blocking connection to the server socket.
func waitForServer(socket string, timeout time.Duration) error {
	conn, err := grpc.NewClient(filepath.Join("unix://", socket), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "Cannot create a gRPC client")
	}

	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()

	// A blocking dial blocks until the clientConn is ready. Based
	// on grpc-go's DialContext() that moved to use NewClient() but
	// marked DialContext() deprecated.
	for {
		state := conn.GetState()
		if state == connectivity.Idle {
			conn.Connect()
		}

		if state == connectivity.Ready {
			return nil
		}

		if !conn.WaitForStateChange(ctx, state) {
			// ctx got timeout or canceled.
			return errors.Wrapf(ctx.Err(), "Failed dial context at %s", socket)
		}
	}
}

func (srv *server) registerWithKubelet(kubeletSocket, pluginEndPoint, resourceName string) error {
	conn, err := grpc.NewClient(filepath.Join("unix://", kubeletSocket), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "Cannot create a gRPC client")
	}

	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndPoint,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return errors.Wrap(err, "Cannot register to kubelet service")
	}

	return nil
}

func watchFile(file string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrapf(err, "Failed to create watcher for %s", file)
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Dir(file))
	if err != nil {
		return errors.Wrapf(err, "Failed to add %s to watcher", file)
	}

	for {
		select {
		case ev := <-watcher.Events:
			if (ev.Op == fsnotify.Remove || ev.Op == fsnotify.Rename) && ev.Name == file {
				return nil
			}
		case err := <-watcher.Errors:
			return errors.WithStack(err)
		}
	}
}
