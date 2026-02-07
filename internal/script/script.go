package script

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type Script struct {
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	Segments []Segment `json:"segments"`
}

type Segment struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type GenerateOptions struct {
	Topic    string
	Tone     string
	Duration string
	Styles   []string
	Model    string
}

type Generator interface {
	Generate(ctx context.Context, content string, opts GenerateOptions) (*Script, error)
}

func SaveScript(s *Script, path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal script: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write script to %s: %w", path, err)
	}
	return nil
}

// NewGenerator returns the appropriate Generator for the given model name.
func NewGenerator(model string) (Generator, error) {
	switch model {
	case "haiku", "sonnet":
		return NewClaudeGenerator(model), nil
	case "gemini-flash", "gemini-pro":
		return NewGeminiGenerator(model), nil
	default:
		return nil, fmt.Errorf("unknown model %q: must be haiku, sonnet, gemini-flash, or gemini-pro", model)
	}
}

// ModelDisplayName returns a human-readable model name for verbose output.
func ModelDisplayName(model string) string {
	names := map[string]string{
		"haiku":        "claude-haiku-4-5-20251001",
		"sonnet":       "claude-sonnet-4-5-20250929",
		"gemini-flash": "gemini-2.5-flash",
		"gemini-pro":   "gemini-2.5-pro",
	}
	if name, ok := names[model]; ok {
		return name
	}
	return model
}

func LoadScript(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read script from %s: %w", path, err)
	}
	var s Script
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse script from %s: %w", path, err)
	}
	if len(s.Segments) == 0 {
		return nil, fmt.Errorf("script %s has no segments", path)
	}
	return &s, nil
}
