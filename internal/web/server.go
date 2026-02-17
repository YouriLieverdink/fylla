package web

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"
)

// Handlers holds the API handler functions provided by the caller.
type Handlers struct {
	Today    http.HandlerFunc
	Tasks    http.HandlerFunc
	Schedule http.HandlerFunc
	Status   http.HandlerFunc
}

// Server serves the Fylla web dashboard.
type Server struct {
	handlers Handlers
	port     int
}

// NewServer creates a new dashboard server.
func NewServer(handlers Handlers, port int) *Server {
	return &Server{handlers: handlers, port: port}
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/today", s.handlers.Today)
	mux.HandleFunc("GET /api/tasks", s.handlers.Tasks)
	mux.HandleFunc("GET /api/schedule", s.handlers.Schedule)
	mux.HandleFunc("GET /api/status", s.handlers.Status)

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticSub))

	// Serve index.html for SPA page routes; delegate to file server for real files.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/timeline", "/tasks", "/schedule", "/status":
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		default:
			fileServer.ServeHTTP(w, r)
		}
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	fmt.Printf("Dashboard running at http://localhost:%d\n", s.port)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
