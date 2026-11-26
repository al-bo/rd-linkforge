// Command linkforged serves a JSON Lines link database over TCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/al-bo/rd-linkforge/internal/config"
	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
	"github.com/al-bo/rd-linkforge/internal/store"
	"github.com/al-bo/rd-linkforge/internal/transport"
	"github.com/al-bo/rd-linkforge/internal/transport/tcp"
	"github.com/al-bo/rd-linkforge/pkg/cache"
)

// EnvLoader reads configuration values. Each run calls it synchronously;
// callers sharing it across runs must provide a concurrency-safe function.
type EnvLoader = func(string) (string, bool)

const shutdownTimeout = 15 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, env EnvLoader, stderr io.Writer) int {
	if env == nil {
		env = os.LookupEnv
	}
	level := slog.LevelInfo

	if value, ok := env("LF_LOG_LEVEL"); ok && strings.TrimSpace(value) != "" {
		parsed, err := config.ParseLevel(value)
		if err != nil {
			logger := slog.New(slog.NewJSONHandler(stderr, nil))
			logger.ErrorContext(ctx, "invalid log level", slog.Any("error", fmt.Errorf("LF_LOG_LEVEL: %w", err)))
			return 2
		}
		level = parsed
	}
	h := slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)

	flags := flag.NewFlagSet("linkforged", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "Path to database (required)")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: linkforged -db FILE\n\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *dbPath == "" {
		logger.ErrorContext(ctx, "flag -db is required")
		flags.Usage()
		return 2
	}
	if flags.NArg() != 0 {
		logger.ErrorContext(ctx, "linkforged does not accept positional arguments")
		flags.Usage()
		return 2
	}

	cfg, err := config.NewEnvLoader(env).Load(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration:\n%v\n", err)
		return 1
	}

	db, err := linkio.LoadFile(*dbPath)

	if err != nil {
		logger.ErrorContext(ctx, "load database:", slog.Any("error", err))
		return 1
	}

	logger.DebugContext(ctx, "database loaded", slog.Int("count", len(db)))
	logger.InfoContext(ctx, "configuration loaded", slog.Any("config", cfg))

	st := store.New()
	for _, stored := range db {
		if _, err := st.Create(ctx, stored); err != nil {
			logger.ErrorContext(ctx, "restore link:", slog.Uint64("id", stored.ID), slog.String("code", stored.Code), slog.Any("error", err))
			return 1
		}
	}

	lru, err := cache.NewLRU[string, link.Link](cfg.CacheSize)

	if err != nil {
		logger.ErrorContext(ctx, "cache failed:", slog.Any("error", err))
		return 1
	}

	service := link.NewService(st, link.WithCache(lru))
	var srv transport.Server = tcp.NewServer(
		cfg.TCPAddr,
		service,
		tcp.WithLogger(logger),
	)

	startErr := srv.Start(ctx)

	if startErr != nil {
		logger.ErrorContext(ctx, "start resolver:", slog.Any("error", startErr))
		return 1
	}

	logger.Info("resolver started",
		"addr", srv.Addr(),
		"links", len(db),
	)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()
	shutDownErr := srv.Shutdown(shutdownCtx)

	if shutDownErr != nil {
		logger.ErrorContext(shutdownCtx, "stop resolver:", slog.Any("error", shutDownErr))

		return 1
	}

	logger.Info("resolver stopped")
	return 0
}
