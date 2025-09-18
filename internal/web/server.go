package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"spotiseek/pkg/models"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	config *models.Config
	port   int
	server *http.Server
}

func NewServer(config *models.Config, port int) *Server {
	return &Server{
		config: config,
		port:   port,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/status", s.corsMiddleware(s.handleStatus))
	mux.HandleFunc("/api/watch", s.corsMiddleware(s.handleWatch))
	mux.HandleFunc("/api/forget/", s.corsMiddleware(s.handleForget))

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to create static file system: %w", err)
	}

	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.loggingMiddleware(mux),
	}

	log.Printf("Starting web server on port %d", s.port)
	log.Printf("Access the web interface at: http://0.0.0.0:%d", s.port)

	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, NewErrorResponse(message))
}