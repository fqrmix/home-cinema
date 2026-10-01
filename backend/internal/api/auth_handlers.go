package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"homecinema/internal/authn"
)

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.logger, http.StatusBadRequest, "invalid request body", err)
		return
	}

	token, expiresAt, err := s.auth.Login(req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, authn.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		writeError(w, s.logger, status, "invalid password", err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token, ExpiresAt: expiresAt.Format(http.TimeFormat)})
}
