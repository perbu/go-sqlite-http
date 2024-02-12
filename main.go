package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func NewServer(logger *slog.Logger, conn *sqlitex.Pool) http.Handler {
	mux := http.NewServeMux()
	addRoutes(
		mux,
		logger,
		conn,
	)
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger, handler)
	return handler
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err := run(ctx, os.Stdout, os.Args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, w io.Writer, args []string) error {
	lh := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(lh)
	pool, err := sqlitex.NewPool("database.sqlite", sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate,
		PoolSize: 5,
	})
	if err != nil {
		return fmt.Errorf("sqlite.OpenConn: %w", err)
	}
	defer pool.Close()

	server := NewServer(logger, pool)
	srv := &http.Server{
		Addr:    ":8080",
		Handler: server,
	}
	go func(s *http.Server) {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}(srv)

	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http.ListenAndServe: %w", err)
	}
	return nil
}
