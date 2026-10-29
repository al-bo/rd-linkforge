package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
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
	if *outPath != "" {
		logger.InfoContext(ctx, "export destination", slog.String("out", *outPath))
	} else {
		logger.InfoContext(ctx, "export destination", slog.String("out", "stdout"))
	}
	return 0
}
