package ingest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

type PDFIngester struct{}

func (p *PDFIngester) Ingest(ctx context.Context, source string) (*Content, error) {
	if err := validateFile(source); err != nil {
		return nil, err
	}

	f, r, err := pdf.Open(source)
	if err != nil {
		return nil, fmt.Errorf("could not read PDF %s: %w", source, err)
	}
	defer f.Close()

	var sb strings.Builder
	numPages := r.NumPage()

	for i := 1; i <= numPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue // Skip pages that fail to extract
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	text := strings.TrimSpace(sb.String())
	if len(text) == 0 {
		return nil, fmt.Errorf("could not extract text from PDF %s — it may be scanned or image-based", source)
	}

	return &Content{
		Text:      text,
		Title:     titleFromText(text, 80),
		Source:    filepath.Base(source),
		WordCount: wordCount(text),
	}, nil
}
