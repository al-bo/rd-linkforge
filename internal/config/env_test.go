package config

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestLoadReportsAllProblemsWithoutInputValues(t *testing.T) {
	values := map[string]string{
		"LF_HTTP_ADDR":  "PRIVATE_HTTP_INPUT",
		"LF_ADMIN_ADDR": ":0",
		"LF_TCP_ADDR":   ":65536",
		"LF_JWT_SECRET": "PRIVATE_JWT_INPUT",
		"LF_CACHE_SIZE": "PRIVATE_CACHE_INPUT",
		"LF_LOG_LEVEL":  "PRIVATE_LEVEL_INPUT",
	}
	loader := NewEnvLoader(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	cfg, err := loader.Load(context.Background())
	if !errors.Is(err, ErrMissing) || !errors.Is(err, ErrInvalid) || cfg != (Config{}) {
		t.Fatalf("expected zero config and joined errors, got config=%+v error=%v", cfg, err)
	}
	for _, key := range []string{"LF_HTTP_ADDR", "LF_ADMIN_ADDR", "LF_TCP_ADDR", "LF_PG_DSN", "LF_MONGO_URI", "LF_JWT_SECRET", "LF_CACHE_SIZE", "LF_LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key+":") {
			t.Errorf("missing %s in %v", key, err)
		}
	}
	if strings.Contains(err.Error(), "PRIVATE_") {
		t.Fatal("configuration error exposes an input value")
	}
}

func TestLoadDefaultsAndTrimming(t *testing.T) {
	values := map[string]string{
		"LF_HTTP_ADDR":  "  ",
		"LF_ADMIN_ADDR": " 127.0.0.1:9090 ",
		"LF_PG_DSN":     " postgres://private ",
		"LF_MONGO_URI":  " mongodb://private ",
		"LF_JWT_SECRET": " " + strings.Repeat("x", 32) + " ",
		"LF_CACHE_SIZE": " 12 ",
		"LF_LOG_LEVEL":  "  ",
	}
	loader := NewEnvLoader(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
	cfg, err := loader.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.AdminAddr != "127.0.0.1:9090" || cfg.TCPAddr != ":7070" || cfg.CacheSize != 12 || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected non-secret config: http=%q admin=%q tcp=%q cache=%d level=%v", cfg.HTTPAddr, cfg.AdminAddr, cfg.TCPAddr, cfg.CacheSize, cfg.LogLevel)
	}
	if cfg.PGDSN != "postgres://private" || cfg.MongoURI != "mongodb://private" || len(cfg.JWTSecret) != 32 {
		t.Fatal("credential whitespace was not trimmed")
	}
}

func TestLoadCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loader := NewEnvLoader(func(string) (string, bool) {
		t.Fatal("canceled loader accessed environment")
		return "", false
	})
	cfg, err := loader.Load(ctx)
	if !errors.Is(err, context.Canceled) || cfg != (Config{}) {
		t.Fatalf("canceled load: config=%+v error=%v", cfg, err)
	}
}

func TestParseLevel(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  slog.Level
	}{
		{" DeBuG ", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{" error ", slog.LevelError},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseLevel(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	_, err := ParseLevel("PRIVATE_LEVEL_VALUE")
	if !errors.Is(err, ErrInvalid) || strings.Contains(err.Error(), "PRIVATE_LEVEL_VALUE") {
		t.Fatalf("unsafe or invalid error: %v", err)
	}
}
