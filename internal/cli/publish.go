package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	sdk "github.com/apresai/apresai.dev/sdk"
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

func runPublish(cmd *cobra.Command, args []string) error {
	mp3Path := args[0]

	// Try AI metadata generation if title/summary not provided via flags
	title := flagPublishTitle
	summary := flagPublishSummary
	if title == "" || summary == "" {
		if scriptObj := loadScriptObj(mp3Path); scriptObj != nil {
			if title == "" {
				title = scriptObj.Title
			}
			if summary == "" {
				summary = scriptObj.Summary
			}
			// Still missing? Try Haiku generation from segments
			if (title == "" || summary == "") && len(scriptObj.Segments) > 0 {
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
	}

	// Resolve API key
	apiKey, keySource, err := resolveAPIKey()
	if err != nil {
		return err
	}
	fmt.Printf("API key: found (%s)\n", keySource)

	client := sdk.NewClient(apiKey)
	if flagPublishAPIURL != "" {
		client.BaseURL = flagPublishAPIURL
	}

	result, err := client.Publish(cmd.Context(), mp3Path, sdk.PublishOptions{
		Title:     title,
		Summary:   summary,
		Owner:     flagPublishOwner,
		SourceURL: flagPublishSourceURL,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nPublished: %s\n", result.Title)
	fmt.Printf("  URL: %s/podcasts\n", client.BaseURL)
	if result.AudioURL != "" {
		fmt.Printf("  Audio: %s\n", result.AudioURL)
	}
	return nil
}

// --- AI metadata generation (podcaster-specific, stays in CLI) ---

func loadScriptObj(mp3Path string) *script.Script {
	scriptPath := pipeline.ScriptPath(mp3Path)
	if s, err := script.LoadScript(scriptPath); err == nil {
		return s
	}

	dir := filepath.Dir(mp3Path)
	base := filepath.Base(mp3Path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	siblingPath := filepath.Join(dir, name+".json")
	if s, err := script.LoadScript(siblingPath); err == nil {
		return s
	}

	return nil
}

func generateMetadata(segments []script.Segment) (title, summary string, err error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return "", "", fmt.Errorf("no ANTHROPIC_API_KEY")
	}

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

	var text string
	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += tb.Text
		}
	}

	var result struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
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

// --- API key resolution (CLI-specific) ---

func resolveAPIKey() (key, source string, err error) {
	if k := os.Getenv("PODCASTER_API_KEY"); k != "" {
		return k, "env:PODCASTER_API_KEY", nil
	}

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
