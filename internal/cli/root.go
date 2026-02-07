package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chad/podcaster/internal/pipeline"
	"github.com/chad/podcaster/internal/tts"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "podcaster",
	Short: "Convert written content into podcast-style audio conversations",
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
	Short: "List available voices for a TTS provider",
	RunE:  runListVoices,
}

var flagListVoicesTTS string

var (
	flagInput       string
	flagOutput      string
	flagTopic       string
	flagTone        string
	flagDuration    string
	flagStyle       string
	flagVoiceAlex   string
	flagVoiceSam    string
	flagScriptOnly  bool
	flagFromScript  string
	flagVerbose     bool
	flagTTS         string
	flagModel       string
	flagInteractive bool
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(listVoicesCmd)
	listVoicesCmd.Flags().StringVar(&flagListVoicesTTS, "tts", "gemini", "TTS provider: elevenlabs, google, or gemini (default gemini)")

	generateCmd.Flags().StringVarP(&flagInput, "input", "i", "", "Source content (URL, PDF path, or text file path)")
	generateCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (MP3 or JSON with --script-only)")
	generateCmd.Flags().StringVar(&flagTopic, "topic", "", "Focus the conversation on a specific topic")
	generateCmd.Flags().StringVar(&flagTone, "tone", "casual", "Conversation tone: casual, technical, educational")
	generateCmd.Flags().StringVar(&flagDuration, "duration", "standard", "Target duration: short (~30 segs), standard (~50), long (~75), deep (~200)")
	generateCmd.Flags().StringVar(&flagStyle, "style", "", "Conversation styles (comma-separated): humor, wow, serious, debate, storytelling")
	generateCmd.Flags().StringVar(&flagVoiceAlex, "voice-alex", "", "Voice ID for Alex (provider-specific)")
	generateCmd.Flags().StringVar(&flagVoiceSam, "voice-sam", "", "Voice ID for Sam (provider-specific)")
	generateCmd.Flags().BoolVar(&flagScriptOnly, "script-only", false, "Output script JSON only, skip TTS and assembly")
	generateCmd.Flags().StringVar(&flagFromScript, "from-script", "", "Generate audio from an existing script JSON file")
	generateCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable detailed logging")
	generateCmd.Flags().BoolVar(&flagInteractive, "interactive", false, "Interactive setup wizard for generation options")
	generateCmd.Flags().StringVar(&flagTTS, "tts", "gemini", "TTS provider: elevenlabs, google, or gemini (default gemini)")
	generateCmd.Flags().StringVar(&flagModel, "model", "haiku", "Script generation model: haiku, sonnet, gemini-flash, gemini-pro")
}

func Execute() error {
	return rootCmd.Execute()
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Run interactive setup if requested
	if flagInteractive {
		if err := runInteractiveSetup(); err != nil {
			return err
		}
	}

	// Validate flags
	if flagFromScript == "" && flagInput == "" {
		return fmt.Errorf("either --input (-i) or --from-script is required")
	}
	if flagFromScript != "" && flagInput != "" {
		return fmt.Errorf("--input and --from-script are mutually exclusive")
	}
	if flagOutput == "" {
		return fmt.Errorf("--output (-o) is required")
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
	validProviders := map[string]bool{"elevenlabs": true, "google": true, "gemini": true}
	if !validProviders[flagTTS] {
		return fmt.Errorf("invalid TTS provider %q: must be elevenlabs, google, or gemini", flagTTS)
	}

	// Validate model
	validModels := map[string]bool{"haiku": true, "sonnet": true, "gemini-flash": true, "gemini-pro": true}
	if !validModels[flagModel] {
		return fmt.Errorf("invalid model %q: must be haiku, sonnet, gemini-flash, or gemini-pro", flagModel)
	}

	// Check API keys
	if err := checkAPIKeys(flagTTS, flagModel); err != nil {
		return err
	}

	// Check FFmpeg (not needed for script-only)
	if !flagScriptOnly {
		if err := checkFFmpeg(); err != nil {
			return err
		}
	}

	opts := pipeline.Options{
		Input:       flagInput,
		Output:      flagOutput,
		Topic:       flagTopic,
		Tone:        flagTone,
		Duration:    flagDuration,
		Styles:      styles,
		VoiceAlex:   flagVoiceAlex,
		VoiceSam:    flagVoiceSam,
		ScriptOnly:  flagScriptOnly,
		FromScript:  flagFromScript,
		Verbose:     flagVerbose,
		TTSProvider: flagTTS,
		Model:       flagModel,
	}

	return pipeline.Run(cmd.Context(), opts)
}

func runListVoices(cmd *cobra.Command, args []string) error {
	voices, err := tts.AvailableVoices(flagListVoicesTTS)
	if err != nil {
		return err
	}

	fmt.Printf("\nAvailable voices for %s:\n\n", flagListVoicesTTS)
	fmt.Printf("  %-28s %-12s %-8s %s\n", "ID", "NAME", "GENDER", "DESCRIPTION")
	fmt.Printf("  %-28s %-12s %-8s %s\n", "---", "----", "------", "-----------")
	for _, v := range voices {
		def := ""
		if v.DefaultFor != "" {
			def = fmt.Sprintf(" (default %s)", v.DefaultFor)
		}
		fmt.Printf("  %-28s %-12s %-8s %s%s\n", v.ID, v.Name, v.Gender, v.Description, def)
	}
	fmt.Println()
	return nil
}

func checkAPIKeys(ttsProvider, model string) error {
	var missing []string

	if flagFromScript == "" {
		switch {
		case model == "haiku" || model == "sonnet":
			if os.Getenv("ANTHROPIC_API_KEY") == "" {
				missing = append(missing, "ANTHROPIC_API_KEY")
			}
		case model == "gemini-flash" || model == "gemini-pro":
			if os.Getenv("GEMINI_API_KEY") == "" {
				missing = append(missing, "GEMINI_API_KEY")
			}
		}
	}

	if !flagScriptOnly {
		switch ttsProvider {
		case "elevenlabs":
			if os.Getenv("ELEVENLABS_API_KEY") == "" {
				missing = append(missing, "ELEVENLABS_API_KEY")
			}
		case "gemini":
			if os.Getenv("GEMINI_API_KEY") == "" {
				missing = append(missing, "GEMINI_API_KEY")
			}
		case "google":
			// Uses Application Default Credentials
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
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
