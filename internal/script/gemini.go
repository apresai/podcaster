package script

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var geminiModels = map[string]string{
	"gemini-flash": "gemini-2.5-flash",
	"gemini-pro":   "gemini-2.5-pro",
}

const geminiGenerateEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

type GeminiGenerator struct {
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewGeminiGenerator(model string) *GeminiGenerator {
	return &GeminiGenerator{
		model:      model,
		apiKey:     os.Getenv("GEMINI_API_KEY"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// geminiTextRequest is the request body for Gemini text generation.
type geminiTextRequest struct {
	SystemInstruction *geminiTextContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiTextContent `json:"contents"`
	GenerationConfig  *geminiTextGenCfg   `json:"generationConfig,omitempty"`
}

type geminiTextContent struct {
	Parts []geminiTextPart `json:"parts"`
}

type geminiTextPart struct {
	Text string `json:"text"`
}

type geminiTextGenCfg struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

// geminiTextResponse is the response from Gemini generateContent (text mode).
type geminiTextResponse struct {
	Candidates []geminiTextCandidate `json:"candidates"`
}

type geminiTextCandidate struct {
	Content geminiTextRespContent `json:"content"`
}

type geminiTextRespContent struct {
	Parts []geminiTextRespPart `json:"parts"`
}

type geminiTextRespPart struct {
	Text string `json:"text"`
}

func (g *GeminiGenerator) Generate(ctx context.Context, content string, opts GenerateOptions) (*Script, error) {
	personas := buildPersonaSlice(opts.Voices)
	sysPrompt := buildSystemPrompt(personas)
	userPrompt := buildUserPrompt(content, opts)

	modelID := geminiModels[g.model]
	if modelID == "" {
		modelID = geminiModels["gemini-flash"]
	}

	maxTokens := 8192
	switch opts.Duration {
	case "long":
		maxTokens = 16384
	case "deep":
		maxTokens = 32768
	}

	reqBody := geminiTextRequest{
		SystemInstruction: &geminiTextContent{
			Parts: []geminiTextPart{{Text: sysPrompt}},
		},
		Contents: []geminiTextContent{
			{Parts: []geminiTextPart{{Text: userPrompt}}},
		},
		GenerationConfig: &geminiTextGenCfg{
			Temperature:     temperature,
			MaxOutputTokens: maxTokens,
		},
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		text, err := g.doRequest(ctx, modelID, reqBody)
		if err != nil {
			lastErr = fmt.Errorf("Gemini API error (attempt %d/%d): %w", attempt, maxRetries, err)
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

		if text == "" {
			lastErr = fmt.Errorf("empty response from Gemini (attempt %d/%d)", attempt, maxRetries)
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

func (g *GeminiGenerator) doRequest(ctx context.Context, modelID string, reqBody geminiTextRequest) (string, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(geminiGenerateEndpoint+"?key=%s", modelID, g.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("retryable error (status %d): %s", res.StatusCode, string(errBody))
	}

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("Gemini API error (status %d): %s", res.StatusCode, string(errBody))
	}

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var resp geminiTextResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("response contained no text")
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}
