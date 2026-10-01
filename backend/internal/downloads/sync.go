package downloads

import (
	"context"
	"log/slog"

	"homecinema/internal/domain"
)

// SyncRepository is the subset of postgres.Repository the Syncer needs.
type SyncRepository interface {
	ListActiveDownloads(ctx context.Context) ([]domain.Download, error)
	UpdateProgress(ctx context.Context, id string, status domain.Status, progressPercent float64) error
}

// Syncer polls Transmission for the status of every non-terminal download
// and mirrors it into our own database, since transmission-daemon on the
// router has no webhook mechanism we can register against.
type Syncer struct {
	repo         SyncRepository
	transmission *TransmissionClient
	// OnCompleted runs once, synchronously, the first time a download's
	// status flips to "completed" — used to trigger TMDB enrichment and a
	// Jellyfin library scan. Errors are logged, not propagated: a failed
	// enrichment must not block future sync ticks.
	onCompleted func(ctx context.Context, d domain.Download)
	logger      *slog.Logger
}

func NewSyncer(repo SyncRepository, transmission *TransmissionClient, onCompleted func(ctx context.Context, d domain.Download), logger *slog.Logger) *Syncer {
	return &Syncer{repo: repo, transmission: transmission, onCompleted: onCompleted, logger: logger}
}

// Run performs one sync pass. It matches scheduler.Job's Run signature.
func (s *Syncer) Run(ctx context.Context) error {
	active, err := s.repo.ListActiveDownloads(ctx)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return nil
	}

	hashes := make([]string, 0, len(active))
	for _, d := range active {
		hashes = append(hashes, d.TransmissionHash)
	}

	statuses, err := s.transmission.GetStatuses(ctx, hashes)
	if err != nil {
		return err
	}
	byHash := make(map[string]TorrentStatus, len(statuses))
	for _, st := range statuses {
		byHash[st.HashString] = st
	}

	for _, d := range active {
		st, ok := byHash[d.TransmissionHash]
		if !ok {
			// Torrent is no longer known to Transmission (removed outside
			// our control) — leave the record as-is rather than guessing.
			continue
		}

		newStatus := domain.StatusDownloading
		if st.PercentDone >= 1 {
			newStatus = domain.StatusCompleted
		}
		progress := st.PercentDone * 100

		if newStatus == d.Status && progress == d.ProgressPercent {
			continue
		}
		if err := s.repo.UpdateProgress(ctx, d.ID, newStatus, progress); err != nil {
			s.logger.Error("sync: update progress failed", "download_id", d.ID, "error", err)
			continue
		}

		if newStatus == domain.StatusCompleted && d.Status != domain.StatusCompleted && s.onCompleted != nil {
			d.Status = newStatus
			d.ProgressPercent = progress
			s.onCompleted(ctx, d)
		}
	}

	return nil
}
