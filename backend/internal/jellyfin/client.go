// Package jellyfin triggers a library rescan after a download completes, so
// the new file shows up without waiting for Jellyfin's own periodic scan.
package jellyfin

import (
	"context"
	"fmt"
	"net/http"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey, httpClient: &http.Client{}}
}

// RefreshLibrary asks Jellyfin to rescan all libraries. A full scan (rather
// than a targeted one) is simplest and cheap enough for a small personal
// library.
func (c *Client) RefreshLibrary(ctx context.Context) error {
	if c.baseURL == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/Library/Refresh", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jellyfin: refresh request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("jellyfin: refresh returned HTTP %d", resp.StatusCode)
	}
	return nil
}
