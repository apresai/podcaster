package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apresai/podcaster/internal/pipeline"
	"github.com/apresai/podcaster/internal/progress"
	"github.com/apresai/podcaster/internal/script"
	"github.com/apresai/podcaster/internal/tts"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "podcaster",
	Short: "Convert written content into podcast-style audio conversations",
	RunE: func(cmd *cobra.Command, args []string) error {
		flagTUI = true
		return runGenerate(cmd, args)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("podcaster %s\n", Version)
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a podcast episode from written content",
	RunE:  runGenerate,
}

var listVoicesCmd = &cobra.Command{
	Use:   "list-voices",
	Short: "List available voices for all TTS providers",
	RunE:  runListVoices,
}

var (
	flagInput            string
	flagOutput           string
	flagTopic            string
	flagTone             string
	flagDuration         string
	flagStyle            string
	flagFormat           string
	flagVoice1           string
	flagVoice2           string
	flagVoice3           string
	flagVoices           int
	flagScriptOnly       bool
	flagFromScript       string
	flagVerbose          bool
	flagTTS              string
	flagModel            string
	flagTUI              bool
	flagTTSModel         string
	flagTTSSpeed         float64
	flagTTSStability     float64
	flagTTSPitch         float64
	flagAnthropicAPIKey  string
	flagGeminiAPIKey     string
	flagElevenLabsAPIKey string
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(listVoicesCmd)
	generateCmd.Flags().StringVarP(&flagInput, "input", "i", "", "Source content (URL, PDF path, or text file path)")
	generateCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (MP3)")
	generateCmd.Flags().StringVarP(&flagTopic, "topic", "p", "", "Focus the conversation on a specific topic")
	generateCmd.Flags().StringVarP(&flagTone, "tone", "n", "casual", "Conversation tone: casual, technical, educational")
	generateCmd.Flags().StringVarP(&flagDuration, "duration", "d", "standard", "Target duration: short (~30 segs), standard (~60), long (~100), deep (~200)")
	generateCmd.Flags().StringVarP(&flagStyle, "style", "s", "", "Conversation styles (comma-separated): humor, wow, serious, debate, storytelling")
	generateCmd.Flags().StringVarP(&flagFormat, "format", "F", "conversation", "Show format: conversation, interview, deep-dive, explainer, debate, news, storytelling, challenger")
	generateCmd.Flags().StringVarP(&flagVoice1, "voice1", "1", "", "Voice for host 1 / Alex (provider:voiceID or plain voiceID)")
	generateCmd.Flags().StringVarP(&flagVoice2, "voice2", "2", "", "Voice for host 2 / Sam (provider:voiceID or plain voiceID)")
	generateCmd.Flags().StringVarP(&flagVoice3, "voice3", "3", "", "Voice for host 3 / Jordan (provider:voiceID or plain voiceID)")
	generateCmd.Flags().IntVarP(&flagVoices, "voices", "V", 2, "Number of podcast hosts (1-3)")
	generateCmd.Flags().BoolVarP(&flagScriptOnly, "script-only", "S", false, "Output script JSON only, skip TTS and assembly")
	generateCmd.Flags().StringVarP(&flagFromScript, "from-script", "f", "", "Generate audio from an existing script JSON file")
	generateCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable detailed logging")
	generateCmd.Flags().BoolVarP(&flagTUI, "tui", "t", false, "Interactive setup wizard for generation options")
	generateCmd.Flags().StringVarP(&flagTTS, "tts", "T", "gemini", "TTS provider: gemini, gemini-vertex, vertex-express, elevenlabs, or google (default gemini)")
	generateCmd.Flags().StringVarP(&flagModel, "model", "m", "haiku", "Script generation model: haiku, sonnet, gemini-flash, gemini-pro")
	generateCmd.Flags().StringVar(&flagTTSModel, "tts-model", "", "TTS model ID (e.g., eleven_v3, gemini-2.5-flash-preview-tts)")
	generateCmd.Flags().Float64Var(&flagTTSSpeed, "tts-speed", 0, "Speech speed (ElevenLabs: 0.7-1.2, Google: 0.25-2.0)")
	generateCmd.Flags().Float64Var(&flagTTSStability, "tts-stability", 0, "Voice stability, ElevenLabs only (0.0-1.0)")
	generateCmd.Flags().Float64Var(&flagTTSPitch, "tts-pitch", 0, "Pitch adjustment in semitones, Google only (-20.0 to 20.0)")
	generateCmd.Flags().StringVar(&flagAnthropicAPIKey, "anthropic-api-key", "", "Anthropic API key (overrides ANTHROPIC_API_KEY env var)")
	generateCmd.Flags().StringVar(&flagGeminiAPIKey, "gemini-api-key", "", "Gemini API key (overrides GEMINI_API_KEY env var)")
	generateCmd.Flags().StringVar(&flagElevenLabsAPIKey, "elevenlabs-api-key", "", "ElevenLabs API key (overrides ELEVENLABS_API_KEY env var)")
}

func Execute() error {
	return rootCmd.Execute()
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Run interactive setup if requested
	if flagTUI {
		if err := runInteractiveSetup(); err != nil {
			return err
		}
	}

	// Validate flags
	if flagFromScript == "" && flagInput == "" {
		return fmt.Errorf("either --input (-i) or --from-script (-f) is required")
	}
	if flagFromScript != "" && flagInput != "" {
		return fmt.Errorf("--input and --from-script are mutually exclusive")
	}

	// Validate format
	if !script.IsValidFormat(flagFormat) {
		return fmt.Errorf("invalid format %q: must be one of %s", flagFormat, strings.Join(script.FormatNames(), ", "))
	}

	// Validate tone
	validTones := map[string]bool{"casual": true, "technical": true, "educational": true}
	if !validTones[flagTone] {
		return fmt.Errorf("invalid tone %q: must be casual, technical, or educational", flagTone)
	}

	// Validate duration
	validDurations := map[string]bool{"short": true, "standard": true, "long": true, "deep": true, "medium": true}
	if !validDurations[flagDuration] {
		return fmt.Errorf("invalid duration %q: must be short, standard, long, or deep", flagDuration)
	}
	if flagDuration == "medium" {
		flagDuration = "standard"
	}

	// Validate voices count
	if flagVoices < 1 || flagVoices > 3 {
		return fmt.Errorf("invalid voices count %d: must be 1, 2, or 3", flagVoices)
	}

	// Validate and parse styles
	var styles []string
	if flagStyle != "" {
		validStyles := map[string]bool{"humor": true, "wow": true, "serious": true, "debate": true, "storytelling": true}
		for _, s := range strings.Split(flagStyle, ",") {
			s = strings.TrimSpace(s)
			if !validStyles[s] {
				return fmt.Errorf("invalid style %q: must be humor, wow, serious, debate, or storytelling", s)
			}
			styles = append(styles, s)
		}
	}

	// Validate TTS provider name
	validProviders := map[string]bool{"elevenlabs": true, "google": true, "gemini": true, "gemini-vertex": true, "vertex-express": true}
	if !validProviders[flagTTS] {
		return fmt.Errorf("invalid TTS provider %q: must be gemini, gemini-vertex, vertex-express, elevenlabs, or google", flagTTS)
	}

	// Validate model
	validModels := map[string]bool{"haiku": true, "sonnet": true, "gemini-flash": true, "gemini-pro": true}
	if !validModels[flagModel] {
		return fmt.Errorf("invalid model %q: must be haiku, sonnet, gemini-flash, or gemini-pro", flagModel)
	}

	// Validate TTS model if specified
	if flagTTSModel != "" {
		if err := tts.ValidateModel(flagTTS, flagTTSModel); err != nil {
			return err
		}
	}

	// Validate TTS speed per provider
	if flagTTSSpeed != 0 {
		switch flagTTS {
		case "elevenlabs":
			if flagTTSSpeed < 0.7 || flagTTSSpeed > 1.2 {
				return fmt.Errorf("--tts-speed for ElevenLabs must be between 0.7 and 1.2 (got %.2f)", flagTTSSpeed)
			}
		case "google":
			if flagTTSSpeed < 0.25 || flagTTSSpeed > 2.0 {
				return fmt.Errorf("--tts-speed for Google must be between 0.25 and 2.0 (got %.2f)", flagTTSSpeed)
			}
		case "gemini", "gemini-vertex", "vertex-express":
			return fmt.Errorf("--tts-speed is not supported by Gemini TTS")
		}
	}

	// Validate TTS stability (ElevenLabs only)
	if flagTTSStability != 0 {
		if flagTTS != "elevenlabs" {
			return fmt.Errorf("--tts-stability is only supported by ElevenLabs")
		}
		if flagTTSStability < 0 || flagTTSStability > 1.0 {
			return fmt.Errorf("--tts-stability must be between 0.0 and 1.0 (got %.2f)", flagTTSStability)
		}
	}

	// Validate TTS pitch (Google only)
	if flagTTSPitch != 0 {
		if flagTTS != "google" {
			return fmt.Errorf("--tts-pitch is only supported by Google Cloud TTS")
		}
		if flagTTSPitch < -20.0 || flagTTSPitch > 20.0 {
			return fmt.Errorf("--tts-pitch must be between -20.0 and 20.0 (got %.2f)", flagTTSPitch)
		}
	}

	// Parse provider:voiceID syntax for each voice flag
	v1Provider, v1ID := tts.ParseVoiceSpec(flagVoice1)
	v2Provider, v2ID := tts.ParseVoiceSpec(flagVoice2)
	v3Provider, v3ID := tts.ParseVoiceSpec(flagVoice3)

	// Default to --tts provider when no prefix
	if v1Provider == "" {
		v1Provider = flagTTS
	}
	if v2Provider == "" {
		v2Provider = flagTTS
	}
	if v3Provider == "" {
		v3Provider = flagTTS
	}

	// Check API keys for all providers in use
	ttsProviders := []string{v1Provider, v2Provider}
	if flagVoices >= 3 {
		ttsProviders = append(ttsProviders, v3Provider)
	}
	if err := checkAPIKeys(ttsProviders, flagModel); err != nil {
		return err
	}

	// Check FFmpeg (not needed for script-only)
	if !flagScriptOnly {
		if err := checkFFmpeg(); err != nil {
			return err
		}
	}

	// Route output to podcaster-output/episodes/ (empty = auto-name after script gen)
	var outputPath, logFile string
	if flagOutput != "" {
		outputPath = filepath.Join(pipeline.OutputBaseDir, "episodes", filepath.Base(flagOutput))
		logFile = pipeline.LogFilePath(flagOutput)
	}

	opts := pipeline.Options{
		Input:            flagInput,
		Output:           outputPath,
		Topic:            flagTopic,
		Tone:             flagTone,
		Duration:         flagDuration,
		Format:           flagFormat,
		Styles:           styles,
		Voice1:           v1ID,
		Voice1Provider:   v1Provider,
		Voice2:           v2ID,
		Voice2Provider:   v2Provider,
		Voice3:           v3ID,
		Voice3Provider:   v3Provider,
		Voices:           flagVoices,
		ScriptOnly:       flagScriptOnly,
		FromScript:       flagFromScript,
		Verbose:          flagVerbose,
		DefaultTTS:       flagTTS,
		Model:            flagModel,
		LogFile:          logFile,
		TTSModel:         flagTTSModel,
		TTSSpeed:         flagTTSSpeed,
		TTSStability:     flagTTSStability,
		TTSPitch:         flagTTSPitch,
		AnthropicAPIKey:  flagAnthropicAPIKey,
		GeminiAPIKey:     flagGeminiAPIKey,
		ElevenLabsAPIKey: flagElevenLabsAPIKey,
	}

	// Wire up progress bar when not in verbose mode
	if !flagVerbose {
		r := progress.NewBarRenderer(os.Stdout)
		defer r.Finish()
		opts.OnProgress = r.Handle
	}

	return pipeline.Run(cmd.Context(), opts)
}

func runListVoices(cmd *cobra.Command, args []string) error {
	providers := []struct {
		name  string
		label string
	}{
		{"gemini", "GEMINI (AI Studio)"},
		{"gemini-vertex", "GEMINI (Vertex AI)"},
		{"elevenlabs", "ELEVENLABS"},
		{"google", "GOOGLE CLOUD TTS"},
	}

	fmt.Println("\nAvailable voices:")

	for _, p := range providers {
		voices, err := tts.AvailableVoices(p.name)
		if err != nil {
			return err
		}

		fmt.Printf("\n  %s\n", p.label)
		fmt.Printf("  %s\n", strings.Repeat("\u2500", 50))
		fmt.Printf("  %-28s %-12s %-8s %s\n", "ID", "NAME", "GENDER", "DESCRIPTION")
		for _, v := range voices {
			def := ""
			if v.DefaultFor != "" {
				def = fmt.Sprintf(" (default %s)", v.DefaultFor)
			}
			fmt.Printf("  %-28s %-12s %-8s %s%s\n", v.ID, v.Name, v.Gender, v.Description, def)
		}
	}
	fmt.Println()
	return nil
}

func checkAPIKeys(ttsProviders []string, model string) error {
	needed := map[string]bool{}

	// Check if a key is available via flag or env var
	hasKey := func(envVar, flagVal string) bool {
		return flagVal != "" || os.Getenv(envVar) != ""
	}

	if flagFromScript == "" {
		switch {
		case model == "haiku" || model == "sonnet":
			if !hasKey("ANTHROPIC_API_KEY", flagAnthropicAPIKey) {
				needed["ANTHROPIC_API_KEY"] = true
			}
		case model == "gemini-flash" || model == "gemini-pro":
			if !hasKey("GEMINI_API_KEY", flagGeminiAPIKey) {
				needed["GEMINI_API_KEY"] = true
			}
		}
	}

	if !flagScriptOnly {
		// Deduplicate providers
		seen := map[string]bool{}
		for _, p := range ttsProviders {
			if seen[p] {
				continue
			}
			seen[p] = true
			switch p {
			case "elevenlabs":
				if !hasKey("ELEVENLABS_API_KEY", flagElevenLabsAPIKey) {
					needed["ELEVENLABS_API_KEY"] = true
				}
			case "gemini":
				if !hasKey("GEMINI_API_KEY", flagGeminiAPIKey) {
					needed["GEMINI_API_KEY"] = true
				}
			case "vertex-express":
				if !hasKey("VERTEX_AI_API_KEY_1", "") && !hasKey("GEMINI_API_KEY", flagGeminiAPIKey) {
					needed["VERTEX_AI_API_KEY_1 or GEMINI_API_KEY"] = true
				}
			case "gemini-vertex":
				// Uses ADC (gcloud auth application-default login or GOOGLE_APPLICATION_CREDENTIALS)
				// GCP_PROJECT is validated in NewVertexProvider
			case "google":
				// Uses Application Default Credentials
			}
		}
	}

	if len(needed) > 0 {
		var missing []string
		for k := range needed {
			missing = append(missing, k)
		}
		return fmt.Errorf("missing required environment variable(s): %s\nYou can also pass these via --anthropic-api-key, --gemini-api-key, --elevenlabs-api-key flags", strings.Join(missing, ", "))
	}
	return nil
}

func checkFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("FFmpeg not found — install with: brew install ffmpeg")
	}
	return nil
}
