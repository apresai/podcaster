package ingest

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type SourceType string

const (
	SourceURL  SourceType = "url"
	SourcePDF  SourceType = "pdf"
	SourceText SourceType = "text"

	// maxInputSize is the maximum allowed size for input content (25 MB).
	maxInputSize = 25 * 1024 * 1024
)

func (s SourceType) String() string {
	return string(s)
}

type Content struct {
	Text      string
	Title     string
	Source    string
	WordCount int
}

type Ingester interface {
	Ingest(ctx context.Context, source string) (*Content, error)
}

func DetectSource(input string) SourceType {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return SourceURL
	}
	if strings.HasSuffix(strings.ToLower(input), ".pdf") {
		return SourcePDF
	}
	return SourceText
}

func NewIngester(input string) Ingester {
	switch DetectSource(input) {
	case SourceURL:
		return &URLIngester{}
	case SourcePDF:
		return &PDFIngester{}
	default:
		return &TextIngester{}
	}
}

func wordCount(text string) int {
	count := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

func titleFromText(text string, maxLen int) string {
	line := text
	if idx := strings.IndexByte(text, '\n'); idx > 0 {
		line = text[:idx]
	}
	line = strings.TrimSpace(line)
	if len(line) > maxLen {
		line = line[:maxLen] + "..."
	}
	if line == "" {
		return "Untitled"
	}
	return line
}

func validateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", path)
	}
	if info.Size() > maxInputSize {
		return fmt.Errorf("%s is too large (%d MB, max %d MB)", path, info.Size()/(1024*1024), maxInputSize/(1024*1024))
	}
	return nil
}
