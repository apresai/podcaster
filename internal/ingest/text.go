package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type TextIngester struct{}

func (t *TextIngester) Ingest(ctx context.Context, source string) (*Content, error) {
	if err := validateFile(source); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", source, err)
	}

	text := string(data)
	if len(text) == 0 {
		return nil, fmt.Errorf("file %s is empty", source)
	}

	return &Content{
		Text:      text,
		Title:     titleFromText(text, 80),
		Source:    filepath.Base(source),
		WordCount: wordCount(text),
	}, nil
}
