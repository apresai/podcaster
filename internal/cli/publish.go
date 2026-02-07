package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/apresai/podcaster/internal/pipeline"
	"github.com/apresai/podcaster/internal/script"
	"github.com/spf13/cobra"
)

var (
	flagPublishTitle     string
	flagPublishSummary   string
	flagPublishOwner     string
	flagPublishSourceURL string
	flagPublishAPIURL    string
)

var publishCmd = &cobra.Command{
	Use:   "publish <mp3-file>",
	Short: "Publish a podcast episode to the Apres AI platform",
	Long:  "Upload an MP3 file and publish it to apresai.dev. Metadata is auto-detected from the companion script JSON if available.",
	Args:  cobra.ExactArgs(1),
	RunE:  runPublish,
}

func init() {
	rootCmd.AddCommand(publishCmd)
	publishCmd.Flags().StringVar(&flagPublishTitle, "title", "", "Episode title (overrides auto-detected)")
	publishCmd.Flags().StringVar(&flagPublishSummary, "summary", "", "Episode summary (overrides auto-detected)")
	defaultOwner := "Apres AI"
	if u, err := user.Current(); err == nil && u.Name != "" {
		defaultOwner = u.Name
	}
	publishCmd.Flags().StringVar(&flagPublishOwner, "owner", defaultOwner, "Episode owner")
	publishCmd.Flags().StringVar(&flagPublishSourceURL, "source-url", "", "Original source URL")
	publishCmd.Flags().StringVar(&flagPublishAPIURL, "api-url", "https://apresai.dev", "API base URL")
}

// --- Types ---

type publishMeta struct {
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Owner      string  `json:"owner"`
	Duration   string  `json:"duration"`
	FileSizeMB float64 `json:"fileSizeMB"`
	SourceURL  string  `json:"sourceUrl,omitempty"`
}

type uploadResponse struct {
	PodcastID string `json:"podcastId"`
	UploadURL string `json:"uploadUrl"`
	AudioKey  string `json:"audioKey"`
}

type confirmResponse struct {
	PodcastID string `json:"podcastId"`
	Status    string `json:"status"`
	AudioURL  string `json:"audioUrl"`
}

// --- Handler ---

func runPublish(cmd *cobra.Command, args []string) error {
	mp3Path := args[0]

	// 1. Validate MP3
	if !strings.HasSuffix(strings.ToLower(mp3Path), ".mp3") {
		return fmt.Errorf("file must have .mp3 extension: %s", mp3Path)
	}
	info, err := os.Stat(mp3Path)
	if err != nil {
		return fmt.Errorf("cannot access file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("file is empty: %s", mp3Path)
	}
	fileSizeMB := float64(info.Size()) / (1024 * 1024)
	fmt.Printf("File: %s (%.1f MB)\n", mp3Path, fileSizeMB)

	// 2. Probe duration
	duration := pipeline.ProbeDuration(mp3Path)
	if duration == "" {
		fmt.Println("Warning: ffprobe not found — install FFmpeg: brew install ffmpeg")
		duration = "unknown"
	} else {
		fmt.Printf("Duration: %s\n", duration)
	}

	// 3. Auto-detect metadata
	title, summary := detectScript(mp3Path)
	if flagPublishTitle != "" {
		title = flagPublishTitle
	}
	if flagPublishSummary != "" {
		summary = flagPublishSummary
	}

	// If title or summary still missing, try AI generation from script segments
	if title == "" || summary == "" {
		if scriptObj := loadScriptObj(mp3Path); scriptObj != nil && len(scriptObj.Segments) > 0 {
			fmt.Print("Generating metadata via Haiku...")
			aiTitle, aiSummary, err := generateMetadata(scriptObj.Segments)
			if err == nil {
				if title == "" && aiTitle != "" {
					title = aiTitle
				}
				if summary == "" && aiSummary != "" {
					summary = aiSummary
				}
				fmt.Println(" done")
			} else {
				fmt.Println(" skipped")
			}
		}
	}

	if title == "" {
		// Fallback: use filename without extension
		base := filepath.Base(mp3Path)
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}
	fmt.Printf("Title: %s\n", title)

	// 4. Resolve API key
	apiKey, keySource, err := resolveAPIKey()
	if err != nil {
		return err
	}
	fmt.Printf("API key: found (%s)\n", keySource)

	// 5. Request upload URL
	meta := publishMeta{
		Title:      title,
		Summary:    summary,
		Owner:      flagPublishOwner,
		Duration:   duration,
		FileSizeMB: fileSizeMB,
		SourceURL:  flagPublishSourceURL,
	}

	fmt.Print("Requesting upload URL...")
	var uploadResp uploadResponse
	err = publishRetry(func() error {
		return postJSON(flagPublishAPIURL+"/api/podcasts/upload-url", apiKey, meta, &uploadResp)
	})
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("request upload URL: %w", err)
	}
	fmt.Printf(" ok (id: %s)\n", uploadResp.PodcastID)

	// 6. Upload MP3 to presigned URL
	fmt.Print("Uploading MP3...")
	err = uploadFile(mp3Path, uploadResp.UploadURL, info.Size())
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("upload MP3: %w", err)
	}
	fmt.Println(" done")

	// 7. Confirm upload
	fmt.Print("Confirming publication...")
	confirmBody := map[string]string{"podcastId": uploadResp.PodcastID}
	var confirmResp confirmResponse
	err = publishRetry(func() error {
		return postJSON(flagPublishAPIURL+"/api/podcasts/confirm", apiKey, confirmBody, &confirmResp)
	})
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("confirm upload (file was uploaded but not confirmed): %w", err)
	}
	fmt.Println(" done")

	// 8. Print success
	fmt.Printf("\nPublished: %s\n", title)
	fmt.Printf("  URL: %s/podcasts\n", flagPublishAPIURL)
	if confirmResp.AudioURL != "" {
		fmt.Printf("  Audio: %s\n", confirmResp.AudioURL)
	}

	return nil
}

// --- Metadata detection ---

func loadScriptObj(mp3Path string) *script.Script {
	// Try standard script path: podcaster-output/scripts/{base}.json
	scriptPath := pipeline.ScriptPath(mp3Path)
	if s, err := script.LoadScript(scriptPath); err == nil {
		return s
	}

	// Try sibling: {dir}/{base}.json
	dir := filepath.Dir(mp3Path)
	base := filepath.Base(mp3Path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	siblingPath := filepath.Join(dir, name+".json")
	if s, err := script.LoadScript(siblingPath); err == nil {
		return s
	}

	return nil
}

func detectScript(mp3Path string) (title, summary string) {
	if s := loadScriptObj(mp3Path); s != nil {
		return s.Title, s.Summary
	}
	return "", ""
}

func generateMetadata(segments []script.Segment) (title, summary string, err error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return "", "", fmt.Errorf("no ANTHROPIC_API_KEY")
	}

	// Concatenate first ~2000 chars of segment text
	var sb strings.Builder
	for _, seg := range segments {
		if sb.Len() > 2000 {
			break
		}
		fmt.Fprintf(&sb, "%s: %s\n", seg.Speaker, seg.Text)
	}

	client := anthropic.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model("claude-haiku-4-5-20251001"),
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Text: "You generate podcast episode metadata. Given a podcast script, return a JSON object with two fields: \"title\" (a compelling episode title, max 80 chars) and \"summary\" (a 1-2 sentence description, max 200 chars). Return ONLY the JSON object, no markdown fences."},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("haiku API call: %w", err)
	}

	// Extract text from response
	var text string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += tb.Text
		}
	}

	// Parse JSON response
	var result struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	// Find JSON object in response
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return "", "", fmt.Errorf("no JSON in response")
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return "", "", fmt.Errorf("parse metadata JSON: %w", err)
	}

	return result.Title, result.Summary, nil
}

// --- API key resolution ---

func resolveAPIKey() (key, source string, err error) {
	// 1. Environment variable
	if k := os.Getenv("PODCASTER_API_KEY"); k != "" {
		return k, "env:PODCASTER_API_KEY", nil
	}

	// 2. Secrets file
	home, _ := os.UserHomeDir()
	if home != "" {
		secretPath := filepath.Join(home, ".secrets", "podcast-api-key")
		if data, err := os.ReadFile(secretPath); err == nil {
			k := strings.TrimSpace(string(data))
			if k != "" {
				return k, secretPath, nil
			}
		}
	}

	// 3. Config file
	if home != "" {
		configPath := filepath.Join(home, ".config", "podcaster", "config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var cfg struct {
				APIKey string `json:"apiKey"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.APIKey != "" {
				return cfg.APIKey, configPath, nil
			}
		}
	}

	return "", "", fmt.Errorf("API key not found — set PODCASTER_API_KEY or create ~/.config/podcaster/config.json")
}

// --- HTTP helpers ---

func postJSON(url, apiKey string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}

	return nil
}

func uploadFile(path, uploadURL string, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "audio/mpeg")
	req.ContentLength = size

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// --- Retry ---

func publishRetry(fn func() error) error {
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < 2 {
			time.Sleep(backoffs[attempt])
		}
	}
	return lastErr
}
