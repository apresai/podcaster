package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
)

type URLIngester struct{}

func (u *URLIngester) Ingest(ctx context.Context, source string) (*Content, error) {
	result, err := u.directFetch(ctx, source)
	if err != nil {
		slog.Warn("direct fetch failed, trying Jina Reader", "url", source, "error", err)
		result, jinaErr := u.jinaFetch(ctx, source)
		if jinaErr != nil {
			return nil, fmt.Errorf("all fetch methods failed for %s: direct=%v, jina=%v", source, err, jinaErr)
		}
		return result, nil
	}
	return result, nil
}

// directFetch attempts a standard HTTP GET with go-readability extraction.
func (u *URLIngester) directFetch(ctx context.Context, source string) (*Content, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %w", source, err)
	}

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

// jinaFetch uses Jina Reader API (r.jina.ai) to fetch and extract content.
// Jina handles JavaScript rendering and bot detection bypass.
func (u *URLIngester) jinaFetch(ctx context.Context, source string) (*Content, error) {
	jinaURL := "https://r.jina.ai/" + source

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jinaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create Jina request for %s: %w", source, err)
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("X-No-Cache", "true")
	req.Header.Set("X-Timeout", "30")

	if key := os.Getenv("JINA_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jina Reader request failed for %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jina Reader returned HTTP %d for %s", resp.StatusCode, source)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInputSize))
	if err != nil {
		return nil, fmt.Errorf("could not read Jina Reader response for %s: %w", source, err)
	}

	text := strings.TrimSpace(string(body))
	if len(text) == 0 {
		return nil, fmt.Errorf("Jina Reader returned empty content for %s", source)
	}

	slog.Info("fetched via Jina Reader fallback", "url", source, "bytes", len(text))

	title := titleFromText(text, 80)

	return &Content{
		Text:      text,
		Title:     title,
		Source:    source,
		WordCount: wordCount(text),
	}, nil
}
