package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/linkio"
)

func daemonEnv(t *testing.T, overrides map[string]string) EnvLoader {
	t.Helper()
	values := map[string]string{
		"LF_PG_DSN":     "postgres://user:private-pg-password@localhost/linkforge",
		"LF_MONGO_URI":  "mongodb://user:private-mongo-password@localhost",
		"LF_JWT_SECRET": "private-jwt-secret-at-least-thirty-two-bytes",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func requireNoCredentials(t *testing.T, output string, env EnvLoader) {
	t.Helper()
	for _, key := range []string{"LF_PG_DSN", "LF_MONGO_URI", "LF_JWT_SECRET"} {
		if value, ok := env(key); ok && value != "" && strings.Contains(output, value) {
			t.Errorf("stderr contains the value of %s", key)
		}
	}
	for _, password := range []string{"private-pg-password", "private-mongo-password"} {
		if strings.Contains(output, password) {
			t.Error("stderr contains a database password")
		}
	}
}

func daemonDatabase(t *testing.T, links ...link.Link) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "links.jsonl")
	if err := linkio.SaveFile(path, links); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		env      map[string]string
		wantText string
	}{
		{"missing db", nil, nil, "Usage: linkforged -db FILE"},
		{"empty db", []string{"-db", ""}, nil, "Usage: linkforged -db FILE"},
		{"missing flag value", []string{"-db"}, nil, "Usage: linkforged -db FILE"},
		{"unknown flag", []string{"-unknown"}, nil, "Usage: linkforged -db FILE"},
		{"extra argument", []string{"-db", "unused", "extra"}, nil, "Usage: linkforged -db FILE"},
		{"invalid log level", []string{"-db", "unused"}, map[string]string{"LF_LOG_LEVEL": "verbose"}, "invalid log level"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			env := daemonEnv(t, tc.env)
			var stderr bytes.Buffer
			code := run(ctx, tc.args, env, &stderr)
			if code != 2 || !strings.Contains(stderr.String(), tc.wantText) {
				t.Errorf("code=%d stderr=%q; want code 2 and %q", code, stderr.String(), tc.wantText)
			}
			requireNoCredentials(t, stderr.String(), env)
		})
	}
}

func TestRunInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"LF_PG_DSN", ""},
		{"LF_MONGO_URI", ""},
		{"LF_JWT_SECRET", "too-short-secret"},
		{"LF_TCP_ADDR", "127.0.0.1:0"},
		{"LF_CACHE_SIZE", "0"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			env := daemonEnv(t, map[string]string{tc.key: tc.value})
			var stderr bytes.Buffer
			code := run(t.Context(), []string{"-db", filepath.Join(t.TempDir(), "unused")}, env, &stderr)
			if code != 1 || !strings.HasPrefix(stderr.String(), "invalid configuration:\n") || !strings.Contains(stderr.String(), tc.key) {
				t.Errorf("code=%d stderr=%q", code, stderr.String())
			}
			requireNoCredentials(t, stderr.String(), env)
		})
	}
}

func TestRunDatabaseErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	malformed := filepath.Join(t.TempDir(), "malformed.jsonl")
	if err := os.WriteFile(malformed, []byte("not JSON\n"), 0600); err != nil {
		t.Fatal(err)
	}
	duplicates := daemonDatabase(t, link.Link{ID: 1, Code: "same"}, link.Link{ID: 2, Code: "same"})
	for _, tc := range []struct{ name, path, wantText string }{
		{"missing", missing, "load database:"},
		{"malformed", malformed, "load database:"},
		{"duplicate codes", duplicates, "restore link:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := daemonEnv(t, nil)
			var stderr bytes.Buffer
			code := run(t.Context(), []string{"-db", tc.path}, env, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), tc.wantText) {
				t.Errorf("code=%d stderr=%q", code, stderr.String())
			}
			if tc.name == "duplicate codes" && (!strings.Contains(stderr.String(), `"code":"same"`) || !strings.Contains(stderr.String(), `"id":2`)) {
				t.Errorf("restore error does not identify the duplicate: %s", stderr.String())
			}
			requireNoCredentials(t, stderr.String(), env)
		})
	}
}

func TestRunBusyAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	env := daemonEnv(t, map[string]string{"LF_TCP_ADDR": listener.Addr().String()})
	var stderr bytes.Buffer
	code := run(t.Context(), []string{"-db", daemonDatabase(t)}, env, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "start resolver:") {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
	requireNoCredentials(t, stderr.String(), env)
}

// startupLog makes readiness observable without polling or racing log writes.
type startupLog struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	ready   chan struct{}
	started bool
	addr    string
	links   int
}

func (l *startupLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.buffer.Write(p)
	var record struct {
		Msg   string `json:"msg"`
		Addr  string `json:"addr"`
		Links int    `json:"links"`
	}
	if json.Unmarshal(p, &record) == nil && record.Msg == "resolver started" && !l.started {
		l.addr, l.links, l.started = record.Addr, record.Links, true
		close(l.ready)
	}
	return n, err
}

func (l *startupLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

func TestRunLifecycle(t *testing.T) {
	path := daemonDatabase(t,
		link.Link{ID: 1, Code: "known", Target: "https://example.com/a"},
		link.Link{ID: 2, Code: "old", Target: "https://old.example", ExpiresAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
	)
	// Config rejects port zero, so reserve an address and release it before run.
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	env := daemonEnv(t, map[string]string{"LF_TCP_ADDR": addr, "LF_LOG_LEVEL": " DeBuG "})
	logs := &startupLog{ready: make(chan struct{})}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	done := make(chan struct{})
	var exitCode int
	go func() {
		exitCode = run(ctx, []string{"-db", path}, env, logs)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("run did not finish after cancellation")
		}
	})
	select {
	case <-logs.ready:
	case <-done:
		t.Fatalf("run exited before startup: code=%d logs=%s", exitCode, logs.String())
	case <-time.After(3 * time.Second):
		t.Fatal("server did not become ready")
	}
	if logs.addr != addr || logs.links != 2 {
		t.Fatalf("startup addr=%q links=%d", logs.addr, logs.links)
	}

	var clients sync.WaitGroup
	for range 8 {
		clients.Add(1)
		go func() {
			defer clients.Done()
			conn, err := net.DialTimeout("tcp", logs.addr, time.Second)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Error(err)
				return
			}
			reader := bufio.NewReader(conn)
			for _, tc := range []struct{ command, want string }{
				{"RESOLVE known", "OK https://example.com/a\n"},
				{"RESOLVE known", "OK https://example.com/a\n"},
				{"RESOLVE missing", "ERR not_found\n"},
				{"RESOLVE old", "ERR not_found\n"},
				{"PING", "PONG\n"},
			} {
				if _, err := fmt.Fprintln(conn, tc.command); err != nil {
					t.Error(err)
					return
				}
				if got, err := reader.ReadString('\n'); err != nil || got != tc.want {
					t.Errorf("%s: got %q, %v; want %q", tc.command, got, err, tc.want)
					return
				}
			}
		}()
	}
	clients.Wait()

	// Keep one accepted connection idle while cancellation initiates shutdown.
	idle, err := net.DialTimeout("tcp", logs.addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	if err := idle.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(idle, "PING\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(idle)
	if got, err := reader.ReadString('\n'); err != nil || got != "PONG\n" {
		t.Fatalf("idle client handshake: %q, %v", got, err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("run did not shut down promptly")
	}
	if exitCode != 0 || !strings.Contains(logs.String(), `"msg":"resolver stopped"`) {
		t.Errorf("code=%d logs=%s", exitCode, logs.String())
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		t.Errorf("idle connection after shutdown: %v; want EOF", err)
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Error("server still accepts connections after run returns")
	}
	requireNoCredentials(t, logs.String(), env)
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Errorf("service emitted a non-JSON log: %q", line)
		}
	}
}
