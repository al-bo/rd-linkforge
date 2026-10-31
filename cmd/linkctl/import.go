package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
	"github.com/al-bo/rd-linkforge/internal/store"
)

func runImport(ctx context.Context, args []string, logger *slog.Logger, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	inPath := flags.String("in", "", "Path to source file (required)")
	dbPath := flags.String("db", "", "Path to database (required)")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: linkctl import -in FILE -db FILE\n\n flags:\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *inPath == "" || *dbPath == "" {
		logger.ErrorContext(ctx, "flags -in and -db are required")
		flags.Usage()
		return 2
	}
	if flags.NArg() != 0 {
		logger.ErrorContext(ctx, "import does not accept positional arguments")
		return 2
	}

	logger.InfoContext(ctx, "import started", slog.String("in", *inPath), slog.String("db", *dbPath))
	targets, err := linkio.ReadTargetsFile(*inPath)
	if err != nil {
		logger.ErrorContext(ctx, "error reading file", slog.Any("error", err))
		return 1
	}

	db, err := linkio.LoadFile(*dbPath)

	if err != nil {
		logger.ErrorContext(ctx, "error reading database", slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "import database success", slog.Any("db", db))
	}

	st := store.New()
	for _, stored := range db {
		st.Create(ctx, stored)
	}

	lastID := st.GetMaxID(ctx)
	service := link.NewService(st, link.WithFirstID(lastID+1))

	skipped := 0
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			logger.ErrorContext(ctx, "import canceled", slog.Any("error", err))
			return 1
		}
		created, err := service.Shorten(ctx, link.ShortenRequest{
			Target:  target.URL,
			OwnerID: "cli",
		})
		if err != nil {
			skipped++
			logger.WarnContext(ctx, "target skipped", slog.Int("line", target.Line), slog.Any("error", err))
			continue
		}
		logger.DebugContext(ctx, "target imported", slog.Int("line", target.Line), slog.Uint64("id", created.ID))
	}
	if err := ctx.Err(); err != nil {
		logger.ErrorContext(ctx, "import canceled", slog.Any("error", err))
		return 1
	}
	logger.InfoContext(ctx, "import finished", slog.Int("imported", st.Count()), slog.Int("skipped", skipped))

	saveErr := linkio.SaveFile(*dbPath, st.All())

	if saveErr != nil {
		logger.ErrorContext(ctx, "save to database failed", slog.Any("error", saveErr))
		return 1
	}

	if skipped > 0 {
		return 1
	}
	return 0
}
