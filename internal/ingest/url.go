package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	readability "github.com/go-shiori/go-readability"
)

type URLIngester struct{}

func (u *URLIngester) Ingest(ctx context.Context, source string) (*Content, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %w", source, err)
	}

	// Fetch URL with a browser-like User-Agent to avoid being blocked by sites
	// that reject default Go HTTP client requests (e.g., Wikipedia, Cloudflare-protected sites)
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request for %s: %w", source, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Podcaster/1.0; +https://podcasts.apresai.dev)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch URL %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not fetch URL %s: HTTP %d", source, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxInputSize)
	article, err := readability.FromReader(limited, parsed)
	if err != nil {
		return nil, fmt.Errorf("could not extract article from %s: %w", source, err)
	}

	text := article.TextContent
	if len(text) == 0 {
		return nil, fmt.Errorf("no readable content extracted from %s", source)
	}

	title := article.Title
	if title == "" {
		title = titleFromText(text, 80)
	}

	return &Content{
		Text:      text,
		Title:     title,
		Source:    source,
		WordCount: wordCount(text),
	}, nil
}
