// Command server is the composition root for the home-cinema backend: it
// wires together storage, the tracker/Transmission/TMDB/Jellyfin
// integrations, and the HTTP API, then runs them until told to shut down.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homecinema/internal/api"
	"homecinema/internal/authn"
	"homecinema/internal/config"
	"homecinema/internal/domain"
	"homecinema/internal/downloads"
	"homecinema/internal/jellyfin"
	"homecinema/internal/metadata"
	"homecinema/internal/scheduler"
	"homecinema/internal/search"
	"homecinema/internal/storage/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("server: fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.JWTSecretGenerated {
		logger.Warn("server: JWT_SECRET not set, generated a random one for this run; existing sessions will not survive a restart")
	}

	if err := postgres.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}
	logger.Info("server: migrations applied")

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repo := postgres.NewRepository(pool)

	transmissionClient := downloads.NewTransmissionClient(cfg.TransmissionURL, cfg.TransmissionUser, cfg.TransmissionPassword)
	downloadsService := downloads.NewService(repo, transmissionClient)

	rutorClient := search.NewRutorClient()
	rutrackerClient := search.NewRutrackerClient(cfg.FlareSolverrURL, cfg.RutrackerLogin, cfg.RutrackerPassword)
	searchService := search.NewService(rutorClient, rutrackerClient, logger)

	metadataClient := metadata.NewClient(cfg.TMDBAPIKey, repo, logger)
	jellyfinClient := jellyfin.NewClient(cfg.JellyfinURL, cfg.JellyfinAPIKey)

	onCompleted := func(ctx context.Context, d domain.Download) {
		metadataClient.Enrich(ctx, d)
		if err := jellyfinClient.RefreshLibrary(ctx); err != nil {
			logger.Error("server: jellyfin refresh failed", "download_id", d.ID, "error", err)
		}
	}
	syncer := downloads.NewSyncer(repo, transmissionClient, onCompleted, logger)

	sched := scheduler.New(logger,
		scheduler.Job{Name: "transmission-sync", Interval: cfg.SyncInterval, Run: syncer.Run},
	)

	authenticator := authn.New(cfg.Auth)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           api.NewRouter(downloadsService, searchService, authenticator, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server: http listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go sched.Start(ctx)

	select {
	case <-ctx.Done():
		logger.Info("server: shutting down")
	case err := <-errCh:
		stop()
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
