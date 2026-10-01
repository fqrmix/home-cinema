package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"homecinema/internal/domain"
)

// ErrNotFound is returned when a lookup by ID matches no row.
var ErrNotFound = errors.New("postgres: not found")

// Repository is the schema small enough (two tables) that hand-written pgx
// queries are simpler than adding a codegen step (sqlc) for it.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreateDownload inserts a new download in the "queued" state.
func (r *Repository) CreateDownload(ctx context.Context, d domain.Download) (domain.Download, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO downloads (source, source_title, magnet_or_torrent_url, transmission_hash, status, download_dir)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, source, source_title, magnet_or_torrent_url, transmission_hash, status,
		          progress_percent, download_dir, error_message, created_at, updated_at`,
		string(d.Source), d.SourceTitle, d.MagnetOrTorrentURL, nullIfEmpty(d.TransmissionHash), string(d.Status), nullIfEmpty(d.DownloadDir))
	return scanDownload(row)
}

// GetDownload fetches a single download by ID, without metadata.
func (r *Repository) GetDownload(ctx context.Context, id string) (domain.Download, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, source, source_title, magnet_or_torrent_url, transmission_hash, status,
		       progress_percent, download_dir, error_message, created_at, updated_at
		FROM downloads WHERE id = $1`, id)
	d, err := scanDownload(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Download{}, ErrNotFound
	}
	return d, err
}

// ListDownloads returns all downloads, newest first, each joined with its
// metadata if present.
func (r *Repository) ListDownloads(ctx context.Context) ([]domain.Download, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.source, d.source_title, d.magnet_or_torrent_url, d.transmission_hash, d.status,
		       d.progress_percent, d.download_dir, d.error_message, d.created_at, d.updated_at,
		       m.download_id, m.tmdb_id, m.title, m.original_title, m.overview, m.poster_path,
		       m.release_year, m.vote_average
		FROM downloads d
		LEFT JOIN movie_metadata m ON m.download_id = d.id
		ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list downloads: %w", err)
	}
	defer rows.Close()

	var out []domain.Download
	for rows.Next() {
		d, meta, err := scanDownloadWithMetadata(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan download: %w", err)
		}
		d.Metadata = meta
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListActiveDownloads returns downloads whose status is not yet terminal,
// for the Transmission sync poller.
func (r *Repository) ListActiveDownloads(ctx context.Context) ([]domain.Download, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, source, source_title, magnet_or_torrent_url, transmission_hash, status,
		       progress_percent, download_dir, error_message, created_at, updated_at
		FROM downloads
		WHERE status NOT IN ('completed', 'error') AND transmission_hash IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("postgres: list active downloads: %w", err)
	}
	defer rows.Close()

	var out []domain.Download
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan download: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetTransmissionHash records the torrent hash returned by Transmission and
// moves the download from "queued" into "downloading".
func (r *Repository) SetTransmissionHash(ctx context.Context, id, hash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE downloads SET transmission_hash = $2, status = 'downloading', updated_at = now() WHERE id = $1`,
		id, hash)
	return err
}

// UpdateProgress updates the status and progress of a download.
func (r *Repository) UpdateProgress(ctx context.Context, id string, status domain.Status, progressPercent float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE downloads SET status = $2, progress_percent = $3, updated_at = now() WHERE id = $1`,
		id, string(status), progressPercent)
	return err
}

// SetError marks a download as failed with a message.
func (r *Repository) SetError(ctx context.Context, id string, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE downloads SET status = 'error', error_message = $2, updated_at = now() WHERE id = $1`,
		id, message)
	return err
}

// DeleteDownload removes a download and its metadata (via cascade).
func (r *Repository) DeleteDownload(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM downloads WHERE id = $1`, id)
	return err
}

// UpsertMetadata stores or replaces the TMDB enrichment for a download.
func (r *Repository) UpsertMetadata(ctx context.Context, m domain.MovieMetadata) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO movie_metadata (download_id, tmdb_id, title, original_title, overview, poster_path, release_year, vote_average)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (download_id) DO UPDATE SET
			tmdb_id = EXCLUDED.tmdb_id,
			title = EXCLUDED.title,
			original_title = EXCLUDED.original_title,
			overview = EXCLUDED.overview,
			poster_path = EXCLUDED.poster_path,
			release_year = EXCLUDED.release_year,
			vote_average = EXCLUDED.vote_average,
			updated_at = now()`,
		m.DownloadID, m.TMDBID, m.Title, m.OriginalTitle, m.Overview, m.PosterPath, m.ReleaseYear, m.VoteAverage)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDownload(row rowScanner) (domain.Download, error) {
	var d domain.Download
	var source, status string
	var transmissionHash, downloadDir, errorMessage *string
	err := row.Scan(&d.ID, &source, &d.SourceTitle, &d.MagnetOrTorrentURL, &transmissionHash, &status,
		&d.ProgressPercent, &downloadDir, &errorMessage, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return domain.Download{}, err
	}
	d.Source = domain.Source(source)
	d.Status = domain.Status(status)
	d.TransmissionHash = derefOrEmpty(transmissionHash)
	d.DownloadDir = derefOrEmpty(downloadDir)
	d.ErrorMessage = derefOrEmpty(errorMessage)
	return d, nil
}

func scanDownloadWithMetadata(row rowScanner) (domain.Download, *domain.MovieMetadata, error) {
	var d domain.Download
	var source, status string
	var transmissionHash, downloadDir, errorMessage *string
	var metaDownloadID, metaTitle, metaOriginalTitle, metaOverview, metaPosterPath *string
	var metaTMDBID, metaReleaseYear *int
	var metaVoteAverage *float64

	err := row.Scan(&d.ID, &source, &d.SourceTitle, &d.MagnetOrTorrentURL, &transmissionHash, &status,
		&d.ProgressPercent, &downloadDir, &errorMessage, &d.CreatedAt, &d.UpdatedAt,
		&metaDownloadID, &metaTMDBID, &metaTitle, &metaOriginalTitle, &metaOverview, &metaPosterPath,
		&metaReleaseYear, &metaVoteAverage)
	if err != nil {
		return domain.Download{}, nil, err
	}
	d.Source = domain.Source(source)
	d.Status = domain.Status(status)
	d.TransmissionHash = derefOrEmpty(transmissionHash)
	d.DownloadDir = derefOrEmpty(downloadDir)
	d.ErrorMessage = derefOrEmpty(errorMessage)

	if metaDownloadID == nil {
		return d, nil, nil
	}
	meta := &domain.MovieMetadata{
		DownloadID:    *metaDownloadID,
		Title:         derefOrEmpty(metaTitle),
		OriginalTitle: derefOrEmpty(metaOriginalTitle),
		Overview:      derefOrEmpty(metaOverview),
		PosterPath:    derefOrEmpty(metaPosterPath),
	}
	if metaTMDBID != nil {
		meta.TMDBID = *metaTMDBID
	}
	if metaReleaseYear != nil {
		meta.ReleaseYear = *metaReleaseYear
	}
	if metaVoteAverage != nil {
		meta.VoteAverage = *metaVoteAverage
	}
	return d, meta, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
