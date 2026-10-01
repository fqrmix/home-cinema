package api

import "net/http"

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		writeError(w, s.logger, http.StatusBadRequest, "query is required", nil)
		return
	}

	results := s.search.Search(r.Context(), query)
	writeJSON(w, http.StatusOK, results)
}
