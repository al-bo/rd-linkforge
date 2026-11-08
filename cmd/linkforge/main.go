package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/al-bo/rd-linkforge/internal/config"
	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/store"
)

func main() {
	ctx := context.Background()
	level := slog.LevelInfo
	if value := strings.TrimSpace(os.Getenv("LF_LOG_LEVEL")); value != "" {
		parsed, err := config.ParseLevel(value)
		if err != nil {
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			logger.ErrorContext(ctx, "invalid log level", slog.Any("error", fmt.Errorf("LF_LOG_LEVEL: %w", err)))
			os.Exit(2)
		}
		level = parsed
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	s := store.New()
	service := link.NewService(s)
	owner := "demo"
	userArgs := os.Args[1:]

	for i, v := range userArgs {
		req := link.ShortenRequest{Target: v, Alias: "", OwnerID: owner}
		_, err := service.Shorten(ctx, req)

		if err != nil {
			logger.ErrorContext(ctx, "shorten failed", slog.Int("argument", i+1), slog.Any("error", err))
		}
	}

	for _, l := range s.All() {
		fmt.Printf("%s    %s\n", l.Code, l.Target)
	}

	fmt.Printf("Total: %v\n", s.Count())
}
