// Package domain holds the core types shared across the backend's packages.
package domain

import "time"

// Source identifies which tracker a search result or download came from.
type Source string

const (
	SourceRutracker Source = "rutracker"
	SourceRutor     Source = "rutor"
)

// Status is the lifecycle state of a Download.
type Status string

const (
	StatusQueued      Status = "queued"
	StatusDownloading Status = "downloading"
	StatusSeeding     Status = "seeding"
	StatusCompleted   Status = "completed"
	StatusError       Status = "error"
)

// SearchResult is one row returned by a tracker search, before any download
// has started.
type SearchResult struct {
	Source             Source    `json:"source"`
	Title              string    `json:"title"`
	SizeBytes          int64     `json:"sizeBytes"`
	Seeders            int       `json:"seeders"`
	Leechers           int       `json:"leechers"`
	MagnetOrTorrentURL string    `json:"magnetOrTorrentUrl"`
	PublishDate        time.Time `json:"publishDate"`
}

// Download is a persisted record of a torrent the user chose to fetch.
type Download struct {
	ID                 string         `json:"id"`
	Source             Source         `json:"source"`
	SourceTitle        string         `json:"sourceTitle"`
	MagnetOrTorrentURL string         `json:"magnetOrTorrentUrl"`
	TransmissionHash   string         `json:"transmissionHash,omitempty"`
	Status             Status         `json:"status"`
	ProgressPercent    float64        `json:"progressPercent"`
	DownloadDir        string         `json:"downloadDir,omitempty"`
	ErrorMessage       string         `json:"errorMessage,omitempty"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	Metadata           *MovieMetadata `json:"metadata,omitempty"`
}

// MovieMetadata is the TMDB enrichment for a Download, if a match was found.
type MovieMetadata struct {
	DownloadID    string  `json:"downloadId"`
	TMDBID        int     `json:"tmdbId"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"posterPath"`
	ReleaseYear   int     `json:"releaseYear"`
	VoteAverage   float64 `json:"voteAverage"`
}
