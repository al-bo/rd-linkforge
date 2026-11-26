package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/transport/tcp"
)

type resolveTestResolver struct{}

func (resolveTestResolver) Resolve(_ context.Context, code string) (link.Link, error) {
	switch code {
	case "known":
		return link.Link{Code: code, Target: "https://example.com/a?q=1"}, nil
	case "old":
		return link.Link{}, link.ErrExpired
	case "broken":
		return link.Link{}, errors.New("private database failure")
	default:
		return link.Link{}, link.ErrNotFound
	}
}

func startResolveServer(t *testing.T) *tcp.Server {
	t.Helper()
	s := tcp.NewServer("127.0.0.1:0", resolveTestResolver{})
	if err := s.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestResolveCommand(t *testing.T) {
	s := startResolveServer(t)
	for _, tc := range []struct {
		code       string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{"known", 0, "https://example.com/a?q=1\n", ""},
		{"missing", 1, "", "resolver answered: ERR not_found\n"},
		{"old", 1, "", "resolver answered: ERR not_found\n"},
		{"broken", 1, "", "resolver answered: ERR internal\n"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			code, stdout, stderr := runCommand(t, "resolve", "-addr", s.Addr(), tc.code)
			if code != tc.wantExit || stdout != tc.wantStdout || stderr != tc.wantStderr {
				t.Errorf("code=%d stdout=%q stderr=%q; want %d, %q, %q", code, stdout, stderr, tc.wantExit, tc.wantStdout, tc.wantStderr)
			}
		})
	}
}

func TestResolveUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing code", nil},
		{"too many codes", []string{"one", "two"}},
		{"empty code", []string{""}},
		{"space", []string{"a b"}},
		{"tab", []string{"a\tb"}},
		{"newline", []string{"a\nPING"}},
		{"carriage return", []string{"a\rb"}},
		{"unicode whitespace", []string{"a\u00a0b"}},
		{"unknown flag", []string{"-unknown"}},
		{"missing address value", []string{"-addr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(t, append([]string{"resolve"}, tc.args...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "Usage: linkctl resolve [-addr HOST:PORT] CODE") {
				t.Errorf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestResolveUnavailableServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCommand(t, "resolve", "-addr", addr, "known")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "connect to resolver failed") {
		t.Errorf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// A nil response leaves the client waiting until its context expires. Waiting
// for EOF also checks that the command closes its connection on return.
func startResolvePeer(t *testing.T, response *string) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	requests := make(chan string, 1)
	done := make(chan struct{})
	var peerErr error
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			peerErr = err
			return
		}
		defer conn.Close()
		if peerErr = conn.SetDeadline(time.Now().Add(3 * time.Second)); peerErr != nil {
			return
		}
		reader := bufio.NewReader(conn)
		request, err := reader.ReadString('\n')
		if err != nil {
			peerErr = err
			return
		}
		requests <- request
		if response != nil {
			if _, peerErr = io.WriteString(conn, *response); peerErr != nil {
				return
			}
		}
		_, peerErr = io.Copy(io.Discard, reader)
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
			if peerErr != nil {
				t.Errorf("test peer: %v", peerErr)
			}
		case <-time.After(4 * time.Second):
			t.Error("test peer did not finish")
		}
	})
	return listener.Addr().String(), requests
}

func TestResolveCancelsBlockedRead(t *testing.T) {
	addr, requests := startResolvePeer(t, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	var stdout, stderr bytes.Buffer
	started := time.Now()
	code := run(ctx, []string{"resolve", "-addr", addr, "known"}, loggingEnv(t, nil), &stdout, &stderr)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("read ignored parent deadline: elapsed=%v", elapsed)
	}
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "read resolver response failed") {
		t.Errorf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	select {
	case request := <-requests:
		if request != "RESOLVE known\n" {
			t.Errorf("request=%q", request)
		}
	default:
		t.Error("command did not reach the read before cancellation")
	}
}

func TestResolveResponseParsing(t *testing.T) {
	for _, tc := range []struct {
		name, response, stdout, stderr string
		code                           int
	}{
		{"CRLF", "OK https://go.dev\r\n", "https://go.dev\n", "", 0},
		{"empty target", "OK \n", "", "resolver answered: OK \n", 1},
		{"unexpected reply", "HELLO\n", "", "resolver answered: HELLO\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, _ := startResolvePeer(t, &tc.response)
			code, stdout, stderr := runCommand(t, "resolve", "-addr", addr, "known")
			if code != tc.code || stdout != tc.stdout || stderr != tc.stderr {
				t.Errorf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}
