package tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/transport"
)

var _ transport.Server = (*Server)(nil)

var ErrStarted = errors.New("tcp: server was already started")

const defaultIdleTimeout = 60 * time.Second
const keepAlivePeriod = 30 * time.Second

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
