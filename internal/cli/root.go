package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chad/podcaster/internal/pipeline"
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

var (
	flagInput      string
	flagOutput     string
	flagTopic      string
	flagTone       string
	flagDuration   string
	flagVoiceAlex  string
	flagVoiceSam   string
	flagScriptOnly bool
	flagFromScript string
	flagVerbose    bool
)

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&flagInput, "input", "i", "", "Source content (URL, PDF path, or text file path)")
	generateCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (MP3 or JSON with --script-only)")
	generateCmd.Flags().StringVar(&flagTopic, "topic", "", "Focus the conversation on a specific topic")
	generateCmd.Flags().StringVar(&flagTone, "tone", "casual", "Conversation tone: casual, technical, educational")
	generateCmd.Flags().StringVar(&flagDuration, "duration", "medium", "Target duration: short, medium, long")
	generateCmd.Flags().StringVar(&flagVoiceAlex, "voice-alex", "", "ElevenLabs voice ID for Alex")
	generateCmd.Flags().StringVar(&flagVoiceSam, "voice-sam", "", "ElevenLabs voice ID for Sam")
	generateCmd.Flags().BoolVar(&flagScriptOnly, "script-only", false, "Output script JSON only, skip TTS and assembly")
	generateCmd.Flags().StringVar(&flagFromScript, "from-script", "", "Generate audio from an existing script JSON file")
	generateCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Enable detailed logging")
}

func Execute() error {
	return rootCmd.Execute()
}

func runGenerate(cmd *cobra.Command, args []string) error {
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
	validDurations := map[string]bool{"short": true, "medium": true, "long": true}
	if !validDurations[flagDuration] {
		return fmt.Errorf("invalid duration %q: must be short, medium, or long", flagDuration)
	}

	// Check API keys
	if err := checkAPIKeys(); err != nil {
		return err
	}

	// Check FFmpeg (not needed for script-only)
	if !flagScriptOnly {
		if err := checkFFmpeg(); err != nil {
			return err
		}
	}

	opts := pipeline.Options{
		Input:      flagInput,
		Output:     flagOutput,
		Topic:      flagTopic,
		Tone:       flagTone,
		Duration:   flagDuration,
		VoiceAlex:  flagVoiceAlex,
		VoiceSam:   flagVoiceSam,
		ScriptOnly: flagScriptOnly,
		FromScript: flagFromScript,
		Verbose:    flagVerbose,
	}

	return pipeline.Run(cmd.Context(), opts)
}

func checkAPIKeys() error {
	var missing []string

	if flagFromScript == "" {
		// Need Claude API for script generation
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			missing = append(missing, "ANTHROPIC_API_KEY")
		}
	}

	if !flagScriptOnly {
		// Need ElevenLabs API for TTS
		if os.Getenv("ELEVENLABS_API_KEY") == "" {
			missing = append(missing, "ELEVENLABS_API_KEY")
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
