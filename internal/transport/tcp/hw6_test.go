package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"YOUR_MODULE/internal/link"
)

// hw6Resolver answers from a map. A code in the "block" set waits until
// release is closed or the context of the call ends.
type hw6Resolver struct {
	links   map[string]link.Link
	failing string
	block   string
	entered chan struct{}
	release chan struct{}
}

func (r *hw6Resolver) Resolve(ctx context.Context, code string) (link.Link, error) {
	if code == r.block {
		r.entered <- struct{}{}
		select {
		case <-r.release:
		case <-ctx.Done():
			return link.Link{}, ctx.Err()
		}
	}
	if code == r.failing {
		return link.Link{}, errors.New("database is down")
	}
	l, ok := r.links[code]
	if !ok {
		return link.Link{}, fmt.Errorf("resolve %q: %w", code, link.ErrNotFound)
	}
	if code == "old" {
		return link.Link{}, link.ErrExpired
	}
	return l, nil
}

func hw6NewResolver() *hw6Resolver {
	return &hw6Resolver{
		links: map[string]link.Link{
			"abc": {Code: "abc", Target: "https://example.com/a"},
			"go":  {Code: "go", Target: "https://go.dev"},
			"old": {Code: "old", Target: "https://old.example"},
		},
		failing: "broken",
		block:   "slow",
		entered: make(chan struct{}, 10),
		release: make(chan struct{}),
	}
}

func hw6Start(t *testing.T, r Resolver, opts ...Option) *Server {
	t.Helper()
	s := NewServer("127.0.0.1:0", r, opts...)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return s
}

type hw6Client struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func hw6Dial(t *testing.T, addr string) *hw6Client {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return &hw6Client{t: t, conn: conn, r: bufio.NewReader(conn)}
}

func (c *hw6Client) send(line string) {
	c.t.Helper()
	if _, err := io.WriteString(c.conn, line); err != nil {
		c.t.Fatalf("write %q: %v", line, err)
	}
}

// readLine returns the next line without its newline.
func (c *hw6Client) readLine() (string, error) {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := c.r.ReadString('\n')
	return strings.TrimSuffix(line, "\n"), err
}

func (c *hw6Client) ask(line string) string {
	c.t.Helper()
	c.send(line + "\n")
	got, err := c.readLine()
	if err != nil {
		c.t.Fatalf("after %q: %v", line, err)
	}
	return got
}

// wantClosed checks that the server closes the connection without sending more. The
// client sees the end of file, or a reset when the server closed with unread input.
func (c *hw6Client) wantClosed() {
	c.t.Helper()
	line, err := c.readLine()
	var netErr net.Error
	if err == nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		c.t.Fatalf("got %q, %v; want the connection closed", line, err)
	}
}

func hw6Shutdown(t *testing.T, s *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestServerCommands(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	c := hw6Dial(t, s.Addr())

	for _, tc := range []struct{ send, want string }{
		{"PING", "PONG"},
		{"RESOLVE abc", "OK https://example.com/a"},
		{"RESOLVE   go  ", "OK https://go.dev"},
		{"RESOLVE nope", "ERR not_found"},
		{"RESOLVE old", "ERR not_found"},
		{"RESOLVE broken", "ERR internal"},
		{"", "ERR bad_request"},
		{"HELLO", "ERR bad_request"},
		{"ping", "ERR bad_request"},
		{"RESOLVE", "ERR bad_request"},
		{"RESOLVE a b", "ERR bad_request"},
		{"PING extra", "ERR bad_request"},
		{"PING", "PONG"}, // the connection survived every error
	} {
		if got := c.ask(tc.send); got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.send, got, tc.want)
		}
	}
}

func TestServerAcceptsCRLFAndPipelinedCommands(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	c := hw6Dial(t, s.Addr())
	c.send("PING\r\nRESOLVE abc\r\nPING\n")
	for _, want := range []string{"PONG", "OK https://example.com/a", "PONG"} {
		if got, err := c.readLine(); err != nil || strings.TrimSuffix(got, "\r") != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
}

func TestServerQuitClosesTheConnection(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	c := hw6Dial(t, s.Addr())
	c.send("QUIT\n")
	c.wantClosed()
}

func TestServerLineLimit(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())

	// A line of exactly 4096 bytes with its newline is accepted.
	c := hw6Dial(t, s.Addr())
	if got := c.ask(strings.Repeat("a", 4095)); got != "ERR bad_request" {
		t.Errorf("4096-byte line -> %q, want a normal answer", got)
	}

	// A longer line gets an error and the connection is closed.
	c = hw6Dial(t, s.Addr())
	c.send(strings.Repeat("a", 10000) + "\n")
	if got, err := c.readLine(); err != nil || got != "ERR line_too_long" {
		t.Fatalf("long line -> %q, %v; want ERR line_too_long", got, err)
	}
	c.wantClosed()
}

func TestServerClosesIdleConnections(t *testing.T) {
	s := hw6Start(t, hw6NewResolver(), WithIdleTimeout(50*time.Millisecond))
	c := hw6Dial(t, s.Addr())
	if got := c.ask("PING"); got != "PONG" {
		t.Fatalf("PING -> %q", got)
	}
	c.wantClosed() // nothing is sent: the server gives up after the idle timeout
}

func TestServerIgnoresInvalidOptions(t *testing.T) {
	s := hw6Start(t, hw6NewResolver(), WithIdleTimeout(0), WithIdleTimeout(-time.Second), WithLogger(nil))
	c := hw6Dial(t, s.Addr())
	if got := c.ask("PING"); got != "PONG" {
		t.Errorf("PING -> %q", got)
	}
}

func TestServerManyClientsAtOnce(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.Dial("tcp", s.Addr())
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			r := bufio.NewReader(conn)
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			for j := 0; j < 50; j++ {
				if _, err := io.WriteString(conn, "RESOLVE abc\n"); err != nil {
					t.Error(err)
					return
				}
				got, err := r.ReadString('\n')
				if err != nil || got != "OK https://example.com/a\n" {
					t.Errorf("got %q, %v", got, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestServerStartAndAddr(t *testing.T) {
	s := NewServer("127.0.0.1:0", hw6NewResolver())
	if s.Addr() != "127.0.0.1:0" {
		t.Errorf("Addr before Start = %q, want the configured address", s.Addr())
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(s.Addr(), ":0") {
		t.Errorf("Addr after Start = %q, want the real port", s.Addr())
	}
	if err := s.Start(context.Background()); !errors.Is(err, ErrStarted) {
		t.Errorf("second Start = %v, want ErrStarted", err)
	}
	hw6Shutdown(t, s)
	if err := s.Start(context.Background()); !errors.Is(err, ErrStarted) {
		t.Errorf("Start after Shutdown = %v, want ErrStarted", err)
	}
}

func TestServerStartFailsOnBusyPort(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	other := NewServer(s.Addr(), hw6NewResolver())
	if err := other.Start(context.Background()); err == nil {
		hw6Shutdown(t, other)
		t.Fatal("Start on a busy port returned nil")
	}
}

func TestServerShutdownWithoutStart(t *testing.T) {
	s := NewServer("127.0.0.1:0", hw6NewResolver())
	hw6Shutdown(t, s)
}

func TestServerShutdownClosesIdleConnectionsAndStopsListening(t *testing.T) {
	s := hw6Start(t, hw6NewResolver())
	c := hw6Dial(t, s.Addr())
	if got := c.ask("PING"); got != "PONG" {
		t.Fatalf("PING -> %q", got)
	}
	addr := s.Addr()

	hw6Shutdown(t, s) // returns although the client is still connected and silent
	c.wantClosed()
	hw6Shutdown(t, s) // a second call is fine

	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Error("the listener still accepts connections after Shutdown")
	}
}

func TestServerShutdownWaitsForTheCommandInProgress(t *testing.T) {
	r := hw6NewResolver()
	s := hw6Start(t, r)
	c := hw6Dial(t, s.Addr())
	c.send("RESOLVE slow\n")
	<-r.entered // the resolver is working on the command

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- s.Shutdown(ctx)
	}()

	select {
	case err := <-done:
		t.Fatalf("Shutdown returned (%v) while a command was still running", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(r.release)
	if got, err := c.readLine(); err != nil || got != "ERR not_found" {
		t.Fatalf("the command in progress got %q, %v; it must be answered", got, err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	c.wantClosed()
}

func TestServerShutdownTimeoutClosesConnections(t *testing.T) {
	r := hw6NewResolver() // release is never closed: only the context can end the command
	s := hw6Start(t, r)
	c := hw6Dial(t, s.Addr())
	c.send("RESOLVE slow\n")
	<-r.entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := s.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want context.DeadlineExceeded", err)
	}
	c.wantClosed()
	// The context of the stuck Resolve was cancelled, so everything ends and a second call succeeds.
	hw6Shutdown(t, s)
}

func TestServerShutdownWithBusyClients(t *testing.T) {
	for round := 0; round < 30; round++ {
		s := NewServer("127.0.0.1:0", hw6NewResolver())
		if err := s.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := net.Dial("tcp", s.Addr())
				if err != nil {
					return // the server may already be down
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				r := bufio.NewReader(conn)
				for {
					if _, err := io.WriteString(conn, "PING\n"); err != nil {
						return
					}
					if _, err := r.ReadString('\n'); err != nil {
						return
					}
				}
			}()
		}
		hw6Shutdown(t, s) // must not wait for the idle timeout of any connection
		wg.Wait()
	}
}
