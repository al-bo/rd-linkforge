package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
)

func runVersion(ctx context.Context, args []string, logger *slog.Logger, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		logger.ErrorContext(ctx, "version does not accept positional arguments")
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "linkctl %s\n", version); err != nil {
		logger.ErrorContext(ctx, "write version failed", slog.Any("error", err))
		return 1
	}
	return 0
}
