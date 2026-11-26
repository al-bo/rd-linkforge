package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"
	"unicode"
)

const resolveTimeout = 5 * time.Second
const defaultResolverAddr = "127.0.0.1:7070"

func runResolve(ctx context.Context, args []string, logger *slog.Logger, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("resolve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", defaultResolverAddr, "Resolver TCP address")
	flags.Usage = func() {
		if _, err := fmt.Fprintf(stderr, "Usage: linkctl resolve [-addr HOST:PORT] CODE\n\n"); err != nil {
			logger.ErrorContext(ctx, "write resolve usage failed", slog.Any("error", err))
		}
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 1 {
		logger.ErrorContext(ctx, "resolve requires exactly one code")
		flags.Usage()
		return 2
	}
	code := flags.Arg(0)
	if code == "" || strings.IndexFunc(code, unicode.IsSpace) >= 0 {
		logger.ErrorContext(ctx, "resolve code must be nonempty and contain no whitespace")
		flags.Usage()
		return 2
	}

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", *addr)
	if err != nil {
		logger.ErrorContext(ctx, "connect to resolver failed", slog.Any("error", err))
		return 1
	}
	defer func() {
		_ = conn.Close()
	}()

	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stop()

	if _, err := io.WriteString(conn, "RESOLVE "+code+"\n"); err != nil {
		logger.ErrorContext(ctx, "write resolve request failed", slog.Any("error", err))
		return 1
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		logger.ErrorContext(ctx, "read resolver response failed", slog.Any("error", err))
		return 1
	}

	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")

	if target, ok := strings.CutPrefix(line, "OK "); ok && target != "" {
		if _, err := fmt.Fprintln(stdout, target); err != nil {
			logger.ErrorContext(ctx, "write resolve result failed", slog.Any("error", err))
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprintf(stderr, "resolver answered: %s\n", line); err != nil {
		logger.ErrorContext(ctx, "write resolver error failed", slog.Any("error", err))
	}
	return 1
}
