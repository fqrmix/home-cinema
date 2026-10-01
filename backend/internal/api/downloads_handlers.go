package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"homecinema/internal/domain"
	"homecinema/internal/storage/postgres"
)

func (s *server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	list, err := s.downloads.List(r.Context())
	if err != nil {
		writeError(w, s.logger, http.StatusInternalServerError, "failed to list downloads", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleGetDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := s.downloads.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeError(w, s.logger, http.StatusNotFound, "download not found", nil)
			return
		}
		writeError(w, s.logger, http.StatusInternalServerError, "failed to get download", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// handleCreateDownload accepts the domain.SearchResult the user picked from
// GET /api/search (search results aren't persisted server-side, so the
// frontend sends the whole chosen result back), resolves it into whatever
// Transmission needs, and starts the download.
func (s *server) handleCreateDownload(w http.ResponseWriter, r *http.Request) {
	var chosen domain.SearchResult
	if err := json.NewDecoder(r.Body).Decode(&chosen); err != nil {
		writeError(w, s.logger, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if chosen.MagnetOrTorrentURL == "" || chosen.Title == "" {
		writeError(w, s.logger, http.StatusBadRequest, "title and magnetOrTorrentUrl are required", nil)
		return
	}

	magnetURL, torrentFileBytes, err := s.search.Resolve(r.Context(), chosen)
	if err != nil {
		writeError(w, s.logger, http.StatusBadGateway, "failed to resolve torrent", err)
		return
	}

	d, err := s.downloads.Create(r.Context(), chosen.Source, chosen.Title, chosen.MagnetOrTorrentURL, magnetURL, torrentFileBytes)
	if err != nil {
		writeError(w, s.logger, http.StatusBadGateway, "failed to start download", err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (s *server) handleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.downloads.Delete(r.Context(), id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeError(w, s.logger, http.StatusNotFound, "download not found", nil)
			return
		}
		writeError(w, s.logger, http.StatusInternalServerError, "failed to delete download", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
