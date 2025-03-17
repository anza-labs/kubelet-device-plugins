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

package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/anza-labs/kubelet-device-plugins/pkg/discovery"
	"github.com/anza-labs/kubelet-device-plugins/pkg/metrics"
	"github.com/anza-labs/kubelet-device-plugins/pkg/plugin"

	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	PluginNamespace = "devices.anza-labs.dev"
	gracePeriod     = 5 * time.Second
)

type Server interface {
	v1beta1.DevicePluginServer
	Name() string
	Socket() string
}

type HealthServer interface {
	grpc_health_v1.HealthServer
	SetServingStatus(service string, servingStatus grpc_health_v1.HealthCheckResponse_ServingStatus)
}

func Run(
	ctx context.Context,
	log *slog.Logger,
	devicePluginServer Server,
	healthServer HealthServer,
) error {
	log.Info("Starting plugin")
	eg, ctx := errgroup.WithContext(ctx)

	dps := plugin.New(log)

	grpcServer := dps.DevicePluginServer(devicePluginServer)
	httpServer := metricsServer()

	if healthServer == nil {
		healthServer = health.NewServer()
	}
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	eg.Go(func() error {
		log.Info("Starting shutdown controller")
		return shutdown(ctx, log, grpcServer, httpServer)
	})

	if devicePluginServer != nil {
		eg.Go(func() error {
			log.Info("Registering device plugin")
			return dps.RegisterDevicePlugin(ctx, devicePluginServer.Name(), devicePluginServer.Socket())
		})

		eg.Go(func() error {
			lis, cleanup, err := listener(ctx, log, "tcp://0.0.0.0:8080")
			if err != nil {
				return fmt.Errorf("failed to create http listener: %w", err)
			}
			defer cleanup()

			log.Info("Starting HTTP server")
			return httpServer.Serve(lis)
		})

		eg.Go(func() error {
			lis, cleanup, err := listener(ctx, log, devicePluginServer.Socket())
			if err != nil {
				return fmt.Errorf("failed to create grpc listener: %w", err)
			}
			defer cleanup()

			// Mark server as healthy
			healthServer.SetServingStatus(devicePluginServer.Name(), grpc_health_v1.HealthCheckResponse_SERVING)

			log.Info("Starting gRPC server")
			return grpcServer.Serve(lis)
		})

		if du, ok := devicePluginServer.(discovery.DiscoverUpdater); ok {
			eg.Go(func() error {
				return discovery.Discovery(ctx, log, du)
			})
		}
	} else {
		eg.Go(func() error {
			lis, cleanup, err := listener(ctx, log, fmt.Sprintf("unix://%s", "/health.sock"))
			if err != nil {
				return fmt.Errorf("failed to create grpc listener: %w", err)
			}
			defer cleanup()

			log.Info("Starting gRPC server")
			return grpcServer.Serve(lis)
		})
	}

	log.Info("Plugin is running")
	return eg.Wait()
}

func listener(
	ctx context.Context,
	log *slog.Logger,
	pluginEndpoint string,
) (net.Listener, func(), error) {
	endpointURL, err := url.Parse(pluginEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse plugin endpoint: %w", err)
	}

	listenConfig := net.ListenConfig{}

	if endpointURL.Scheme == "unix" {
		// best effort call to remove the socket if it exists, fixes issue with restarted pod that did not exit gracefully
		_ = os.Remove(endpointURL.Path)
	}

	listener, err := listenConfig.Listen(ctx, endpointURL.Scheme, endpointURL.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create listener: %w", err)
	}

	cleanup := func() {
		if err := listener.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Error("Failed to close listener", "error", err)
			}
		}

		if endpointURL.Scheme == "unix" {
			if err := os.Remove(endpointURL.Path); err != nil {
				log.Error("Failed to remove old socket", "error", err)
			}
		}
	}

	return listener, cleanup, nil
}

func metricsServer() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	return &http.Server{Handler: mux}
}

func shutdown(
	ctx context.Context,
	log *slog.Logger,
	grpcServer *grpc.Server,
	httpServer *http.Server,
) error {
	<-ctx.Done()
	log.Info("Shutting down")
	dctx, stop := context.WithTimeout(context.Background(), gracePeriod)
	defer stop()

	eg, dctx := errgroup.WithContext(dctx)

	if grpcServer != nil {
		eg.Go(func() error {
			log.Debug("Shutting down gRPC server")

			c := make(chan struct{})
			go func() {
				grpcServer.GracefulStop()
				c <- struct{}{}
			}()

			for {
				select {
				case <-dctx.Done():
					log.Info("Forcing gRPC shutdown")
					grpcServer.Stop()
					return nil
				case <-c:
					return nil
				}
			}
		})
	}

	if httpServer != nil {
		eg.Go(func() error {
			log.Debug("Shutting down HTTP server")

			c := make(chan error)
			go func() {
				c <- httpServer.Shutdown(dctx)
			}()

			for {
				select {
				case <-dctx.Done():
					log.Info("Forcing HTTP shutdown")
					return httpServer.Close()
				case err := <-c:
					return err
				}
			}
		})
	}

	return eg.Wait()
}
