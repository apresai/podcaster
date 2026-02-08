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
	geminiDefaultVoice1 = "Charon"
	geminiDefaultVoice2 = "Leda"
	geminiDefaultVoice3 = "Fenrir"

	geminiDefaultTTSModel = "gemini-2.5-pro-preview-tts"
	geminiEndpointBase    = "https://generativelanguage.googleapis.com/v1beta/models/"
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
	model      string
}

func NewGeminiProvider(voice1, voice2, voice3 string, cfg ProviderConfig) *GeminiProvider {
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

	model := geminiDefaultTTSModel
	if cfg.Model != "" {
		model = cfg.Model
	}

	return &GeminiProvider{
		voices: VoiceMap{
			Host1: Voice{ID: v1, Name: v1},
			Host2: Voice{ID: v2, Name: v2},
			Host3: Voice{ID: v3, Name: v3},
		},
		apiKey:     os.Getenv("GEMINI_API_KEY"),
		httpClient: &http.Client{Timeout: 300 * time.Second},
		model:      model,
	}
}

// endpoint returns the full API URL for this provider's model.
func (p *GeminiProvider) endpoint() string {
	return geminiEndpointBase + p.model + ":generateContent"
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) DefaultVoices() VoiceMap {
	return VoiceMap{
		Host1: Voice{ID: geminiDefaultVoice1, Name: "Charon"},
		Host2: Voice{ID: geminiDefaultVoice2, Name: "Leda"},
		Host3: Voice{ID: geminiDefaultVoice3, Name: "Fenrir"},
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

	// Dynamically build SpeakerVoiceConfigs from the speakers present in segments
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

	data, err := p.doRequest(ctx, req)
	if err != nil {
		return AudioResult{}, err
	}

	return AudioResult{Data: data, Format: FormatPCM}, nil
}

func (p *GeminiProvider) doRequest(ctx context.Context, reqBody geminiRequest) ([]byte, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini request: %w", err)
	}

	url := p.endpoint() + "?key=" + p.apiKey

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
		{ID: "Charon", Name: "Charon", Gender: "male", Description: "Informative", DefaultFor: "Voice 1"},
		{ID: "Leda", Name: "Leda", Gender: "female", Description: "Youthful", DefaultFor: "Voice 2"},
		{ID: "Fenrir", Name: "Fenrir", Gender: "male", Description: "Excitable", DefaultFor: "Voice 3"},
		{ID: "Achernar", Name: "Achernar", Gender: "female", Description: "Soft"},
		{ID: "Achird", Name: "Achird", Gender: "male", Description: "Friendly"},
		{ID: "Algenib", Name: "Algenib", Gender: "male", Description: "Gravelly"},
		{ID: "Algieba", Name: "Algieba", Gender: "male", Description: "Smooth"},
		{ID: "Alnilam", Name: "Alnilam", Gender: "male", Description: "Firm"},
		{ID: "Aoede", Name: "Aoede", Gender: "female", Description: "Breezy"},
		{ID: "Autonoe", Name: "Autonoe", Gender: "female", Description: "Bright"},
		{ID: "Callirrhoe", Name: "Callirrhoe", Gender: "female", Description: "Easy-going"},
		{ID: "Despina", Name: "Despina", Gender: "female", Description: "Smooth"},
		{ID: "Enceladus", Name: "Enceladus", Gender: "male", Description: "Breathy"},
		{ID: "Erinome", Name: "Erinome", Gender: "female", Description: "Clear"},
		{ID: "Gacrux", Name: "Gacrux", Gender: "male", Description: "Mature"},
		{ID: "Iapetus", Name: "Iapetus", Gender: "male", Description: "Clear"},
		{ID: "Kore", Name: "Kore", Gender: "female", Description: "Firm"},
		{ID: "Laomedeia", Name: "Laomedeia", Gender: "female", Description: "Upbeat"},
		{ID: "Orus", Name: "Orus", Gender: "male", Description: "Firm"},
		{ID: "Puck", Name: "Puck", Gender: "male", Description: "Upbeat"},
		{ID: "Pulcherrima", Name: "Pulcherrima", Gender: "female", Description: "Forward"},
		{ID: "Rasalgethi", Name: "Rasalgethi", Gender: "male", Description: "Informative"},
		{ID: "Sadachbia", Name: "Sadachbia", Gender: "female", Description: "Lively"},
		{ID: "Sadaltager", Name: "Sadaltager", Gender: "male", Description: "Knowledgeable"},
		{ID: "Schedar", Name: "Schedar", Gender: "female", Description: "Even"},
		{ID: "Sulafat", Name: "Sulafat", Gender: "female", Description: "Warm"},
		{ID: "Umbriel", Name: "Umbriel", Gender: "male", Description: "Easy-going"},
		{ID: "Vindemiatrix", Name: "Vindemiatrix", Gender: "female", Description: "Gentle"},
		{ID: "Zephyr", Name: "Zephyr", Gender: "female", Description: "Bright"},
		{ID: "Zubenelgenubi", Name: "Zubenelgenubi", Gender: "male", Description: "Casual"},
	}
}
