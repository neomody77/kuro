package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/neomody77/kuro/internal/auth"
	"github.com/neomody77/kuro/internal/config"
)

type Server struct {
	cfg    *config.Config
	mux    *http.ServeMux
	srv    *http.Server
	tokens map[string]string
}

func New(cfg *config.Config, tokens map[string]string) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		tokens: tokens,
	}
	return s
}

// HandleAPI registers a handler under /api/ with auth middleware applied.
func (s *Server) HandleAPI(pattern string, handler http.HandlerFunc) {
	authMW := auth.Middleware(s.tokens)
	s.mux.Handle(pattern, authMW(withUserDir(s.cfg.DataDir, handler)))
}

// ServeUI serves embedded frontend assets at / with SPA fallback.
func (s *Server) ServeUI(assets fs.FS) {
	fileServer := http.FileServer(http.FS(assets))
	s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file directly first.
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Check if the file exists in the embedded FS.
		f, err := assets.Open(path[1:]) // strip leading /
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for client-side routes.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}))
}

// ListenAndServe starts the HTTP server and blocks until shutdown signal.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.Addr()
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      withCORS(s.mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Kuro starting on %s", addr)
		errCh <- s.srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("Received %v, shutting down...", sig)
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

// withUserDir ensures the user directory exists before handling the request.
func withUserDir(dataDir string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := auth.GetUser(r.Context())
		if username == "" {
			WriteError(w, http.StatusUnauthorized, "no user in context")
			return
		}
		if _, err := auth.EnsureUserDir(dataDir, username); err != nil {
			log.Printf("ERROR: creating user dir for %q: %v", username, err)
			WriteError(w, http.StatusInternalServerError, "failed to create user directory")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withCORS adds CORS headers for development.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}
