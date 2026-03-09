// Package fetch provides an interface and implementations for fetching remote content.
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxResponseSize is the maximum allowed response body size (100 MB).
const maxResponseSize = 100 * 1024 * 1024

// Result holds the fetched content and its detected content type.
type Result struct {
	Data        []byte
	ContentType string
}

// Fetcher retrieves content from a URL.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (*Result, error)
}

// HTTPFetcher fetches content via HTTP GET requests.
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher creates a new HTTP fetcher with a 30-second timeout.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Fetch performs an HTTP GET and returns the response body and content type.
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxResponseSize+1)
	data, err := io.ReadAll(limited)
	// coverage:ignore - requires broken HTTP body stream mid-read
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if len(data) > maxResponseSize { // coverage:ignore - requires 100MB+ HTTP response in test
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxResponseSize)
	}

	contentType := resp.Header.Get("Content-Type")
	return &Result{
		Data:        data,
		ContentType: contentType,
	}, nil
}
