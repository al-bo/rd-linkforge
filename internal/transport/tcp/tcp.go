package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/transport"
)

var _ transport.Server = (*Server)(nil)

var ErrStarted = errors.New("tcp: server was already started")

const errBadRequest = "ERR bad_request\n"
const errRequestLong = "ERR line_too_long\n"
const errNotFound = "ERR not_found\n"
const errInternal = "ERR internal\n"

const defaultIdleTimeout = 60 * time.Second
const keepAlivePeriod = 30 * time.Second
const maxLineSize = 4096

// Resolver is what the server needs from the application.
// *link.Service satisfies it without an adapter.
type Resolver interface {
	Resolve(ctx context.Context, code string) (link.Link, error)
}

type Option func(*Server)

// WithIdleTimeout sets how long a connection may stay silent. Default 60 seconds.
// A value that is not positive is ignored.
func WithIdleTimeout(d time.Duration) Option {
	return func(s *Server) {
		if d > 0 {
			s.idleTimeout = d
		}
	}
}

// WithLogger sets the logger of the server. Default: a logger that discards everything.
// A nil logger is ignored.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		if l != nil {
			s.logger = l
		}
	}
}

type Server struct {
	logger      *slog.Logger
	addr        string
	resolver    Resolver
	idleTimeout time.Duration
	mu          sync.Mutex
	listener    net.Listener
	started     bool
	closed      bool
	wg          sync.WaitGroup
	conns       map[net.Conn]struct{}
	cancel      context.CancelFunc
}

func NewServer(addr string, r Resolver, opts ...Option) *Server {
	server := &Server{
		idleTimeout: defaultIdleTimeout,
		addr:        addr,
		resolver:    r,
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		conns:       make(map[net.Conn]struct{}),
	}

	for _, opt := range opts {
		opt(server)
	}

	return server
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.closed {
		return ErrStarted
	}
	s.started = true

	lc := net.ListenConfig{
		KeepAlive: keepAlivePeriod,
	}

	listener, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("tcp: listen %q: %w", s.addr, err)
	}

	s.listener = listener

	resolveCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go s.acceptLoop(resolveCtx, listener)

	return nil
}

func (s *Server) acceptLoop(ctx context.Context, listener net.Listener) {
	defer s.wg.Done()

	for {
		conn, err := listener.Accept()

		if err != nil {
			s.mu.Lock()

			closed := s.closed
			s.mu.Unlock()

			if !closed {
				s.logger.Error("tcp: accept failed", "error", err)
			}
			return
		}

		s.mu.Lock()

		if s.closed {
			s.mu.Unlock()
			conn.Close()
			return
		}

		s.conns[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)

	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}

		err := conn.SetReadDeadline(time.Now().Add(s.idleTimeout))
		s.mu.Unlock()

		if err != nil {
			s.logger.Error("tcp: set read deadline failed", "error", err)
			return
		}

		if !scanner.Scan() {
			break
		}

		fields := strings.Fields(scanner.Text())
		response := errBadRequest

		switch {
		case len(fields) == 1 && fields[0] == "PING":
			response = "PONG\n"
		case len(fields) == 1 && fields[0] == "QUIT":
			return
		case len(fields) == 2 && fields[0] == "RESOLVE":
			resolved, err := s.resolver.Resolve(ctx, fields[1])

			if err == nil {
				response = "OK " + resolved.Target + "\n"
			} else if errors.Is(err, link.ErrNotFound) || errors.Is(err, link.ErrExpired) {
				response = errNotFound
			} else {
				response = errInternal
				s.logger.Error("resolver: resolve failed", "error", err)
			}
		}

		if _, err := io.WriteString(conn, response); err != nil {
			s.logger.Error("tcp: write response failed", "error", err)
			return
		}
	}

	var netErr net.Error
	err := scanner.Err()

	if err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			if _, err := io.WriteString(conn, errRequestLong); err != nil {
				s.logger.Error("tcp: write response failed", "error", err)
				return
			}
		} else if errors.As(err, &netErr) && netErr.Timeout() {
			return
		} else {
			s.logger.Error("tcp: read failed", "error", err)
		}
	}
}

func (s *Server) Addr() string {
	s.mu.Lock()

	defer s.mu.Unlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}

	return s.addr
}
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
