package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/al-bo/rd-linkforge/internal/config"
)

func runConfig(ctx context.Context, args []string, env EnvLoader, logger *slog.Logger, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		logger.ErrorContext(ctx, "config does not accept positional arguments")
		return 2
	}

	cfg, err := config.NewEnvLoader(env).Load(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "invalid configuration", slog.Any("error", err))
		return 1
	}
	logger.InfoContext(ctx, "configuration loaded", slog.Any("config", cfg))
	if _, err := fmt.Fprintln(stdout, "config ok"); err != nil {
		logger.ErrorContext(ctx, "error writing result", slog.Any("error", err))
		return 1
	}
	return 0
}
