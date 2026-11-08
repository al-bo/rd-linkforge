// Command linkctl is the Linkforge operator CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/al-bo/rd-linkforge/internal/config"
)

var version = "dev"

type EnvLoader = func(string) (string, bool)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, env EnvLoader, stdout, stderr io.Writer) int {
	if env == nil {
		env = os.LookupEnv
	}
	level := slog.LevelInfo

	if value, ok := env("LF_LOG_LEVEL"); ok && strings.TrimSpace(value) != "" {
		parsed, err := config.ParseLevel(value)
		if err != nil {
			logger := slog.New(slog.NewTextHandler(stderr, nil))
			logger.ErrorContext(ctx, "invalid log level", slog.Any("error", fmt.Errorf("LF_LOG_LEVEL: %w", err)))
			return 2
		}
		level = parsed
	}
	h := slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)

	if len(args) == 0 {
		logger.ErrorContext(ctx, "command is required")
		return 2
	}
	switch args[0] {
	case "import":
		return runImport(ctx, args[1:], logger, stdout, stderr)
	case "config":
		return runConfig(ctx, args[1:], env, logger, stdout, stderr)
	case "export":
		return runExport(ctx, args[1:], logger, stdout, stderr)
	case "version":
		return runVersion(ctx, args[1:], logger, stdout, stderr)
	default:
		logger.ErrorContext(ctx, "unknown command")
		return 2
	}
}
