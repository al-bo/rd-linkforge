package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
)

func loggingEnv(t *testing.T, values map[string]string) EnvLoader {
	t.Helper()
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func writeLoggingFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestImportLoggingLevels(t *testing.T) {
	cases := []struct {
		level       string
		debug, info bool
		warn        bool
	}{
		{level: "", info: true, warn: true},
		{level: " DeBuG ", debug: true, info: true, warn: true},
		{level: "WARN", warn: true},
		{level: "error"},
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			dir := t.TempDir()
			input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "links.jsonl")
			writeLoggingFile(t, input, "https://user:NEW_PASSWORD@example.com\nnot-a-url\n")
			if err := linkio.SaveFile(db, []link.Link{{ID: 7, Code: "000007", Target: "https://user:OLD_PASSWORD@example.com", OwnerID: "cli"}}); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"import", "-in", input, "-db", db}, loggingEnv(t, map[string]string{"LF_LOG_LEVEL": tc.level}), &stdout, &stderr)
			if code != 1 || stdout.String() != "imported: 1\nskipped: 1\n" {
				t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
			}
			logs := stderr.String()
			for _, check := range []struct {
				text string
				want bool
			}{
				{`level=INFO msg="import started"`, tc.info},
				{`level=INFO msg="import finished" imported=1 skipped=1`, tc.info},
				{`level=WARN msg="target skipped" line=2 error=`, tc.warn},
				{`level=DEBUG msg="target imported" line=1 id=8`, tc.debug},
			} {
				if strings.Contains(logs, check.text) != check.want {
					t.Errorf("presence of %q must be %v; logs=%s", check.text, check.want, logs)
				}
			}
			for _, secret := range []string{"OLD_PASSWORD", "NEW_PASSWORD", "https://", "not-a-url"} {
				if strings.Contains(logs, secret) {
					t.Errorf("logs contain input value %q", secret)
				}
			}
		})
	}
}

func TestImportNewDatabaseAndRepeatedSummary(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "new.jsonl")
	writeLoggingFile(t, input, "https://example.com\n")
	for range 2 {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"import", "-in", input, "-db", db}, loggingEnv(t, nil), &stdout, &stderr)
		if code != 0 || stdout.String() != "imported: 1\nskipped: 0\n" || strings.Contains(stderr.String(), "level=ERROR") {
			t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
		}
	}
}

func TestImportDoesNotLogFinishedWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	writeLoggingFile(t, input, "https://example.com\n")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"import", "-in", input, "-db", filepath.Join(dir, "missing", "db")}, loggingEnv(t, nil), &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "save database failed") || strings.Contains(stderr.String(), "import finished") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestConfigLoggingRedactsCredentials(t *testing.T) {
	values := map[string]string{
		"LF_PG_DSN":     "postgres://user:PG_PASSWORD@localhost/db",
		"LF_MONGO_URI":  "mongodb://user:MONGO_PASSWORD@localhost/db",
		"LF_JWT_SECRET": strings.Repeat("JWT_PRIVATE", 4),
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"config"}, loggingEnv(t, values), &stdout, &stderr)
	if code != 0 || stdout.String() != "config ok\n" {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
	logs := stderr.String()
	for _, key := range []string{"http_addr", "admin_addr", "tcp_addr", "cache_size", "log_level"} {
		if !strings.Contains(logs, "config."+key+"=") {
			t.Errorf("missing config attribute %q: %s", key, logs)
		}
	}
	for _, key := range []string{"pg_dsn", "mongo_uri", "jwt_secret"} {
		if !strings.Contains(logs, "config."+key+"=[redacted]") {
			t.Errorf("missing redacted attribute %q: %s", key, logs)
		}
	}
	for _, secret := range values {
		if strings.Contains(logs, secret) {
			t.Error("credential leaked to log")
		}
	}
}

func TestConfigReportsMissingVariablesOnSeparateLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"config"}, loggingEnv(t, nil), &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, stdout.String())
	}
	want := "invalid configuration:\n" +
		"LF_PG_DSN: config: required variable is not set\n" +
		"LF_MONGO_URI: config: required variable is not set\n" +
		"LF_JWT_SECRET: config: required variable is not set\n"
	if stderr.String() != want {
		t.Errorf("stderr=%q, want %q", stderr.String(), want)
	}
}

func TestInvalidLogLevelDoesNotEchoValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	secret := "postgres://user:PRIVATE_PASSWORD@localhost/db"
	code := run(context.Background(), []string{"config"}, loggingEnv(t, map[string]string{"LF_LOG_LEVEL": secret}), &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "LF_LOG_LEVEL:") || strings.Contains(stderr.String(), "PRIVATE_PASSWORD") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestExportDoesNotLogDatabaseValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.jsonl")
	if err := linkio.SaveFile(path, []link.Link{{ID: 1, Code: "000001", Target: "https://user:PRIVATE_PASSWORD@example.com", OwnerID: "cli"}}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"export", "-db", path}, loggingEnv(t, map[string]string{"LF_LOG_LEVEL": "debug"}), &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "PRIVATE_PASSWORD") || strings.Contains(stderr.String(), "PRIVATE_PASSWORD") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `msg="export finished" exported=1`) || strings.Contains(stdout.String(), "level=") {
		t.Fatalf("unexpected output streams: stdout=%q stderr=%s", stdout.String(), stderr.String())
	}

	data, err := json.Marshal(map[string]string{"CreatedAt": "PRIVATE_PASSWORD"})
	if err != nil {
		t.Fatal(err)
	}
	writeLoggingFile(t, path, string(data)+"\n")
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"export", "-db", path}, loggingEnv(t, nil), &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "record 1:") || strings.Contains(stderr.String(), "PRIVATE_PASSWORD") || strings.Contains(stderr.String(), "export finished") {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
}

type failingOutput struct{ err error }

func (w failingOutput) Write([]byte) (int, error) { return 0, w.err }

func TestImportLogsResultWriteFailure(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "urls.txt")
	writeLoggingFile(t, input, "https://example.com\n")
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"import", "-in", input, "-db", filepath.Join(dir, "db")}, loggingEnv(t, nil), failingOutput{err: errors.New("output unavailable")}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "write import result failed") || strings.Contains(stderr.String(), "import finished") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestCanceledImportLogsFailureWithoutChangingDatabase(t *testing.T) {
	dir := t.TempDir()
	input, db := filepath.Join(dir, "urls.txt"), filepath.Join(dir, "db")
	writeLoggingFile(t, input, "https://example.com\n")
	writeLoggingFile(t, db, "existing database")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer
	code := run(ctx, []string{"import", "-in", input, "-db", db}, loggingEnv(t, nil), io.Discard, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "import canceled") || strings.Contains(stderr.String(), "import finished") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(db)
	if err != nil || string(data) != "existing database" {
		t.Fatalf("database changed: %q error=%v", data, err)
	}
}
