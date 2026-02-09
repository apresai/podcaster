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
	"time"

	"github.com/apresai/podcaster/internal/script"
	"golang.org/x/oauth2/google"
)

const (
	vertexDefaultModel  = "gemini-2.5-flash-tts"
	vertexDefaultRegion = "us-central1"
)

// VertexProvider implements both Provider and BatchProvider using the
// Vertex AI API (aiplatform.googleapis.com) for Gemini TTS.
// Same voice names and request format as AI Studio, but with OAuth2 auth
// and 30,000 RPM — effectively no rate limit.
type VertexProvider struct {
	voices     VoiceMap
	project    string
	region     string
	model      string
	httpClient *http.Client
	batchHTTPClient *http.Client
}

func NewVertexProvider(voice1, voice2, voice3 string, cfg ProviderConfig) (*VertexProvider, error) {
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

	model := vertexDefaultModel
	if cfg.Model != "" {
		model = cfg.Model
	}

	project := os.Getenv("GCP_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("GCP_PROJECT environment variable is required for gemini-vertex TTS provider")
	}

	region := os.Getenv("GCP_REGION")
	if region == "" {
		region = vertexDefaultRegion
	}

	return &VertexProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: v1},
			Host2: Voice{ID: v2, Name: v2},
			Host3: Voice{ID: v3, Name: v3},
		},
		project: project,
		region:  region,
		model:   model,
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

func (p *VertexProvider) Name() string { return "gemini-vertex" }

func (p *VertexProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: geminiDefaultVoice1, Name: "Charon"},
		Host2: Voice{ID: geminiDefaultVoice2, Name: "Leda"},
		Host3: Voice{ID: geminiDefaultVoice3, Name: "Fenrir"},
	}
}

func (p *VertexProvider) endpoint() string {
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.project, p.region, p.model)
}

// getAccessToken obtains an OAuth2 token via Application Default Credentials.
func (p *VertexProvider) getAccessToken(ctx context.Context) (string, error) {
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("get default token source: %w (hint: run 'gcloud auth application-default login' or set GOOGLE_APPLICATION_CREDENTIALS)", err)
	}
	token, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("get access token: %w", err)
	}
	return token.AccessToken, nil
}

// Synthesize does single-speaker synthesis for one segment.
func (p *VertexProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
	req := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: text}}},
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
func (p *VertexProvider) SynthesizeBatch(ctx context.Context, segments []script.Segment, voices VoiceMap) (AudioResult, error) {
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

	fmt.Fprintf(os.Stderr, "[vertex-batch] Starting batch TTS: segments=%d speakers=%d chars=%d model=%s\n",
		len(segments), len(speakerConfigs), len(dialogue), p.model)
	start := time.Now()

	req := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: dialogue}}},
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
		fmt.Fprintf(os.Stderr, "[vertex-batch] FAILED after %s: %v\n", elapsed, err)
		return AudioResult{}, err
	}

	fmt.Fprintf(os.Stderr, "[vertex-batch] SUCCESS in %s: audio_bytes=%d\n", elapsed, len(data))
	return AudioResult{Data: data, Format: FormatPCM}, nil
}

func (p *VertexProvider) doRequest(ctx context.Context, reqBody geminiRequest, client *http.Client) ([]byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal Vertex request: %w", err)
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := p.endpoint()
	reqSize := len(bodyBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	fmt.Fprintf(os.Stderr, "[vertex] POST %s request_bytes=%d timeout=%s\n", p.model, reqSize, client.Timeout)
	start := time.Now()

	res, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[vertex] HTTP error after %s: %v\n", elapsed, err)
		return nil, &RetryableError{StatusCode: 0, Body: fmt.Sprintf("network error after %s: %v", elapsed, err)}
	}
	defer res.Body.Close()

	fmt.Fprintf(os.Stderr, "[vertex] Response status=%d after %s\n", res.StatusCode, elapsed)

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		bodyStr := string(errBody)
		fmt.Fprintf(os.Stderr, "[vertex] Retryable error %d: %s\n", res.StatusCode, bodyStr[:min(200, len(bodyStr))])

		var retryAfter time.Duration
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if secs, parseErr := strconv.Atoi(ra); parseErr == nil && secs > 0 {
				retryAfter = time.Duration(secs) * time.Second
				fmt.Fprintf(os.Stderr, "[vertex] Rate limited (429), Retry-After: %s\n", retryAfter)
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
		fmt.Fprintf(os.Stderr, "[vertex] API error %d: %s\n", res.StatusCode, bodyStr[:min(200, len(bodyStr))])
		return nil, fmt.Errorf("Vertex AI API error (status %d): %s", res.StatusCode, bodyStr)
	}

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Vertex response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[vertex] Response body read: %d bytes in %s\n", len(respBody), time.Since(start).Round(time.Millisecond))

	var resp geminiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse Vertex response: %w", err)
	}

	if len(resp.Candidates) == 0 ||
		len(resp.Candidates[0].Content.Parts) == 0 ||
		resp.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, fmt.Errorf("Vertex response contained no audio data")
	}

	audioB64 := resp.Candidates[0].Content.Parts[0].InlineData.Data
	audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return nil, fmt.Errorf("decode Vertex audio base64: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[vertex] Audio decoded: %d bytes (base64: %d)\n", len(audioBytes), len(audioB64))
	return audioBytes, nil
}

func (p *VertexProvider) Close() error { return nil }

// Ensure VertexProvider satisfies both interfaces.
var (
	_ Provider      = (*VertexProvider)(nil)
	_ BatchProvider = (*VertexProvider)(nil)
)
