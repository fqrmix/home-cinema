package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type flareSolverrRequest struct {
	Cmd        string `json:"cmd"`
	URL        string `json:"url"`
	PostData   string `json:"postData,omitempty"`
	MaxTimeout int    `json:"maxTimeout"`
}

type flareSolverrResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Solution struct {
		Status    int    `json:"status"`
		UserAgent string `json:"userAgent"`
		Cookies   []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	} `json:"solution"`
}

// flareSolverrResult is what solveFlareSolverr hands back: the cookies
// alone aren't enough to reuse the session with plain HTTP requests —
// Cloudflare ties cf_clearance to the User-Agent that obtained it, so
// every later request has to replay the same one or it gets challenged
// again (confirmed directly: identical cookies get a 403 with a
// mismatched User-Agent and 200 with the matching one).
type flareSolverrResult struct {
	Cookies   []*http.Cookie
	UserAgent string
}

// solveFlareSolverr asks a FlareSolverr sidecar to drive a real (headless)
// browser through targetURL — GET, or POST if postData is non-empty —
// clearing whatever Cloudflare challenge the site shows, and returns the
// resulting cookies. This is the only way to reach rutracker.org
// programmatically: it sits behind a Cloudflare managed challenge that
// blocks every plain HTTP client (curl, net/http, axios, ...) on every
// page, including the login form, confirmed by testing directly. Unlike
// Search and DownloadTorrent, which reuse the resulting cookies for plain
// HTTP requests, logging in has to go through this every time the session
// expires.
func solveFlareSolverr(ctx context.Context, flareSolverrURL, targetURL, postData string) (*flareSolverrResult, error) {
	cmd := "request.get"
	if postData != "" {
		cmd = "request.post"
	}

	// 120s, not FlareSolverr's own 60s default: on a CPU-constrained host
	// (observed: a 2-vCPU box running several other stacks, load average
	// above 2 at the time) solving Cloudflare's challenge is CPU-bound and
	// can simply take longer under contention - confirmed directly, a
	// solve that would otherwise succeed hit "Timeout after 60.0 seconds"
	// with Chrome alone using 135% CPU. This doesn't fix the underlying
	// resource pressure, just gives a slow-but-working solve more room
	// before giving up.
	body, err := json.Marshal(flareSolverrRequest{
		Cmd:        cmd,
		URL:        targetURL,
		PostData:   postData,
		MaxTimeout: 120000,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, flareSolverrURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// FlareSolverr's own maxTimeout above bounds how long it spends solving
	// the challenge; this client timeout just needs enough headroom on top
	// of that for the HTTP round trip itself.
	httpClient := &http.Client{Timeout: 150 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flaresolverr: request: %w", err)
	}
	defer resp.Body.Close()

	var fsResp flareSolverrResponse
	if err := json.NewDecoder(resp.Body).Decode(&fsResp); err != nil {
		return nil, fmt.Errorf("flaresolverr: decode response: %w", err)
	}
	if fsResp.Status != "ok" {
		return nil, fmt.Errorf("flaresolverr: %s", fsResp.Message)
	}

	cookies := make([]*http.Cookie, 0, len(fsResp.Solution.Cookies))
	for _, c := range fsResp.Solution.Cookies {
		cookies = append(cookies, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return &flareSolverrResult{Cookies: cookies, UserAgent: fsResp.Solution.UserAgent}, nil
}

func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, c := range cookies {
		if c.Name == name {
			return true
		}
	}
	return false
}
