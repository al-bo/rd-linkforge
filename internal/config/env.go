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

type EnvLoader struct { /* your fields */
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
		return Config{}, nil
	}

	httpAddr, ok := l.lookup("LF_HTTP_ADDR")

	if !ok {
		httpAddr = ":8080"
	} else if _, _, err := net.SplitHostPort(httpAddr); err != nil {
		return Config{}, fmt.Errorf("LF_HTTP_ADDR: %w", err)
	}

	adminAddr, ok := l.lookup("LF_ADMIN_ADDR")

	if !ok {
		adminAddr = "127.0.0.1:6060"
	} else if _, _, err := net.SplitHostPort(adminAddr); err != nil {
		return Config{}, fmt.Errorf("LF_ADMIN_ADDR: %w", err)
	}

	tcpAddr, ok := l.lookup("LF_TCP_ADDR")

	if !ok {
		tcpAddr = ":7070"
	} else if _, _, err := net.SplitHostPort(tcpAddr); err != nil {
		return Config{}, fmt.Errorf("LF_TCP_ADDR: %w", err)
	}

	pgDSN, ok := l.lookup("LF_PG_DSN")

	if !ok {
		return Config{}, fmt.Errorf("LF_PG_DSN: %w", ErrMissing)
	}

	mongoUri, ok := l.lookup("LF_MONGO_URI")

	if !ok {
		return Config{}, fmt.Errorf("LF_MONGO_URI: %w", ErrMissing)
	}

	jwtSecret, ok := l.lookup("LF_JWT_SECRET")

	if !ok {
		return Config{}, fmt.Errorf("LF_JWT_SECRET: %w", ErrMissing)
	}

	var cacheSize int
	cache, ok := l.lookup("LF_CACHE_SIZE")

	if ok {
		num, err := strconv.Atoi(cache)

		if err != nil {
			return Config{}, fmt.Errorf("LF_CACHE_SIZE: %w", err)
		}

		cacheSize = num
	} else {
		cacheSize = 100000
	}

	level, ok := l.lookup("LF_LOG_LEVEL")
	var logLevel slog.Level

	if ok {
		l, err := ParseLevel(level)

		if err != nil {
			return Config{}, fmt.Errorf("LF_LOG_LEVEL: %w", err)
		}

		logLevel = l
	} else {
		logLevel = slog.LevelInfo
	}

	return Config{
		HTTPAddr:  httpAddr,
		AdminAddr: adminAddr,
		TCPAddr:   tcpAddr,
		PGDSN:     pgDSN,
		MongoURI:  mongoUri,
		JWTSecret: jwtSecret,
		CacheSize: cacheSize,
		LogLevel:  logLevel,
	}, nil
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
		return 0, fmt.Errorf("log level %q: %w", s, ErrInvalid)
	}
}
