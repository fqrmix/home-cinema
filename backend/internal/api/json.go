package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, logger *slog.Logger, status int, msg string, err error) {
	if err != nil {
		logger.Warn("api: request failed", "status", status, "msg", msg, "error", err)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
