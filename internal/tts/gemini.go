package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/apresai/podcaster/internal/script"
)

const (
	geminiDefaultVoiceAlex = "Charon"
	geminiDefaultVoiceSam  = "Leda"

	geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-preview-tts:generateContent"
)

// geminiRequest is the top-level request to the Gemini generateContent TTS endpoint.
type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig geminiGenConfig  `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenConfig struct {
	ResponseModalities []string          `json:"responseModalities"`
	SpeechConfig       geminiSpeechConfig `json:"speechConfig"`
}

type geminiSpeechConfig struct {
	VoiceConfig           *geminiVoiceConfig           `json:"voiceConfig,omitempty"`
	MultiSpeakerVoiceConfig *geminiMultiSpeakerConfig  `json:"multiSpeakerVoiceConfig,omitempty"`
}

type geminiVoiceConfig struct {
	PrebuiltVoiceConfig geminiPrebuiltVoice `json:"prebuiltVoiceConfig"`
}

type geminiMultiSpeakerConfig struct {
	SpeakerVoiceConfigs []geminiSpeakerVoiceConfig `json:"speakerVoiceConfigs"`
}

type geminiSpeakerVoiceConfig struct {
	Speaker     string            `json:"speaker"`
	VoiceConfig geminiVoiceConfig `json:"voiceConfig"`
}

type geminiPrebuiltVoice struct {
	VoiceName string `json:"voiceName"`
}

// geminiResponse is the generateContent response structure.
type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiRespContent `json:"content"`
}

type geminiRespContent struct {
	Parts []geminiRespPart `json:"parts"`
}

type geminiRespPart struct {
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded PCM
}

// GeminiProvider implements both Provider and BatchProvider.
type GeminiProvider struct {
	voices     VoiceMap
	apiKey     string
	httpClient *http.Client
}

func NewGeminiProvider(voiceAlex, voiceSam string) *GeminiProvider {
	alexName := geminiDefaultVoiceAlex
	samName := geminiDefaultVoiceSam
	if voiceAlex != "" {
		alexName = voiceAlex
	}
	if voiceSam != "" {
		samName = voiceSam
	}
	return &GeminiProvider{
		voices: VoiceMap{
			Alex: Voice{ID: alexName, Name: alexName},
			Sam:  Voice{ID: samName, Name: samName},
		},
		apiKey:     os.Getenv("GEMINI_API_KEY"),
		httpClient: &http.Client{Timeout: 300 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Alex: Voice{ID: geminiDefaultVoiceAlex, Name: "Charon"},
		Sam:  Voice{ID: geminiDefaultVoiceSam, Name: "Leda"},
	}
}

// Synthesize does single-speaker synthesis for one segment.
func (p *GeminiProvider) Synthesize(ctx context.Context, text string, voice Voice) (AudioResult, error) {
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

	data, err := p.doRequest(ctx, req)
	if err != nil {
		return AudioResult{}, err
	}

	return AudioResult{Data: data, Format: FormatPCM}, nil
}

// SynthesizeBatch sends the entire script as a multi-speaker dialogue.
// Gemini returns a single PCM audio stream for the whole conversation.
func (p *GeminiProvider) SynthesizeBatch(ctx context.Context, segments []script.Segment, voices VoiceMap) (AudioResult, error) {
	// Build the dialogue text with speaker labels (format: "Speaker: text\n")
	var dialogue string
	for _, seg := range segments {
		dialogue += fmt.Sprintf("%s: %s\n", seg.Speaker, seg.Text)
	}

	req := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: dialogue}}},
		},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: geminiSpeechConfig{
				MultiSpeakerVoiceConfig: &geminiMultiSpeakerConfig{
					SpeakerVoiceConfigs: []geminiSpeakerVoiceConfig{
						{
							Speaker: "Alex",
							VoiceConfig: geminiVoiceConfig{
								PrebuiltVoiceConfig: geminiPrebuiltVoice{VoiceName: voices.Alex.ID},
							},
						},
						{
							Speaker: "Sam",
							VoiceConfig: geminiVoiceConfig{
								PrebuiltVoiceConfig: geminiPrebuiltVoice{VoiceName: voices.Sam.ID},
							},
						},
					},
				},
			},
		},
	}

	fmt.Printf("  Sending %d segments to Gemini multi-speaker TTS...", len(segments))
	data, err := p.doRequest(ctx, req)
	if err != nil {
		fmt.Println(" failed")
		return AudioResult{}, err
	}
	fmt.Printf(" done (%d bytes PCM)\n", len(data))

	return AudioResult{Data: data, Format: FormatPCM}, nil
}

func (p *GeminiProvider) doRequest(ctx context.Context, reqBody geminiRequest) ([]byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini request: %w", err)
	}

	url := geminiEndpoint + "?key=" + p.apiKey

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send Gemini request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests ||
		res.StatusCode >= http.StatusInternalServerError {
		errBody, _ := io.ReadAll(res.Body)
		return nil, &RetryableError{
			StatusCode: res.StatusCode,
			Body:       string(errBody),
		}
	}

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("Gemini API error (status %d): %s", res.StatusCode, string(errBody))
	}

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Gemini response: %w", err)
	}

	var resp geminiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse Gemini response: %w", err)
	}

	if len(resp.Candidates) == 0 ||
		len(resp.Candidates[0].Content.Parts) == 0 ||
		resp.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, fmt.Errorf("Gemini response contained no audio data")
	}

	audioB64 := resp.Candidates[0].Content.Parts[0].InlineData.Data
	audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return nil, fmt.Errorf("decode Gemini audio base64: %w", err)
	}

	return audioBytes, nil
}

func (p *GeminiProvider) Close() error { return nil }

func geminiAvailableVoices() []VoiceInfo {
	return []VoiceInfo{
		{ID: "Charon", Name: "Charon", Gender: "male", Description: "Informative, clear male narrator", DefaultFor: "Alex"},
		{ID: "Leda", Name: "Leda", Gender: "female", Description: "Youthful, bright female voice", DefaultFor: "Sam"},
		{ID: "Kore", Name: "Kore", Gender: "female", Description: "Firm, confident female voice"},
		{ID: "Fenrir", Name: "Fenrir", Gender: "male", Description: "Excitable, deep male voice"},
		{ID: "Aoede", Name: "Aoede", Gender: "female", Description: "Bright, expressive female voice"},
		{ID: "Puck", Name: "Puck", Gender: "male", Description: "Upbeat, energetic male voice"},
		{ID: "Orus", Name: "Orus", Gender: "male", Description: "Firm, authoritative male narrator"},
		{ID: "Zephyr", Name: "Zephyr", Gender: "female", Description: "Breezy, relaxed female voice"},
	}
}
