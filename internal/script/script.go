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
	Topic        string
	Tone         string
	Duration     string
	Styles       []string
	Model        string
	Voices       int      // 1-3, defaults to 2 if 0
	Format       string   // show format: conversation, interview, debate, etc.
	SpeakerNames []string // override persona names with voice names (len must match Voices)
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
// apiKey is an optional per-request key override; if empty, providers fall back to env vars.
func NewGenerator(model, apiKey string) (Generator, error) {
	switch model {
	case "haiku", "sonnet":
		return NewClaudeGenerator(model, apiKey), nil
	case "gemini-flash", "gemini-pro":
		return NewGeminiGenerator(model, apiKey), nil
	case "nova-lite":
		return NewNovaGenerator(model)
	default:
		return nil, fmt.Errorf("unknown model %q: must be haiku, sonnet, gemini-flash, gemini-pro, or nova-lite", model)
	}
}

// ModelDisplayName returns a human-readable model name for verbose output.
func ModelDisplayName(model string) string {
	names := map[string]string{
		"haiku":        "claude-haiku-4-5-20251001",
		"sonnet":       "claude-sonnet-4-5-20250929",
		"gemini-flash": "gemini-3-flash-preview",
		"gemini-pro":   "gemini-3-pro-preview",
		"nova-lite":    "us.amazon.nova-2-lite-v1:0",
	}
	if name, ok := names[model]; ok {
		return name
	}
	return model
}

// buildPersonaSlice returns the personas for the given voice count.
// If names is provided and has the right length, persona names are overridden.
func buildPersonaSlice(voices int, names []string) []Persona {
	var personas []Persona
	switch voices {
	case 1:
		personas = []Persona{DefaultAlexPersona}
	case 3:
		personas = []Persona{DefaultAlexPersona, DefaultSamPersona, DefaultJordanPersona}
	default:
		personas = []Persona{DefaultAlexPersona, DefaultSamPersona}
	}
	if len(names) == len(personas) {
		for i, n := range names {
			personas[i].Name = n
		}
	}
	return personas
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
