// Command linkctl is the Linkforge operator CLI.
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

	"github.com/al-bo/rd-linkforge/internal/config"
	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
)

const version = "dev"

type EnvLoader = func(string) (string, bool)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func loadConfig(ctx context.Context, env EnvLoader, logger *slog.Logger) {
	cl := config.NewEnvLoader(env)
	config, err := cl.Load(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "error loading config", slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "configuration loaded", slog.Any("config", config))
	}
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

	exportCmd := flag.NewFlagSet("export", flag.ContinueOnError)
	exportCmd.SetOutput(stderr)
	dbPath := exportCmd.String("db", "", "Path to database (required)")
	outPath := exportCmd.String("out", "", "Path to output (optional)")
	exportCmd.Usage = func() {
		fmt.Fprintf(stderr, "Usage: linkctl export -db FILE [-out FILE]\n\n flags:\n")
		exportCmd.PrintDefaults()
	}

	importCmd := flag.NewFlagSet("import", flag.ContinueOnError)
	importCmd.SetOutput(stderr)
	inPath := importCmd.String("in", "", "path to source file (required)")
	dstPath := importCmd.String("db", "", "Path to database (required)")
	importCmd.Usage = func() {
		fmt.Fprintf(stderr, "Usage: linkctl import -in File -db FILE\n\n flags:\n")
		importCmd.PrintDefaults()
	}

	if len(args) == 0 {
		logger.ErrorContext(ctx, "command is required")
		return 2
	}
	switch args[0] {
	case "import":
		{
			if err := importCmd.Parse(args[1:]); err != nil {
				return 2
			}
			if *inPath != "" && *dstPath != "" {
				logger.InfoContext(ctx, "import started", slog.String("in", *inPath), slog.String("db", *dstPath))
			} else {
				logger.ErrorContext(ctx, "flags -in and -db are required")
				return 1
			}
			break
		}
	case "config":
		{
			loadConfig(ctx, env, logger)
			break
		}
	case "export":
		{
			if err := exportCmd.Parse(args[1:]); err != nil {
				return 2
			}
			if *dbPath == "" {
				logger.ErrorContext(ctx, "flag -db is required")
				exportCmd.Usage()
				return 1
			}

			logger.InfoContext(ctx, "export started", slog.String("db", *dbPath))
			if *outPath != "" {
				logger.InfoContext(ctx, "export destination", slog.String("out", *outPath))
			} else {
				logger.InfoContext(ctx, "export destination", slog.String("out", "stdout"))
			}

			return 0
		}
	default:
		{
			f := "out.txt"
			tmp := []link.Link{
				{ID: 1, Code: "adb", Target: "http://sample.com", OwnerID: "abc"},
				{ID: 2, Code: "bar", Target: "http://sample2.com", OwnerID: "abc"},
			}
			e := linkio.SaveFile(f, tmp)

			if e != nil {
				logger.ErrorContext(ctx, "save file failed", slog.String("path", f), slog.Any("error", e))
			} else {
				logger.InfoContext(ctx, "file saved", slog.String("path", f))
			}
			return 2
		}
	}

	return 0
}
