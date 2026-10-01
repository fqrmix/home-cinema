// Package downloads talks to the transmission-daemon running on the router
// and keeps our own record of each torrent's progress in sync with it.
package downloads

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// TransmissionClient is a minimal client for the Transmission RPC protocol:
// https://github.com/transmission/transmission/blob/main/docs/rpc-spec.md
//
// Transmission requires an X-Transmission-Session-Id header on every
// request; a stale or missing one gets a 409 response carrying the current
// id, which the client must retry with. That handshake is hidden here.
type TransmissionClient struct {
	baseURL    string
	user       string
	password   string
	httpClient *http.Client

	mu        sync.Mutex
	sessionID string
}

func NewTransmissionClient(baseURL, user, password string) *TransmissionClient {
	return &TransmissionClient{
		baseURL:    baseURL,
		user:       user,
		password:   password,
		httpClient: &http.Client{},
	}
}

type rpcRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments,omitempty"`
}

type rpcResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments"`
}

func (c *TransmissionClient) call(ctx context.Context, method string, args, out any) error {
	body, err := json.Marshal(rpcRequest{Method: method, Arguments: args})
	if err != nil {
		return fmt.Errorf("transmission: encode request: %w", err)
	}

	// One retry is enough: the first attempt either succeeds or teaches us
	// the current session id via a 409, and the second attempt uses it.
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("transmission: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.user != "" {
			req.SetBasicAuth(c.user, c.password)
		}
		c.mu.Lock()
		sessionID := c.sessionID
		c.mu.Unlock()
		if sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessionID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("transmission: request %s: %w", method, err)
		}

		if resp.StatusCode == http.StatusConflict {
			newSessionID := resp.Header.Get("X-Transmission-Session-Id")
			_ = resp.Body.Close()
			c.mu.Lock()
			c.sessionID = newSessionID
			c.mu.Unlock()
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("transmission: %s returned HTTP %d", method, resp.StatusCode)
		}

		var rpcResp rpcResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			return fmt.Errorf("transmission: decode response: %w", err)
		}
		if rpcResp.Result != "success" {
			return fmt.Errorf("transmission: %s failed: %s", method, rpcResp.Result)
		}
		if out != nil && len(rpcResp.Arguments) > 0 {
			if err := json.Unmarshal(rpcResp.Arguments, out); err != nil {
				return fmt.Errorf("transmission: decode arguments: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("transmission: could not obtain a valid session id for %s", method)
}

// AddTorrent submits a magnet link or .torrent URL to Transmission and
// returns the resulting torrent's info hash. Transmission uses its own
// pre-configured download-dir (set up on the router — see README); we
// never override it here, since any path we could pass is relative to
// this container's filesystem, not Transmission's.
func (c *TransmissionClient) AddTorrent(ctx context.Context, magnetOrTorrentURL string) (hash string, err error) {
	args := map[string]any{"filename": magnetOrTorrentURL}

	var out struct {
		TorrentAdded *struct {
			HashString string `json:"hashString"`
		} `json:"torrent-added"`
		TorrentDuplicate *struct {
			HashString string `json:"hashString"`
		} `json:"torrent-duplicate"`
	}
	if err := c.call(ctx, "torrent-add", args, &out); err != nil {
		return "", err
	}
	switch {
	case out.TorrentAdded != nil:
		return out.TorrentAdded.HashString, nil
	case out.TorrentDuplicate != nil:
		return out.TorrentDuplicate.HashString, nil
	default:
		return "", fmt.Errorf("transmission: torrent-add returned neither torrent-added nor torrent-duplicate")
	}
}

// AddTorrentFile submits raw .torrent file bytes to Transmission, base64
// encoded as Transmission's RPC requires. This is used for rutracker
// results: Transmission cannot fetch dl.php itself (it has no rutracker
// session cookie), so we download the .torrent ourselves and hand it the
// bytes instead of a URL.
func (c *TransmissionClient) AddTorrentFile(ctx context.Context, torrentFileBytes []byte) (hash string, err error) {
	args := map[string]any{"metainfo": base64.StdEncoding.EncodeToString(torrentFileBytes)}

	var out struct {
		TorrentAdded *struct {
			HashString string `json:"hashString"`
		} `json:"torrent-added"`
		TorrentDuplicate *struct {
			HashString string `json:"hashString"`
		} `json:"torrent-duplicate"`
	}
	if err := c.call(ctx, "torrent-add", args, &out); err != nil {
		return "", err
	}
	switch {
	case out.TorrentAdded != nil:
		return out.TorrentAdded.HashString, nil
	case out.TorrentDuplicate != nil:
		return out.TorrentDuplicate.HashString, nil
	default:
		return "", fmt.Errorf("transmission: torrent-add returned neither torrent-added nor torrent-duplicate")
	}
}

// TorrentStatus is the subset of Transmission's torrent-get fields we track.
type TorrentStatus struct {
	HashString  string  `json:"hashString"`
	Status      int     `json:"status"`
	PercentDone float64 `json:"percentDone"`
	DownloadDir string  `json:"downloadDir"`
	ErrorString string  `json:"errorString"`
}

// GetStatuses fetches current status for the given torrent hashes.
func (c *TransmissionClient) GetStatuses(ctx context.Context, hashes []string) ([]TorrentStatus, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	args := map[string]any{
		"fields": []string{"hashString", "status", "percentDone", "downloadDir", "errorString"},
		"ids":    hashes,
	}
	var out struct {
		Torrents []TorrentStatus `json:"torrents"`
	}
	if err := c.call(ctx, "torrent-get", args, &out); err != nil {
		return nil, err
	}
	return out.Torrents, nil
}

// RemoveTorrent removes a torrent from Transmission. deleteData also
// removes the downloaded file(s) from disk.
func (c *TransmissionClient) RemoveTorrent(ctx context.Context, hash string, deleteData bool) error {
	args := map[string]any{"ids": []string{hash}, "delete-local-data": deleteData}
	return c.call(ctx, "torrent-remove", args, nil)
}
