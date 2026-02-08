package script

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

var claudeModels = map[string]string{
	"haiku":  "claude-haiku-4-5-20251001",
	"sonnet": "claude-sonnet-4-5-20250929",
}

const (
	temperature    = 0.7
	maxRetries     = 3
	initialBackoff = 1 * time.Second
	backoffMult    = 2
)

func maxTokensForDuration(duration string) int64 {
	switch duration {
	case "long":
		return 24576
	case "deep":
		return 32768
	default: // short, standard, medium
		return 8192
	}
}

type ClaudeGenerator struct {
	model string
}

func NewClaudeGenerator(model string) *ClaudeGenerator {
	return &ClaudeGenerator{model: model}
}

func (g *ClaudeGenerator) Generate(ctx context.Context, content string, opts GenerateOptions) (*Script, error) {
	client := anthropic.NewClient()

	personas := buildPersonaSlice(opts.Voices)
	sysPrompt := buildSystemPrompt(personas)
	userPrompt := buildUserPrompt(content, opts)

	modelID := claudeModels[g.model]
	if modelID == "" {
		modelID = claudeModels["haiku"]
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(modelID),
			MaxTokens: maxTokensForDuration(opts.Duration),
			Temperature: anthropic.Float(temperature),
			System: []anthropic.TextBlockParam{
				{Text: sysPrompt},
			},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
			},
		})
		if err != nil {
			lastErr = fmt.Errorf("Claude API error (attempt %d/%d): %w", attempt, maxRetries, err)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		// Extract text from response
		text := extractText(message)
		if text == "" {
			lastErr = fmt.Errorf("empty response from Claude (attempt %d/%d)", attempt, maxRetries)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		// Parse the JSON script
		script, err := parseScript(text)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse script JSON (attempt %d/%d): %w", attempt, maxRetries, err)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		return script, nil
	}

	return nil, lastErr
}

func extractText(msg *anthropic.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	return strings.Join(parts, "")
}

func parseScript(text string) (*Script, error) {
	// Strip scratchpad tags and content
	text = stripScratchpad(text)

	// Strip markdown JSON fences if present
	text = stripMarkdownFences(text)

	// Try to extract JSON object
	text = extractJSON(text)

	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("no JSON content found in response")
	}

	var s Script
	if err := json.Unmarshal([]byte(text), &s); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nRaw text (first 500 chars): %s", err, truncate(text, 500))
	}

	// Validate
	if len(s.Segments) == 0 {
		return nil, fmt.Errorf("script has no segments")
	}
	validSpeakers := map[string]bool{"Alex": true, "Sam": true, "Jordan": true}
	for i, seg := range s.Segments {
		if !validSpeakers[seg.Speaker] {
			return nil, fmt.Errorf("segment %d has invalid speaker %q (must be Alex, Sam, or Jordan)", i, seg.Speaker)
		}
		if strings.TrimSpace(seg.Text) == "" {
			return nil, fmt.Errorf("segment %d has empty text", i)
		}
	}

	return &s, nil
}

var scratchpadRe = regexp.MustCompile(`(?s)<scratchpad>.*?</scratchpad>`)

func stripScratchpad(text string) string {
	return scratchpadRe.ReplaceAllString(text, "")
}

func stripMarkdownFences(text string) string {
	// Strip ```json ... ``` or ``` ... ```
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)\n?```")
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		return matches[1]
	}
	return text
}

func extractJSON(text string) string {
	// Find the first { and last } to extract the JSON object
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
