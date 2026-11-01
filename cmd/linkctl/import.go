package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
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
	if err := ctx.Err(); err != nil {
		logger.ErrorContext(ctx, "import canceled", slog.Any("error", err))
		return 1
	}
	targets, err := linkio.ReadTargetsFile(*inPath)
	if err != nil {
		logger.ErrorContext(ctx, "read input failed", slog.Any("error", err))
		return 1
	}

	db, err := linkio.LoadFile(*dbPath)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.ErrorContext(ctx, "load database failed", slog.Any("error", err))
		return 1
	}
	logger.DebugContext(ctx, "database loaded", slog.Int("count", len(db)))

	st := store.New()
	var maxID uint64
	for _, stored := range db {
		if _, err := st.Create(ctx, stored); err != nil {
			logger.ErrorContext(ctx, "restore link failed", slog.Uint64("id", stored.ID), slog.Any("error", err))
			return 1
		}
		if stored.ID > maxID {
			maxID = stored.ID
		}
	}

	service := link.NewService(st, link.WithFirstID(maxID+1))

	imported := 0
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
			if ctxErr := ctx.Err(); ctxErr != nil {
				logger.ErrorContext(ctx, "import canceled", slog.Any("error", ctxErr))
				return 1
			}
			skipped++
			logger.WarnContext(ctx, "target skipped", slog.Int("line", target.Line), slog.Any("error", err))
			continue
		}
		imported++
		logger.DebugContext(ctx, "target imported", slog.Int("line", target.Line), slog.Uint64("id", created.ID))
	}
	if err := ctx.Err(); err != nil {
		logger.ErrorContext(ctx, "import canceled", slog.Any("error", err))
		return 1
	}
	saveErr := linkio.SaveFile(*dbPath, st.All())

	if saveErr != nil {
		logger.ErrorContext(ctx, "save database failed", slog.Any("error", saveErr))
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "imported: %d\nskipped: %d\n", imported, skipped); err != nil {
		logger.ErrorContext(ctx, "write import result failed", slog.Any("error", err))
		return 1
	}
	logger.InfoContext(ctx, "import finished", slog.Int("imported", imported), slog.Int("skipped", skipped))

	if skipped > 0 {
		return 1
	}
	return 0
}
