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

const version = "dev"

type EnvLoader = func(string) (string, bool)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, env EnvLoader, stdout, stderr io.Writer) int {
	level := slog.LevelInfo

	if value, ok := env("LF_LOG_LEVEL"); ok && strings.TrimSpace(value) != "" {
		parsed, err := config.ParseLevel(value)
		if err != nil {
			logger := slog.New(slog.NewTextHandler(stderr, nil))
			logger.ErrorContext(ctx, "invalid log level", slog.Any("error", err))
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
		if len(args) != 1 {
			logger.ErrorContext(ctx, "version does not accept arguments")
			return 2
		}
		if _, err := fmt.Fprintf(stdout, "linkctl %s\n", version); err != nil {
			logger.ErrorContext(ctx, "error writing version", slog.Any("error", err))
			return 1
		}
		return 0
	default:
		logger.ErrorContext(ctx, "unknown command")
		return 2
	}
}
