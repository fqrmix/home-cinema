package search

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"homecinema/internal/domain"
)

// Service merges results from both trackers into one ranked list, and
// resolves a chosen result into whatever Transmission needs to start it.
type Service struct {
	rutor     *RutorClient
	rutracker *RutrackerClient
	logger    *slog.Logger
}

func NewService(rutor *RutorClient, rutracker *RutrackerClient, logger *slog.Logger) *Service {
	return &Service{rutor: rutor, rutracker: rutracker, logger: logger}
}

// Search queries both trackers concurrently. A failure in one source (e.g.
// rutracker's session expired, or rutor's mirror is down) is logged and
// does not prevent returning whatever the other source found — degraded
// search results beat none.
func (s *Service) Search(ctx context.Context, query string) []domain.SearchResult {
	var (
		wg               sync.WaitGroup
		rutorResults     []domain.SearchResult
		rutrackerResults []domain.SearchResult
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		results, err := s.rutor.Search(ctx, query)
		if err != nil {
			s.logger.Warn("search: rutor failed", "query", query, "error", err)
			return
		}
		rutorResults = results
	}()
	go func() {
		defer wg.Done()
		results, err := s.rutracker.Search(ctx, query)
		if err != nil {
			s.logger.Warn("search: rutracker failed", "query", query, "error", err)
			return
		}
		rutrackerResults = results
	}()
	wg.Wait()

	merged := make([]domain.SearchResult, 0, len(rutorResults)+len(rutrackerResults))
	merged = append(merged, rutorResults...)
	merged = append(merged, rutrackerResults...)
	sort.Slice(merged, func(i, j int) bool { return merged[i].Seeders > merged[j].Seeders })
	return merged
}

// Resolve turns a chosen search result into what Transmission needs: rutor
// magnet links can be handed to Transmission as-is, but rutracker's
// dl.php link requires our authenticated session, so we fetch the
// .torrent file ourselves and return its bytes instead.
func (s *Service) Resolve(ctx context.Context, result domain.SearchResult) (magnetURL string, torrentFileBytes []byte, err error) {
	switch result.Source {
	case domain.SourceRutor:
		return result.MagnetOrTorrentURL, nil, nil
	case domain.SourceRutracker:
		data, err := s.rutracker.DownloadTorrent(ctx, result.MagnetOrTorrentURL)
		if err != nil {
			return "", nil, err
		}
		return "", data, nil
	default:
		return "", nil, fmt.Errorf("search: unknown source %q", result.Source)
	}
}
