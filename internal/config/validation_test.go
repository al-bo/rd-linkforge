package config

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func validEnvironment(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"LF_PG_DSN":     "postgres://test-user:test-password@localhost/test",
		"LF_MONGO_URI":  "mongodb://test-user:test-password@localhost/test",
		"LF_JWT_SECRET": strings.Repeat("x", 32),
	}
}

func loaderForValues(t *testing.T, values map[string]string) *EnvLoader {
	t.Helper()
	return NewEnvLoader(func(key string) (string, bool) { value, ok := values[key]; return value, ok })
}

func TestEnvironmentValidation(t *testing.T) {
	cases := []struct {
		name, key, value string
		unset            bool
		wantErr          error
	}{
		{name: "missing postgres", key: "LF_PG_DSN", unset: true, wantErr: ErrMissing},
		{name: "missing mongo", key: "LF_MONGO_URI", unset: true, wantErr: ErrMissing},
		{name: "missing JWT", key: "LF_JWT_SECRET", unset: true, wantErr: ErrMissing},
		{name: "empty postgres", key: "LF_PG_DSN", wantErr: ErrMissing},
		{name: "blank mongo", key: "LF_MONGO_URI", value: " \t ", wantErr: ErrMissing},
		{name: "blank JWT", key: "LF_JWT_SECRET", value: " \n ", wantErr: ErrMissing},
		{name: "short JWT", key: "LF_JWT_SECRET", value: strings.Repeat("x", 31), wantErr: ErrInvalid},
		{name: "JWT exactly 32 bytes", key: "LF_JWT_SECRET", value: strings.Repeat("x", 32)},
		{name: "JWT Unicode byte length", key: "LF_JWT_SECRET", value: strings.Repeat("я", 16)},
		{name: "postgres is not parsed", key: "LF_PG_DSN", value: "opaque DSN"},
		{name: "mongo is not parsed", key: "LF_MONGO_URI", value: "opaque URI"},
		{name: "HTTP missing port", key: "LF_HTTP_ADDR", value: "localhost", wantErr: ErrInvalid},
		{name: "admin empty port", key: "LF_ADMIN_ADDR", value: "localhost:", wantErr: ErrInvalid},
		{name: "TCP nonnumeric port", key: "LF_TCP_ADDR", value: ":abc", wantErr: ErrInvalid},
		{name: "port zero", key: "LF_HTTP_ADDR", value: ":0", wantErr: ErrInvalid},
		{name: "port negative", key: "LF_ADMIN_ADDR", value: ":-1", wantErr: ErrInvalid},
		{name: "port too large", key: "LF_TCP_ADDR", value: ":65536", wantErr: ErrInvalid},
		{name: "port minimum", key: "LF_HTTP_ADDR", value: ":1"},
		{name: "port maximum", key: "LF_TCP_ADDR", value: ":65535"},
		{name: "IPv6 address", key: "LF_ADMIN_ADDR", value: "[::1]:6060"},
		{name: "empty optional address", key: "LF_HTTP_ADDR"},
		{name: "blank optional address", key: "LF_TCP_ADDR", value: " \t "},
		{name: "cache invalid number", key: "LF_CACHE_SIZE", value: "abc", wantErr: ErrInvalid},
		{name: "cache fractional", key: "LF_CACHE_SIZE", value: "1.5", wantErr: ErrInvalid},
		{name: "cache overflow", key: "LF_CACHE_SIZE", value: strings.Repeat("9", 50), wantErr: ErrInvalid},
		{name: "cache zero", key: "LF_CACHE_SIZE", value: "0", wantErr: ErrInvalid},
		{name: "cache negative", key: "LF_CACHE_SIZE", value: "-1", wantErr: ErrInvalid},
		{name: "cache minimum", key: "LF_CACHE_SIZE", value: "1"},
		{name: "blank cache uses default", key: "LF_CACHE_SIZE", value: " "},
		{name: "invalid log level", key: "LF_LOG_LEVEL", value: "trace", wantErr: ErrInvalid},
		{name: "blank log level uses default", key: "LF_LOG_LEVEL", value: " "},
		{name: "mixed case log level", key: "LF_LOG_LEVEL", value: " DeBuG "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := validEnvironment(t)
			if tc.unset {
				delete(values, tc.key)
			} else {
				values[tc.key] = tc.value
			}
			cfg, err := loaderForValues(t, values).Load(context.Background())
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("valid environment rejected: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), tc.key+":") || cfg != (Config{}) {
				t.Errorf("expected variable-prefixed error and zero config; error=%v", err)
			}
			for _, key := range []string{"LF_PG_DSN", "LF_MONGO_URI", "LF_JWT_SECRET"} {
				if secret := strings.TrimSpace(values[key]); secret != "" && strings.Contains(err.Error(), secret) {
					t.Errorf("error exposes credential %s", key)
				}
			}
		})
	}
}

func TestConfigAllDefaultsAndOverrides(t *testing.T) {
	for _, override := range []bool{false, true} {
		name := "defaults"
		if override {
			name = "trimmed overrides"
		}
		t.Run(name, func(t *testing.T) {
			values := validEnvironment(t)
			want := Config{HTTPAddr: ":8080", AdminAddr: "127.0.0.1:6060", TCPAddr: ":7070", PGDSN: values["LF_PG_DSN"], MongoURI: values["LF_MONGO_URI"], JWTSecret: values["LF_JWT_SECRET"], CacheSize: 100000, LogLevel: slog.LevelInfo}
			if override {
				want.HTTPAddr, want.AdminAddr, want.TCPAddr = ":8081", "localhost:6061", "[::1]:7071"
				want.CacheSize, want.LogLevel = 15, slog.LevelWarn
				values["LF_HTTP_ADDR"], values["LF_ADMIN_ADDR"], values["LF_TCP_ADDR"] = want.HTTPAddr, want.AdminAddr, want.TCPAddr
				values["LF_CACHE_SIZE"], values["LF_LOG_LEVEL"] = "15", "WaRn"
				for key, value := range values {
					values[key] = " \t" + value + " \t"
				}
			}
			got, err := loaderForValues(t, values).Load(context.Background())
			if err != nil || got != want {
				t.Fatalf("config does not match expected fields; error=%v", err)
			}
		})
	}
}

func TestConfigLogValue(t *testing.T) {
	for _, populated := range []bool{false, true} {
		cfg := Config{HTTPAddr: ":8080", AdminAddr: "127.0.0.1:6060", TCPAddr: ":7070", CacheSize: 100000, LogLevel: slog.LevelInfo}
		if populated {
			cfg.PGDSN, cfg.MongoURI, cfg.JWTSecret = "PRIVATE_PG", "PRIVATE_MONGO", "PRIVATE_JWT"
		}
		value := cfg.LogValue()
		if value.Kind() != slog.KindGroup {
			t.Fatal("LogValue must return a group")
		}
		attrs := make(map[string]slog.Value)
		for _, attr := range value.Group() {
			attrs[attr.Key] = attr.Value
		}
		if len(attrs) != 8 {
			t.Fatalf("got %d attributes; want 8", len(attrs))
		}
		for _, key := range []string{"http_addr", "admin_addr", "tcp_addr", "cache_size", "log_level"} {
			if _, ok := attrs[key]; !ok {
				t.Errorf("missing attribute %s", key)
			}
		}
		for _, key := range []string{"pg_dsn", "mongo_uri", "jwt_secret"} {
			if attrs[key].String() != "[redacted]" {
				t.Errorf("%s is not redacted", key)
			}
		}
		if strings.Contains(value.String(), "PRIVATE_") {
			t.Fatal("LogValue exposes a secret")
		}
	}
}
