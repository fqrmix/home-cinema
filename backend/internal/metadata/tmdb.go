// Package metadata enriches a completed download with poster, description,
// year and rating fetched from TMDB, matched against the tracker's raw
// (noisy) release title.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"homecinema/internal/domain"
)

// Repository is the subset of postgres.Repository the Client needs.
type Repository interface {
	UpsertMetadata(ctx context.Context, m domain.MovieMetadata) error
}

const tmdbBaseURL = "https://api.themoviedb.org/3"

type Client struct {
	apiKey     string
	repo       Repository
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(apiKey string, repo Repository, logger *slog.Logger) *Client {
	return &Client{apiKey: apiKey, repo: repo, httpClient: &http.Client{}, logger: logger}
}

// Enrich looks up TMDB for the given download's title and, on a match,
// stores the result. It never returns an error to the caller: a missing or
// wrong match just means the library page falls back to the raw title, and
// enrichment is best retried later rather than blocking the sync loop.
func (c *Client) Enrich(ctx context.Context, d domain.Download) {
	if c.apiKey == "" {
		return
	}

	title, year := cleanTitle(d.SourceTitle)
	if title == "" {
		return
	}

	result, err := c.searchMovie(ctx, title, year)
	if err != nil {
		c.logger.Warn("metadata: tmdb search failed", "download_id", d.ID, "title", title, "error", err)
		return
	}
	if result == nil {
		c.logger.Info("metadata: no tmdb match", "download_id", d.ID, "title", title)
		return
	}

	meta := domain.MovieMetadata{
		DownloadID:    d.ID,
		TMDBID:        result.ID,
		Title:         result.Title,
		OriginalTitle: result.OriginalTitle,
		Overview:      result.Overview,
		PosterPath:    result.PosterPath,
		ReleaseYear:   releaseYear(result.ReleaseDate),
		VoteAverage:   result.VoteAverage,
	}
	if err := c.repo.UpsertMetadata(ctx, meta); err != nil {
		c.logger.Error("metadata: save failed", "download_id", d.ID, "error", err)
	}
}

type tmdbMovie struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	ReleaseDate   string  `json:"release_date"`
	VoteAverage   float64 `json:"vote_average"`
}

func (c *Client) searchMovie(ctx context.Context, title string, year int) (*tmdbMovie, error) {
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("query", title)
	q.Set("language", "ru-RU")
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tmdbBaseURL+"/search/movie?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb: search returned HTTP %d", resp.StatusCode)
	}

	var out struct {
		Results []tmdbMovie `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Results) == 0 {
		return nil, nil
	}
	return &out.Results[0], nil
}

func releaseYear(releaseDate string) int {
	if len(releaseDate) < 4 {
		return 0
	}
	year, _ := strconv.Atoi(releaseDate[:4])
	return year
}

var (
	yearPattern    = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	qualityPattern = regexp.MustCompile(`(?i)\b(BDRip|BRRip|WEB-?DL|WEBRip|HDRip|DVDRip|HDTV|SATRip|1080p|720p|2160p|4K|x264|x265|HEVC|AAC\d*|AC3|DTS|REMUX|Blu-?Ray)\b.*$`)
)

// cleanTitle strips the quality/release-group noise trackers append to
// titles (e.g. "Матрица.1999.BDRip.1080p.x264-GROUP") down to a plain
// search query, plus the release year if one was found.
func cleanTitle(raw string) (title string, year int) {
	if m := yearPattern.FindString(raw); m != "" {
		year, _ = strconv.Atoi(m)
	}

	cleaned := qualityPattern.ReplaceAllString(raw, "")
	cleaned = yearPattern.ReplaceAllString(cleaned, "")
	cleaned = strings.NewReplacer(".", " ", "_", " ").Replace(cleaned)
	cleaned = strings.Trim(cleaned, " -()[]")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned, year
}
