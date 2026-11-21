package tcp

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
)

var ErrStarted = errors.New("tcp: server was already started")

// Resolver is what the server needs from the application.
// *link.Service satisfies it without an adapter.
type Resolver interface {
	Resolve(ctx context.Context, code string) (link.Link, error)
}

type Option func(*Server)

// WithIdleTimeout sets how long a connection may stay silent. Default 60 seconds.
// A value that is not positive is ignored.
func WithIdleTimeout(d time.Duration) Option

// WithLogger sets the logger of the server. Default: a logger that discards everything.
// A nil logger is ignored.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		s.logger = l
	}
}

type Server struct { /* your fields */
	logger *slog.Logger
}

func NewServer(addr string, r Resolver, opts ...Option) *Server {
	return &Server{}
}

func (s *Server) Start(ctx context.Context) error {
	return nil
}
func (s *Server) Addr() string {
	return ""
}
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
