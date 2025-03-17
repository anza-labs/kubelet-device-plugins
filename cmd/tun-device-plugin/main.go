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

	"github.com/anza-labs/kubelet-device-plugins/internal/entrypoint"
	"github.com/anza-labs/kubelet-device-plugins/pkg/servers/tundeviceplugin"
)

var (
	logLevel   string
	maxDevices uint
)

func main() {
	flag.StringVar(&logLevel, "log-level", "info", "Set log level (debug, info, warn, error)")
	flag.UintVar(&maxDevices, "devices", 10, "Set number of devices presented to kubelet")
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

	tun := tundeviceplugin.New(entrypoint.PluginNamespace, maxDevices, log)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := entrypoint.Run(ctx, log, tun, nil); err != nil {
		log.Error("Critical failure", "error", err)
		os.Exit(1)
	}
}
