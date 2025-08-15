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
	"maps"
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

// devicePluginServer maintains a gRPC server satisfying
// pluginapi.PluginInterfaceServer interfaces.
// This internal unexposed interface simplifies unit testing.
type devicePluginServer interface {
	Serve(namespace string) error
	Stop() error
	Update(devices map[string]DeviceInfo)
}

// server implements devicePluginServer and pluginapi.PluginInterfaceServer interfaces.
type server struct {
	grpcServer *grpc.Server
	devices    map[string]DeviceInfo
	// updateCh represents the updated full DeviceInfo,
	// for example,it might be [1, 2, 3] before the update, and [1, 2, 3, 5] after the update.
	updatesCh  chan map[string]DeviceInfo
	devType    string
	state      serverState
	stateMutex sync.Mutex
}

func newServer(devType string) devicePluginServer {
	return &server{
		devices:   make(map[string]DeviceInfo),
		updatesCh: make(chan map[string]DeviceInfo, 1),
		state:     uninitialized,
		devType:   devType,
	}
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
	klog.V(4).Info("Started ListAndWatch for", srv.devType)

	if err := srv.sendDevices(stream); err != nil {
		return err
	}

	// Upload the new information to kubelet when the device status is updated.
	for srv.devices = range srv.updatesCh {
		if err := srv.sendDevices(stream); err != nil {
			return err
		}
	}

	return nil
}

func (srv *server) sendDevices(stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	resp := new(pluginapi.ListAndWatchResponse)
	for id, device := range srv.devices {
		resp.Devices = append(resp.Devices, &pluginapi.Device{
			ID:     id,
			Health: device.health,
		})
	}

	klog.V(4).Info("Sending to kubelet", resp.Devices)

	if err := stream.Send(resp); err != nil {
		if stopErr := srv.Stop(); stopErr != nil {
			klog.Errorf("Stop grpc server failed: %v", stopErr)
		}
		return errors.Wrapf(err, "Cannot update device list")
	}

	return nil
}

func (srv *server) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	response := new(pluginapi.AllocateResponse)

	for _, crqt := range rqt.ContainerRequests {
		cresp := new(pluginapi.ContainerAllocateResponse)

		cresp.Envs = map[string]string{}
		cresp.Annotations = map[string]string{}

		for _, id := range crqt.DevicesIDs {
			dev, ok := srv.devices[id]
			if !ok {
				return nil, errors.Errorf("Invalid allocation request with non-existing device %s", id)
			}

			if dev.health != pluginapi.Healthy {
				return nil, errors.Errorf("Invalid allocation request with unhealthy device %s", id)
			}

			for i := range dev.devices {
				cresp.Devices = append(cresp.Devices, &dev.devices[i])
			}

			for i := range dev.mounts {
				cresp.Mounts = append(cresp.Mounts, &dev.mounts[i])
			}

			maps.Copy(cresp.Envs, dev.envs)

			maps.Copy(cresp.Annotations, dev.annotations)
		}

		response.ContainerResponses = append(response.ContainerResponses, cresp)
	}

	return response, nil
}

// Serve starts a gRPC server to serve pluginapi.PluginInterfaceServer interface.
func (srv *server) Serve(namespace string) error {
	return srv.setupAndServe(namespace, pluginapi.DevicePluginPath, pluginapi.KubeletSocket)
}

// Stop stops serving pluginapi.PluginInterfaceServer interface.
func (srv *server) Stop() error {
	if srv.grpcServer == nil {
		return errors.New("Can't stop non-existing gRPC server. Calling Stop() before Serve()?")
	}

	srv.setState(terminating)
	srv.grpcServer.Stop()
	close(srv.updatesCh)

	return nil
}

// Update sends updates from Manager to ListAndWatch's event loop.
func (srv *server) Update(devices map[string]DeviceInfo) {
	srv.updatesCh <- devices
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
