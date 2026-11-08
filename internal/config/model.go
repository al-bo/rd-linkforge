// Package config defines how runtime configuration is obtained. Configuration
// is environment-only and validated at startup.
package config

import "log/slog"

// Config is the fully resolved runtime configuration.
type Config struct {
	HTTPAddr  string
	AdminAddr string
	TCPAddr   string
	PGDSN     string
	MongoURI  string
	JWTSecret string
	CacheSize int
	LogLevel  slog.Level
}
