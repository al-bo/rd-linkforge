package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	ErrMissing = errors.New("config: required variable is not set")
	ErrInvalid = errors.New("config: invalid value")
)

var _ Loader = (*EnvLoader)(nil)

type EnvLoader struct {
	lookup func(string) (string, bool)
}

// NewEnvLoader takes the function that reads variables, so that tests do not have
// to touch the real environment. A nil lookup means os.LookupEnv.
func NewEnvLoader(lookup func(key string) (value string, ok bool)) *EnvLoader {
	if lookup == nil {
		return &EnvLoader{
			lookup: os.LookupEnv,
		}
	}

	return &EnvLoader{
		lookup: lookup,
	}
}

func (l *EnvLoader) Load(ctx context.Context) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:  l.value("LF_HTTP_ADDR", ":8080"),
		AdminAddr: l.value("LF_ADMIN_ADDR", "127.0.0.1:6060"),
		TCPAddr:   l.value("LF_TCP_ADDR", ":7070"),
		PGDSN:     l.value("LF_PG_DSN", ""),
		MongoURI:  l.value("LF_MONGO_URI", ""),
		JWTSecret: l.value("LF_JWT_SECRET", ""),
	}

	var problems []error
	for _, address := range []struct{ key, value string }{
		{"LF_HTTP_ADDR", cfg.HTTPAddr},
		{"LF_ADMIN_ADDR", cfg.AdminAddr},
		{"LF_TCP_ADDR", cfg.TCPAddr},
	} {
		if err := validateAddress(address.value); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", address.key, err))
		}
	}
	for _, credential := range []struct{ key, value string }{
		{"LF_PG_DSN", cfg.PGDSN},
		{"LF_MONGO_URI", cfg.MongoURI},
		{"LF_JWT_SECRET", cfg.JWTSecret},
	} {
		if credential.value == "" {
			problems = append(problems, fmt.Errorf("%s: %w", credential.key, ErrMissing))
		}
	}
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		problems = append(problems, fmt.Errorf("LF_JWT_SECRET: %w: must contain at least 32 bytes", ErrInvalid))
	}

	cacheSize, err := strconv.Atoi(l.value("LF_CACHE_SIZE", "100000"))
	if err != nil || cacheSize <= 0 {
		problems = append(problems, fmt.Errorf("LF_CACHE_SIZE: %w: must be a positive integer", ErrInvalid))
	}
	cfg.CacheSize = cacheSize
	level, err := ParseLevel(l.value("LF_LOG_LEVEL", "info"))
	if err != nil {
		problems = append(problems, fmt.Errorf("LF_LOG_LEVEL: %w", err))
	}
	cfg.LogLevel = level
	if err := errors.Join(problems...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (l *EnvLoader) value(key, fallback string) string {
	value, ok := l.lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func validateAddress(value string) error {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("%w: expected host:port", ErrInvalid)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("%w: port must be an integer from 1 to 65535", ErrInvalid)
	}
	return nil
}

// ParseLevel accepts debug, info, warn and error in any letter case, with spaces around.
// Anything else returns an error that wraps ErrInvalid.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: expected debug, info, warn or error", ErrInvalid)
	}
}

func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("http_addr", c.HTTPAddr),
		slog.String("admin_addr", c.AdminAddr),
		slog.String("tcp_addr", c.TCPAddr),
		slog.String("pg_dsn", "[redacted]"),
		slog.String("mongo_uri", "[redacted]"),
		slog.String("jwt_secret", "[redacted]"),
		slog.Int("cache_size", c.CacheSize),
		slog.String("log_level", c.LogLevel.String()),
	)
}
