package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/al-bo/rd-linkforge/internal/link"
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
		logger.ErrorContext(ctx, "error reading database", slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "import database success", slog.Any("db", db))
	}

	csv, err := link.ListToCSV(db)

	if err != nil {
		logger.ErrorContext(ctx, "error exporting to csv:", slog.Any("error", err))
		return 1
	}

	if *outPath != "" {
		logger.InfoContext(ctx, "export destination", slog.String("out", *outPath))

		file, err := os.Create(*outPath)

		if err != nil {
			logger.ErrorContext(ctx, "error creating file for csv output:", slog.Any("error", err))
		}
		defer file.Close()

		w := bufio.NewWriter(file)
		_, writeError := w.WriteString(csv)

		if writeError != nil {
			logger.ErrorContext(ctx, "error writing csv to output:", slog.Any("error", err))
		}

		e := w.Flush()

		if e != nil {
			logger.ErrorContext(ctx, "error writing flush file:", slog.Any("error", err))
		}
	} else {
		logger.InfoContext(ctx, "export destination", slog.String("out", "stdout"))
		if _, err := io.WriteString(stdout, csv); err != nil {
			logger.ErrorContext(ctx, "error writing csv to stdout", slog.Any("error", err))
			return 1
		}
	}
	return 0
}
