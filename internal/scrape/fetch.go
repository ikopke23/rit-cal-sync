package scrape

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// userAgent identifies the tool to rit.edu so the site owners can see who is
// scraping and how to reach the maintainer.
const userAgent = "rit-cal-sync (+github.com/ikopke/rit-cal-sync)"

// maxBodyBytes caps the page read; the real page is ~95 KB, so 2 MiB leaves
// ample headroom while protecting against a runaway response.
const maxBodyBytes = 2 << 20

// Fetch makes a single GET to url and returns the body. The client's timeout
// and ctx bound the request; non-2xx responses are errors so a broken page
// never reaches the parser.
func Fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building calendar request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching calendar page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching calendar page: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading calendar page: %w", err)
	}
	return body, nil
}
