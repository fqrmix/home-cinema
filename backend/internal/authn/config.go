// Package authn protects the API with a single app-wide password and
// JWT-based sessions: POST /api/auth/login exchanges the password for a
// token, which every other /api route then requires.
package authn

import "time"

// Config holds the app password hash and JWT signing settings.
type Config struct {
	// PasswordHash is a bcrypt hash of the single app password.
	PasswordHash string
	Secret       []byte
	TokenTTL     time.Duration
}
