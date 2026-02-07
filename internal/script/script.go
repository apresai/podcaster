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
