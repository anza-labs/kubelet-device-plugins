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

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	flag "github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/health"

	"github.com/anza-labs/kubelet-device-plugins/internal/entrypoint"
	"github.com/anza-labs/kubelet-device-plugins/pkg/servers/tapdeviceplugin"
)

var (
	logLevel    string
	maxDevices  uint
	deviceNames []string
)

func main() {
	flag.StringVar(&logLevel, "log-level", "info", "Set log level (debug, info, warn, error)")
	flag.UintVar(&maxDevices, "devices", 10, "Set number of devices presented to kubelet")
	flag.StringArrayVar(&deviceNames, "device-names", []string{},
		"List of device names that should be discovered by plugin")
	flag.Parse()

	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // Default to info if unknown
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, log, deviceNames); err != nil {
		log.Error("Critical failure", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger, deviceNames []string) error {
	eg, ctx := errgroup.WithContext(ctx)

	hs := health.NewServer()
	eg.Go(func() error {
		return entrypoint.Run(ctx, log, nil, hs)
	})

	for _, name := range deviceNames {
		eg.Go(func() error {
			tap, err := tapdeviceplugin.New(entrypoint.PluginNamespace, name, maxDevices, log)
			if err != nil {
				return err
			}

			return entrypoint.Run(ctx, log, tap, hs)
		})
	}

	return eg.Wait()
}
