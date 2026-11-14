//go:build ignore

// This standalone manual development utility is excluded from package builds,
// lint and tests. Run explicitly with: go run ./tools/dummy/main.go.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return listenAndServe(ctx, "127.0.0.1:8080", os.Stdout)
}

func listenAndServe(ctx context.Context, address string, out io.Writer) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("iniciar servidor dummy em %s (verifique se a porta está livre): %w", address, err)
	}
	return serve(ctx, listener, out)
}

// serve owns the listener and joins its serving goroutine before returning.
func serve(ctx context.Context, listener net.Listener, out io.Writer) error {
	defer func() { _ = listener.Close() }()
	if ctx.Err() != nil {
		return nil
	}
	server := &http.Server{
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	if _, err := fmt.Fprintf(out, "servidor dummy em http://%s (Ctrl+C para encerrar)\n", listener.Addr()); err != nil {
		return fmt.Errorf("informar endereço do servidor dummy: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		return fmt.Errorf("executar servidor dummy: %w", err)
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			closeErr := server.Close()
			serveErr := <-done
			if errors.Is(serveErr, http.ErrServerClosed) {
				serveErr = nil
			}
			return fmt.Errorf("encerrar servidor dummy: %w", errors.Join(err, closeErr, serveErr))
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("executar servidor dummy: %w", err)
		}
		return nil
	}
}

func newHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = io.WriteString(w, "{\"error\":\"method not allowed\"}\n")
			return
		}
		switch r.URL.Path {
		case "/health":
			_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "{\"error\":\"dummy server error\"}\n")
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "{\"error\":\"not found\"}\n")
		}
	})
}
