package authn

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("authn: invalid password")
	ErrInvalidToken       = errors.New("authn: invalid or expired token")
)

// Authenticator issues and verifies JWT sessions for the single configured
// app password.
type Authenticator struct {
	cfg Config
}

func New(cfg Config) *Authenticator {
	return &Authenticator{cfg: cfg}
}

// Login checks the password and, if valid, issues a signed JWT.
func (a *Authenticator) Login(password string) (token string, expiresAt time.Time, err error) {
	if bcrypt.CompareHashAndPassword([]byte(a.cfg.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, ErrInvalidCredentials
	}

	expiresAt = time.Now().Add(a.cfg.TokenTTL)
	claims := jwt.RegisteredClaims{
		Subject:   "app",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.cfg.Secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (a *Authenticator) verify(tokenString string) error {
	_, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) {
		return a.cfg.Secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return ErrInvalidToken
	}
	return nil
}

// Middleware rejects any request without a valid "Authorization: Bearer
// <token>" header.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, prefix) || a.verify(strings.TrimPrefix(h, prefix)) != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"authentication required"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
