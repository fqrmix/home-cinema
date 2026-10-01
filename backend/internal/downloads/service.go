package downloads

import (
	"context"
	"fmt"

	"homecinema/internal/domain"
)

// Repository is the subset of postgres.Repository the Service needs.
type Repository interface {
	CreateDownload(ctx context.Context, d domain.Download) (domain.Download, error)
	GetDownload(ctx context.Context, id string) (domain.Download, error)
	ListDownloads(ctx context.Context) ([]domain.Download, error)
	SetTransmissionHash(ctx context.Context, id, hash string) error
	DeleteDownload(ctx context.Context, id string) error
}

// Service implements the download lifecycle: hand a chosen search result to
// Transmission, and persist the resulting record.
type Service struct {
	repo         Repository
	transmission *TransmissionClient
}

func NewService(repo Repository, transmission *TransmissionClient) *Service {
	return &Service{repo: repo, transmission: transmission}
}

// Create starts a new download: it records the choice, hands it to
// Transmission, and stores the resulting torrent hash.
//
// Exactly one of magnetURL or torrentFileBytes must be set: rutor results
// are plain magnet links Transmission can resolve itself, while rutracker
// results require us to fetch the authenticated .torrent file first (see
// search.Service.Resolve) and hand Transmission the bytes directly.
func (s *Service) Create(ctx context.Context, source domain.Source, sourceTitle, sourceURL, magnetURL string, torrentFileBytes []byte) (domain.Download, error) {
	d, err := s.repo.CreateDownload(ctx, domain.Download{
		Source:             source,
		SourceTitle:        sourceTitle,
		MagnetOrTorrentURL: sourceURL,
		Status:             domain.StatusQueued,
	})
	if err != nil {
		return domain.Download{}, fmt.Errorf("downloads: create record: %w", err)
	}

	var hash string
	if len(torrentFileBytes) > 0 {
		hash, err = s.transmission.AddTorrentFile(ctx, torrentFileBytes)
	} else {
		hash, err = s.transmission.AddTorrent(ctx, magnetURL)
	}
	if err != nil {
		_ = s.repo.DeleteDownload(ctx, d.ID)
		return domain.Download{}, fmt.Errorf("downloads: add to transmission: %w", err)
	}

	if err := s.repo.SetTransmissionHash(ctx, d.ID, hash); err != nil {
		return domain.Download{}, fmt.Errorf("downloads: record transmission hash: %w", err)
	}
	d.TransmissionHash = hash
	d.Status = domain.StatusDownloading
	return d, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Download, error) {
	return s.repo.ListDownloads(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Download, error) {
	return s.repo.GetDownload(ctx, id)
}

// Delete removes a download's Transmission entry (best-effort — if it is
// already gone from Transmission, we still drop our own record) and its
// database record.
func (s *Service) Delete(ctx context.Context, id string) error {
	d, err := s.repo.GetDownload(ctx, id)
	if err != nil {
		return err
	}
	if d.TransmissionHash != "" {
		_ = s.transmission.RemoveTorrent(ctx, d.TransmissionHash, false)
	}
	return s.repo.DeleteDownload(ctx, id)
}
