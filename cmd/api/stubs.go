package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stevenwilliam/ruuma/internal/platform/config"
)

// Filled in by the serve/worker/seed steps of the build.
var errNotWired = errors.New("command not wired yet")

func runServe(_ context.Context, _ *config.Config, _ *slog.Logger) error  { return errNotWired }
func runWorker(_ context.Context, _ *config.Config, _ *slog.Logger) error { return errNotWired }
func runSeed(_ context.Context, _ *config.Config, _ *slog.Logger) error   { return errNotWired }
