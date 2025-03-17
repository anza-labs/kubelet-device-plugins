// Copyright 2025 anza-labs contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tapdeviceplugin

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"path"

	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type Server struct {
	log       *slog.Logger
	namespace string
	name      string
	update    chan struct{}
	devices   uint
	devs      []*v1beta1.Device
}

var _ v1beta1.DevicePluginServer = (*Server)(nil)

func New(namespace, tap string, devices uint, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	s := &Server{
		log:       log.With(slog.String("device_name", tap)),
		name:      tap,
		namespace: namespace,
		devices:   devices,
		update:    make(chan struct{}),
		devs:      []*v1beta1.Device{},
	}

	return s, s.Discover()
}

func (s *Server) Update() {
	s.update <- struct{}{}
}

func (s *Server) Discover() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to discover interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Name == s.name {
			s.log.Debug("Discovered TAP device")

			for i := uint(0); i < s.devices; i++ {
				s.devs = append(s.devs, &v1beta1.Device{
					ID:     fmt.Sprintf("%sd%d", s.name, i),
					Health: v1beta1.Healthy,
				})
			}
			return nil
		}
	}

	s.log.Error("No TAP device found")
	return nil
}

func (s *Server) Name() string {
	return path.Join(s.namespace, s.name)
}

func (s *Server) Socket() string {
	return fmt.Sprintf("unix://%s", path.Join(v1beta1.DevicePluginPath, s.name+".sock"))
}

func (s *Server) GetDevicePluginOptions(
	ctx context.Context,
	_ *v1beta1.Empty,
) (*v1beta1.DevicePluginOptions, error) {
	return &v1beta1.DevicePluginOptions{
		PreStartRequired:                false,
		GetPreferredAllocationAvailable: false,
	}, nil
}

func (s *Server) ListAndWatch(
	_ *v1beta1.Empty,
	lws v1beta1.DevicePlugin_ListAndWatchServer,
) error {
	if err := lws.Send(&v1beta1.ListAndWatchResponse{Devices: s.devs}); err != nil {
		s.log.Error("Failed to send ListAndWatch response", "error", err)
	}

	for range s.update {
		if err := lws.Send(&v1beta1.ListAndWatchResponse{Devices: s.devs}); err != nil {
			s.log.Error("Failed to send ListAndWatch response", "error", err)
		}
	}

	panic("unexpected error")
}

func (s *Server) Allocate(
	ctx context.Context,
	req *v1beta1.AllocateRequest,
) (*v1beta1.AllocateResponse, error) {
	s.update <- struct{}{}

	envs := map[string]string{
		"TAP_DEVICE_NAME": s.name,
	}

	return &v1beta1.AllocateResponse{
		ContainerResponses: []*v1beta1.ContainerAllocateResponse{{Envs: envs}},
	}, nil
}

func (s *Server) GetPreferredAllocation(
	ctx context.Context,
	req *v1beta1.PreferredAllocationRequest,
) (*v1beta1.PreferredAllocationResponse, error) {
	return &v1beta1.PreferredAllocationResponse{}, nil
}

func (s *Server) PreStartContainer(
	ctx context.Context,
	req *v1beta1.PreStartContainerRequest,
) (*v1beta1.PreStartContainerResponse, error) {
	return &v1beta1.PreStartContainerResponse{}, nil
}
