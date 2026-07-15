// Package m3u resolves M3U playlists to a playable stream URL.
package m3u

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout bounds a playlist fetch. It does not bound playback: the
// player runs as a separate process and streams for as long as it likes.
const DefaultTimeout = 10 * time.Second

// maxBodyBytes caps how much of a playlist is read. Real playlists are a few
// hundred bytes, so anything approaching this is a server misbehaving.
const maxBodyBytes = 1 << 20 // 1 MiB

// IsPlaylist reports whether raw points at an M3U playlist.
//
// It tests the URL path, so a query string such as ?token=abc does not defeat
// detection. Extended M3U8/HLS playlists are deliberately not treated as
// playlists: their entries are variant streams, not playable URLs.
func IsPlaylist(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".m3u")
}

// Resolver fetches M3U playlists.
type Resolver struct {
	client *http.Client
}

// New returns a Resolver whose requests are bounded by [DefaultTimeout].
func New() *Resolver {
	return &Resolver{client: &http.Client{Timeout: DefaultTimeout}}
}

// NewWithClient returns a Resolver that issues requests through c.
func NewWithClient(c *http.Client) *Resolver {
	return &Resolver{client: c}
}

// Resolve fetches the playlist at raw and returns its first stream URL.
//
// The response must have a 2xx status: an error page's body frequently
// contains links, and treating one as a playlist would play whatever the
// server happened to link to.
func (r *Resolver) Resolve(ctx context.Context, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", raw, err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching playlist %s: %w", raw, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("fetching playlist %s: server returned %s", raw, resp.Status)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxBodyBytes))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading playlist %s: %w", raw, err)
	}
	return "", fmt.Errorf("no stream URL found in playlist %s", raw)
}
