package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/charmap"

	"homecinema/internal/domain"
)

const (
	rutrackerBaseURL   = "https://rutracker.org/forum"
	rutrackerLoginURL  = rutrackerBaseURL + "/login.php"
	rutrackerSearchURL = rutrackerBaseURL + "/tracker.php"
)

// RutrackerClient logs in through a FlareSolverr sidecar (see
// flaresolverr.go) and reuses the resulting session cookie for subsequent
// searches and .torrent downloads via plain HTTP, re-logging in only when
// the session appears to have expired.
type RutrackerClient struct {
	flareSolverrURL string
	login, password string
	httpClient      *http.Client

	mu        sync.Mutex
	loggedIn  bool
	userAgent string
}

func NewRutrackerClient(flareSolverrURL, login, password string) *RutrackerClient {
	jar, _ := cookiejar.New(nil)
	return &RutrackerClient{
		flareSolverrURL: flareSolverrURL,
		login:           login,
		password:        password,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *RutrackerClient) ensureLoggedIn() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	return c.login_()
}

// login_ has a trailing underscore only to avoid colliding with the login
// field. It deliberately does not take the caller's context: a cold login
// through FlareSolverr can take 10-60s+ (driving a real browser through
// Cloudflare's challenge), and if the triggering HTTP request's context is
// cancelled partway through — the client retried, nginx's proxy_read_timeout
// fired — we still want the session to finish establishing and get cached.
// Tying this to the caller's context was observed causing exactly the
// opposite: each cancelled request threw away an in-progress login, and the
// next request started a brand new one from scratch; concurrent logins
// piling onto FlareSolverr's single shared browser is what crashed its tab.
func (c *RutrackerClient) login_() error {
	form := url.Values{}
	form.Set("login_username", c.login)
	form.Set("login_password", c.password)
	form.Set("login", "Вход")

	result, err := solveFlareSolverr(context.Background(), c.flareSolverrURL, rutrackerLoginURL, form.Encode())
	if err != nil {
		return fmt.Errorf("rutracker: flaresolverr login: %w", err)
	}
	if !hasCookie(result.Cookies, "bb_session") {
		return fmt.Errorf("rutracker: flaresolverr login did not yield a bb_session cookie (wrong credentials?)")
	}

	base, _ := url.Parse(rutrackerBaseURL)
	c.httpClient.Jar.SetCookies(base, result.Cookies)
	// Cloudflare ties cf_clearance to the User-Agent that obtained it —
	// every request from here on must replay FlareSolverr's, or it gets
	// challenged again despite having a valid cookie (confirmed directly).
	c.userAgent = result.UserAgent
	c.loggedIn = true
	return nil
}

func (c *RutrackerClient) newRequest(ctx context.Context, method, target string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	userAgent := c.userAgent
	c.mu.Unlock()
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}

var (
	rutrackerRowPattern   = regexp.MustCompile(`(?s)<tr[^>]*data-topic_id="(\d+)".*?</tr>`)
	rutrackerTitlePattern = regexp.MustCompile(`(?s)<a[^>]*class="[^"]*\btLink\b[^"]*"[^>]*>(.*?)</a>`)
	rutrackerSizePattern  = regexp.MustCompile(`(?s)<a[^>]*class="[^"]*\btr-dl\b[^"]*"[^>]*href="(dl\.php\?t=\d+)"[^>]*>([\d.,]+)&nbsp;(\w+)`)
	rutrackerSeedsPattern = regexp.MustCompile(`(?s)<b class="seedmed">(\d+)</b>`)
	rutrackerLeechPattern = regexp.MustCompile(`(?s)<td[^>]*class="[^"]*\bleechmed\b[^"]*"[^>]*>(\d+)</td>`)
	htmlTagPattern        = regexp.MustCompile(`<[^>]+>`)
)

// Search queries rutracker's search page and scrapes the results table. If
// the session turns out to have expired (rutracker.org's cookies don't
// live forever), it re-logs in via FlareSolverr once and retries
// transparently — no manual step, unlike before FlareSolverr was wired in.
//
// KNOWN RISK: this parses rutracker's HTML by regex rather than through a
// stable API, so it WILL break whenever rutracker changes its markup.
// Re-verified directly against a live page on 2026-10-01: tLink/tr-dl/
// leechmed now appear as one class among several (`class="med tLink ..."`),
// not standalone, and the size is plain text in the tr-dl anchor rather
// than wrapped in a <u> tag — the patterns below already account for that,
// but check again if parsing silently returns zero results.
func (c *RutrackerClient) Search(ctx context.Context, query string) ([]domain.SearchResult, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	results, expired, err := c.fetchSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	if expired {
		c.mu.Lock()
		c.loggedIn = false
		c.mu.Unlock()

		if err := c.ensureLoggedIn(); err != nil {
			return nil, err
		}
		results, expired, err = c.fetchSearch(ctx, query)
		if err != nil {
			return nil, err
		}
		if expired {
			return nil, fmt.Errorf("rutracker: session still invalid after re-login via flaresolverr")
		}
	}
	return results, nil
}

// fetchSearch performs one search request. The second return value reports
// whether the response indicates an expired session (login page or
// Cloudflare challenge) rather than actual results.
func (c *RutrackerClient) fetchSearch(ctx context.Context, query string) ([]domain.SearchResult, bool, error) {
	endpoint := fmt.Sprintf("%s?nm=%s", rutrackerSearchURL, url.QueryEscape(query))
	req, err := c.newRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("rutracker: search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	// rutracker.org serves this page as Windows-1251, not UTF-8.
	decoded, err := charmap.Windows1251.NewDecoder().Bytes(body)
	if err != nil {
		return nil, false, fmt.Errorf("rutracker: decode windows-1251: %w", err)
	}
	html := string(decoded)

	if strings.Contains(html, `name="login_username"`) ||
		resp.Header.Get("cf-mitigated") == "challenge" ||
		strings.Contains(html, "Just a moment") {
		return nil, true, nil
	}

	var results []domain.SearchResult
	for _, row := range rutrackerRowPattern.FindAllString(html, -1) {
		title := firstSubmatch(rutrackerTitlePattern, row)
		if title == "" {
			continue
		}

		sizeMatch := rutrackerSizePattern.FindStringSubmatch(row)
		if sizeMatch == nil {
			continue
		}
		torrentPath, sizeValue, sizeUnit := sizeMatch[1], sizeMatch[2], sizeMatch[3]

		seeders, _ := strconv.Atoi(firstSubmatch(rutrackerSeedsPattern, row))
		leechers, _ := strconv.Atoi(firstSubmatch(rutrackerLeechPattern, row))
		size, _ := strconv.ParseFloat(strings.ReplaceAll(sizeValue, ",", "."), 64)

		results = append(results, domain.SearchResult{
			Source:             domain.SourceRutracker,
			Title:              cleanHTML(title),
			SizeBytes:          bytesForUnit(size, sizeUnit),
			Seeders:            seeders,
			Leechers:           leechers,
			MagnetOrTorrentURL: rutrackerBaseURL + "/" + torrentPath,
		})
	}
	return results, false, nil
}

// DownloadTorrent fetches the .torrent file bytes for a rutracker result's
// MagnetOrTorrentURL (a dl.php?t=... link) using the authenticated
// session. Transmission cannot fetch this URL itself since it has no
// rutracker session cookie.
func (c *RutrackerClient) DownloadTorrent(ctx context.Context, torrentURL string) ([]byte, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodGet, torrentURL)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rutracker: download torrent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rutracker: download torrent returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func cleanHTML(s string) string {
	return strings.TrimSpace(htmlTagPattern.ReplaceAllString(s, ""))
}
