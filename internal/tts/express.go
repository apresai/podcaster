package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apresai/podcaster/internal/script"
)

const (
	vertexExpressDefaultModel = "gemini-2.5-flash-tts"
	vertexExpressEndpointBase = "https://aiplatform.googleapis.com/v1/publishers/google/models/"
)

// VertexExpressProvider implements both Provider and BatchProvider using the
// Vertex AI API in "express mode" — API key auth (like AI Studio) but via the
// aiplatform.googleapis.com endpoint, which may have higher quotas.
type VertexExpressProvider struct {
	voices          VoiceMap
	model           string
	apiKey          string
	httpClient      *http.Client
	batchHTTPClient *http.Client
}

func NewVertexExpressProvider(voice1, voice2, voice3 string, cfg ProviderConfig) (*VertexExpressProvider, error) {
	v1 := geminiDefaultVoice1
	v2 := geminiDefaultVoice2
	v3 := geminiDefaultVoice3
	if voice1 != "" {
		v1 = voice1
	}
	if voice2 != "" {
		v2 = voice2
	}
	if voice3 != "" {
		v3 = voice3
	}

	model := vertexExpressDefaultModel
	if cfg.Model != "" {
		model = cfg.Model
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("VERTEX_AI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("VERTEX_AI_API_KEY environment variable is required for vertex-express TTS provider (Google Cloud API key, not AI Studio key)")
	}

	return &VertexExpressProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: v1},
			Host2: Voice{ID: v2, Name: v2},
			Host3: Voice{ID: v3, Name: v3},
		},
		model:  model,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 10 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 70 * time.Second,
				IdleConnTimeout:       10 * time.Second,
				DisableKeepAlives:     true,
			},
		},
		batchHTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 10 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 4 * time.Minute,
				IdleConnTimeout:       10 * time.Second,
				DisableKeepAlives:     true,
			},
		},
	}, nil
}

func (p *VertexExpressProvider) Name() string { return "vertex-express" }

func (p *VertexExpressProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: geminiDefaultVoice1, Name: "Charon"},
		Host2: Voice{ID: geminiDefaultVoice2, Name: "Leda"},
		Host3: Voice{ID: geminiDefaultVoice3, Name: "Fenrir"},
	}
}

func (p *VertexExpressProvider) endpoint() string {
	return vertexExpressEndpointBase + p.model + ":generateContent"
}

// Synthesize does single-speaker synthesis for one segment.
func (p *VertexExpressProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	req := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: text}}},
		},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: geminiSpeechConfig{
				VoiceConfig: &geminiVoiceConfig{
					PrebuiltVoiceConfig: geminiPrebuiltVoice{VoiceName: voice.ID},
				},
			},
		},
	}

	data, err := p.doRequest(ctx, req, p.httpClient)
	if err != nil {
		return AudioResult{}, err
	}

	return AudioResult{Data: data, Format: FormatPCM}, nil
}

// SynthesizeBatch sends the entire script as a multi-speaker dialogue.
func (p *VertexExpressProvider) SynthesizeBatch(ctx context.Context, segments []script.Segment, voices VoiceMap) (AudioResult, error) {
	var dialogue string
	for _, seg := range segments {
		dialogue += fmt.Sprintf("%s: %s\n", seg.Speaker, seg.Text)
	}

	seen := map[string]bool{}
	var speakerConfigs []geminiSpeakerVoiceConfig
	for _, seg := range segments {
		if seen[seg.Speaker] {
			continue
		}
		seen[seg.Speaker] = true
		voice := VoiceForSpeaker(seg.Speaker, voices)
		speakerConfigs = append(speakerConfigs, geminiSpeakerVoiceConfig{
			Speaker: seg.Speaker,
			VoiceConfig: geminiVoiceConfig{
				PrebuiltVoiceConfig: geminiPrebuiltVoice{VoiceName: voice.ID},
			},
		})
	}

	fmt.Fprintf(os.Stderr, "[vertex-express-batch] Starting batch TTS: segments=%d speakers=%d chars=%d model=%s\n",
		len(segments), len(speakerConfigs), len(dialogue), p.model)
	start := time.Now()

	req := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: dialogue}}},
		},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: geminiSpeechConfig{
				MultiSpeakerVoiceConfig: &geminiMultiSpeakerConfig{
					SpeakerVoiceConfigs: speakerConfigs,
				},
			},
		},
	}

	data, err := p.doRequest(ctx, req, p.batchHTTPClient)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vertex-express-batch] FAILED after %s: %v\n", elapsed, err)
		return AudioResult{}, err
	}

	fmt.Fprintf(os.Stderr, "[vertex-express-batch] SUCCESS in %s: audio_bytes=%d\n", elapsed, len(data))
	return AudioResult{Data: data, Format: FormatPCM}, nil
}

func (p *VertexExpressProvider) doRequest(ctx context.Context, reqBody geminiRequest, client *http.Client) ([]byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal vertex-express request: %w", err)
	}

	url := p.endpoint() + "?key=" + p.apiKey
	reqSize := len(bodyBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[vertex-express] POST %s request_bytes=%d timeout=%s\n", p.model, reqSize, client.Timeout)
	start := time.Now()

	res, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vertex-express] HTTP error after %s: %v\n", elapsed, err)
		return nil, &RetryableError{StatusCode: 0, Body: fmt.Sprintf("network error after %s: %v", elapsed, err)}
	}
	defer res.Body.Close()

	fmt.Fprintf(os.Stderr, "[vertex-express] Response status=%d after %s\n", res.StatusCode, elapsed)

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		bodyStr := string(errBody)
		fmt.Fprintf(os.Stderr, "[vertex-express] Retryable error %d: %s\n", res.StatusCode, bodyStr[:min(200, len(bodyStr))])

		// On 429, check if this is a daily quota exhaustion (non-retryable)
		if res.StatusCode == http.StatusTooManyRequests {
			bodyLower := strings.ToLower(bodyStr)
			if strings.Contains(bodyLower, "resource_exhausted") &&
				(strings.Contains(bodyLower, "per day") || strings.Contains(bodyLower, "per_day") || strings.Contains(bodyLower, "rpd")) {
				fmt.Fprintf(os.Stderr, "[vertex-express] Daily quota exhausted (RPD limit reached)\n")
				return nil, fmt.Errorf("Vertex Express TTS daily quota exhausted (RPD limit). Try again tomorrow or switch to --tts gemini-vertex or --tts elevenlabs")
			}
		}

		var retryAfter time.Duration
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if secs, parseErr := strconv.Atoi(ra); parseErr == nil && secs > 0 {
				retryAfter = time.Duration(secs) * time.Second
				fmt.Fprintf(os.Stderr, "[vertex-express] Rate limited (429), Retry-After: %s\n", retryAfter)
			}
		}

		return nil, &RetryableError{
			StatusCode: res.StatusCode,
			Body:       bodyStr,
			RetryAfter: retryAfter,
		}
	}

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		bodyStr := string(errBody)
		fmt.Fprintf(os.Stderr, "[vertex-express] API error %d: %s\n", res.StatusCode, bodyStr[:min(200, len(bodyStr))])
		return nil, fmt.Errorf("Vertex Express API error (status %d): %s", res.StatusCode, bodyStr)
	}

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read vertex-express response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[vertex-express] Response body read: %d bytes in %s\n", len(respBody), time.Since(start).Round(time.Millisecond))

	var resp geminiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse vertex-express response: %w", err)
	}

	if len(resp.Candidates) == 0 ||
		len(resp.Candidates[0].Content.Parts) == 0 ||
		resp.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, &RetryableError{StatusCode: 200, Body: "vertex-express response contained no audio data"}
	}

	audioB64 := resp.Candidates[0].Content.Parts[0].InlineData.Data
	audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return nil, fmt.Errorf("decode vertex-express audio base64: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[vertex-express] Audio decoded: %d bytes (base64: %d)\n", len(audioBytes), len(audioB64))
	return audioBytes, nil
}

func (p *VertexExpressProvider) Close() error { return nil }

// Ensure VertexExpressProvider satisfies both interfaces.
var (
	_ Provider      = (*VertexExpressProvider)(nil)
	_ BatchProvider = (*VertexExpressProvider)(nil)
)
