// Package search implements the two tracker integrations (rutor, rutracker)
// and merges their results into a single ranked list for the API.
package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"homecinema/internal/domain"
)

const rutorBaseURL = "http://rutor.info"

// RutorClient searches rutor's web search page. Rutor requires no
// authentication.
type RutorClient struct {
	httpClient *http.Client
}

func NewRutorClient() *RutorClient {
	return &RutorClient{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

var (
	rutorRowPattern      = regexp.MustCompile(`(?s)<tr class="(?:gai|tum)">.*?</tr>`)
	rutorTitlePattern    = regexp.MustCompile(`(?s)<a href="/torrent/\d+[^"]*">(.*?)</a>`)
	rutorMagnetPattern   = regexp.MustCompile(`href="(magnet:\?[^"]+)"`)
	rutorSizePattern     = regexp.MustCompile(`<td align="right">([\d.,]+)&nbsp;(\w+)</td>`)
	rutorSeedsPattern    = regexp.MustCompile(`class="green">.*?&nbsp;(\d+)</span>`)
	rutorLeechersPattern = regexp.MustCompile(`class="red">&nbsp;(\d+)</span>`)
	rutorDatePattern     = regexp.MustCompile(`<td>(\d{1,2})&nbsp;(\p{L}+)&nbsp;(\d{2})</td>`)
)

var rutorMonths = map[string]time.Month{
	"Янв": time.January, "Фев": time.February, "Мар": time.March,
	"Апр": time.April, "Май": time.May, "Июн": time.June,
	"Июл": time.July, "Авг": time.August, "Сен": time.September,
	"Окт": time.October, "Ноя": time.November, "Дек": time.December,
}

// Search queries rutor's search page and scrapes the results table.
//
// KNOWN RISK: rutor dropped its RSS search endpoint at some point after
// this integration was first written (it now 404s) and merged its mirrors
// onto rutor.info/rutor.is, serving results only as an HTML table at
// /search/<query>. This parses that HTML by regex rather than through a
// stable API, so it WILL break again whenever rutor changes its markup —
// re-verify the "gai"/"tum" row classes and magnet/title/size/peers
// patterns below against a live page before relying on this in production.
func (c *RutorClient) Search(ctx context.Context, query string) ([]domain.SearchResult, error) {
	endpoint := fmt.Sprintf("%s/search/%s", rutorBaseURL, url.PathEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rutor: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rutor: unexpected status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	var results []domain.SearchResult
	for _, row := range rutorRowPattern.FindAllString(body, -1) {
		magnet := firstSubmatch(rutorMagnetPattern, row)
		if magnet == "" {
			continue
		}
		title := firstSubmatch(rutorTitlePattern, row)
		if title == "" {
			continue
		}

		r := domain.SearchResult{
			Source:             domain.SourceRutor,
			Title:              cleanHTML(title),
			MagnetOrTorrentURL: magnet,
			SizeBytes:          parseRutorSize(row),
			PublishDate:        parseRutorDate(row),
		}
		r.Seeders, _ = strconv.Atoi(firstSubmatch(rutorSeedsPattern, row))
		r.Leechers, _ = strconv.Atoi(firstSubmatch(rutorLeechersPattern, row))
		results = append(results, r)
	}
	return results, nil
}

func parseRutorSize(row string) int64 {
	m := rutorSizePattern.FindStringSubmatch(row)
	if m == nil {
		return 0
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if err != nil {
		return 0
	}
	return bytesForUnit(value, m[2])
}

func parseRutorDate(row string) time.Time {
	m := rutorDatePattern.FindStringSubmatch(row)
	if m == nil {
		return time.Time{}
	}
	day, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}
	}
	month, ok := rutorMonths[m[2]]
	if !ok {
		return time.Time{}
	}
	year, err := strconv.Atoi(m[3])
	if err != nil {
		return time.Time{}
	}
	return time.Date(2000+year, month, day, 0, 0, 0, 0, time.UTC)
}

func bytesForUnit(value float64, unit string) int64 {
	switch strings.ToUpper(unit) {
	case "GB", "ГБ":
		return int64(value * 1024 * 1024 * 1024)
	case "MB", "МБ":
		return int64(value * 1024 * 1024)
	case "KB", "КБ":
		return int64(value * 1024)
	default:
		return 0
	}
}
