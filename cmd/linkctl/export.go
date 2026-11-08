package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/al-bo/rd-linkforge/internal/linkio"
)

func runExport(ctx context.Context, args []string, logger *slog.Logger, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "Path to database (required)")
	outPath := flags.String("out", "", "Path to output (optional)")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: linkctl export -db FILE [-out FILE]\n\n flags:\n")
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
		logger.ErrorContext(ctx, "export does not accept positional arguments")
		return 2
	}

	logger.InfoContext(ctx, "export started", slog.String("db", *dbPath))

	db, err := linkio.LoadFile(*dbPath)

	if err != nil {
		logger.ErrorContext(ctx, "load database failed", slog.Any("error", err))
		return 1
	}
	logger.DebugContext(ctx, "database loaded", slog.Int("count", len(db)))

	if *outPath != "" {
		logger.DebugContext(ctx, "export destination", slog.String("out", *outPath))

		file, err := os.Create(*outPath)

		if err != nil {
			logger.ErrorContext(ctx, "create export file failed", slog.Any("error", err))
			return 1
		}
		writeErr := linkio.WriteCSV(file, db)
		closeErr := file.Close()

		if writeErr != nil {
			logger.ErrorContext(ctx, "write export failed", slog.Any("error", writeErr))
		}
		if closeErr != nil {
			logger.ErrorContext(ctx, "close export file failed", slog.Any("error", closeErr))
		}
		if writeErr != nil || closeErr != nil {
			return 1
		}
	} else {
		logger.DebugContext(ctx, "export destination", slog.String("out", "stdout"))
		if err := linkio.WriteCSV(stdout, db); err != nil {
			logger.ErrorContext(ctx, "write export failed", slog.Any("error", err))
			return 1
		}
	}
	logger.InfoContext(ctx, "export finished", slog.Int("exported", len(db)))
	return 0
}
