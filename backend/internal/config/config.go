// Package config loads runtime configuration from environment variables.
package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"homecinema/internal/authn"
)

// Config holds all runtime settings for the backend service.
type Config struct {
	DatabaseURL string
	HTTPPort    string

	Auth authn.Config
	// JWTSecretGenerated reports whether Auth.Secret was auto-generated
	// because JWT_SECRET was unset, so the caller can warn about it once.
	JWTSecretGenerated bool

	TMDBAPIKey string

	JellyfinURL    string
	JellyfinAPIKey string

	TransmissionURL      string
	TransmissionUser     string
	TransmissionPassword string

	RutrackerLogin    string
	RutrackerPassword string
	// FlareSolverrURL is the FlareSolverr sidecar's /v1 endpoint, used to
	// clear rutracker.org's Cloudflare challenge on login.
	FlareSolverrURL string

	SMBMountPath string

	SyncInterval time.Duration
}

// Load reads configuration from the environment, applying sane defaults for
// anything not explicitly set.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),

		TMDBAPIKey: os.Getenv("TMDB_API_KEY"),

		JellyfinURL:    os.Getenv("JELLYFIN_URL"),
		JellyfinAPIKey: os.Getenv("JELLYFIN_API_KEY"),

		TransmissionURL:      getEnv("TRANSMISSION_URL", "http://192.168.1.1:9091/transmission/rpc"),
		TransmissionUser:     os.Getenv("TRANSMISSION_USER"),
		TransmissionPassword: os.Getenv("TRANSMISSION_PASSWORD"),

		RutrackerLogin:    os.Getenv("RUTRACKER_LOGIN"),
		RutrackerPassword: os.Getenv("RUTRACKER_PASSWORD"),
		FlareSolverrURL:   getEnv("FLARESOLVERR_URL", "http://flaresolverr:8191/v1"),

		SMBMountPath: getEnv("SMB_MOUNT_PATH", "/mnt/downloads"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}

	passwordHash := os.Getenv("APP_PASSWORD_HASH")
	if passwordHash == "" {
		return Config{}, fmt.Errorf("config: APP_PASSWORD_HASH is required")
	}

	tokenTTL, err := getDuration("JWT_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	syncInterval, err := getDuration("SYNC_INTERVAL", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.SyncInterval = syncInterval

	secretHex := os.Getenv("JWT_SECRET")
	var secret []byte
	if secretHex == "" {
		secret, err = randomBytes(32)
		if err != nil {
			return Config{}, fmt.Errorf("config: generate JWT secret: %w", err)
		}
		cfg.JWTSecretGenerated = true
	} else {
		secret = []byte(secretHex)
	}

	cfg.Auth = authn.Config{
		PasswordHash: passwordHash,
		Secret:       secret,
		TokenTTL:     tokenTTL,
	}

	return cfg, nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s=%q: %w", key, v, err)
	}
	return d, nil
}
