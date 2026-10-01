// Package api exposes the backend's REST API: search, downloads, and auth.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"homecinema/internal/authn"
	"homecinema/internal/downloads"
	"homecinema/internal/search"
)

type server struct {
	downloads *downloads.Service
	search    *search.Service
	auth      *authn.Authenticator
	logger    *slog.Logger
}

// NewRouter builds the HTTP handler for the backend API. Every /api route
// except /api/auth/login requires a valid JWT (see authn.Authenticator).
func NewRouter(downloadsSvc *downloads.Service, searchSvc *search.Service, auth *authn.Authenticator, logger *slog.Logger) http.Handler {
	s := &server{downloads: downloadsSvc, search: searchSvc, auth: auth, logger: logger}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))

	r.Get("/health", s.handleHealth)
	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", s.handleLogin)

		r.Group(func(r chi.Router) {
			r.Use(s.auth.Middleware)
			r.Get("/search", s.handleSearch)
			r.Get("/downloads", s.handleListDownloads)
			r.Post("/downloads", s.handleCreateDownload)
			r.Get("/downloads/{id}", s.handleGetDownload)
			r.Delete("/downloads/{id}", s.handleDeleteDownload)
		})
	})

	return r
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
