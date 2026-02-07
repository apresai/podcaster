package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/chad/podcaster/internal/tts"
)

func runInteractiveSetup() error {
	// Step 1: TTS Provider
	var provider string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("TTS Provider").
				Description("Which text-to-speech service?").
				Options(
					huh.NewOption("ElevenLabs", "elevenlabs"),
					huh.NewOption("Google Cloud TTS", "google"),
					huh.NewOption("Gemini", "gemini"),
				).
				Value(&provider),
		),
	).Run()
	if err != nil {
		return err
	}
	flagTTS = provider

	// Step 2: Tone
	var tone string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Tone").
				Description("What's the vibe?").
				Options(
					huh.NewOption("Casual — light and engaging", "casual"),
					huh.NewOption("Technical — precise, domain-specific", "technical"),
					huh.NewOption("Educational — accessible, builds understanding", "educational"),
				).
				Value(&tone),
		),
	).Run()
	if err != nil {
		return err
	}
	flagTone = tone

	// Step 3: Styles (multi-select)
	var styles []string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Styles").
				Description("Optional. Select any that apply (space to toggle, enter to confirm).").
				Options(
					huh.NewOption("Humor — witty banter, jokes, playful comebacks", "humor"),
					huh.NewOption("Wow — dramatic reveals, mind-blown moments", "wow"),
					huh.NewOption("Serious — gravitas, reflective, no jokes", "serious"),
					huh.NewOption("Debate — hosts disagree, push back, tension", "debate"),
					huh.NewOption("Storytelling — narrative arc, suspense, foreshadowing", "storytelling"),
				).
				Value(&styles),
		),
	).Run()
	if err != nil {
		return err
	}
	if len(styles) > 0 {
		flagStyle = strings.Join(styles, ",")
	}

	// Step 4: Duration
	var duration string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Duration").
				Description("How long should the episode be?").
				Options(
					huh.NewOption("Short — ~30 segments, ~10 min", "short"),
					huh.NewOption("Standard — ~50 segments, ~15 min", "standard"),
					huh.NewOption("Long — ~75 segments, ~25 min", "long"),
					huh.NewOption("Deep Dive — ~200 segments, ~65 min", "deep"),
				).
				Value(&duration),
		),
	).Run()
	if err != nil {
		return err
	}
	flagDuration = duration

	// Step 5: Voice selection
	voices, err := tts.AvailableVoices(flagTTS)
	if err != nil {
		return fmt.Errorf("load voices: %w", err)
	}

	var alexOptions []huh.Option[string]
	var samOptions []huh.Option[string]
	for _, v := range voices {
		label := fmt.Sprintf("%s — %s (%s)", v.Name, v.Description, v.Gender)
		opt := huh.NewOption(label, v.ID)
		alexOptions = append(alexOptions, opt)
		samOptions = append(samOptions, huh.NewOption(label, v.ID))
	}

	var alexVoice, samVoice string
	// Set defaults based on provider defaults
	for _, v := range voices {
		if v.DefaultFor == "Alex" {
			alexVoice = v.ID
		}
		if v.DefaultFor == "Sam" {
			samVoice = v.ID
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Voice for Alex (Host)").
				Options(alexOptions...).
				Value(&alexVoice),
			huh.NewSelect[string]().
				Title("Voice for Sam (Analyst)").
				Options(samOptions...).
				Value(&samVoice),
		),
	).Run()
	if err != nil {
		return err
	}
	flagVoiceAlex = alexVoice
	flagVoiceSam = samVoice

	// Step 6: Summary + confirm
	styleDisplay := "(none)"
	if len(styles) > 0 {
		styleDisplay = strings.Join(styles, ", ")
	}

	summary := fmt.Sprintf(
		"Provider: %s\nTone: %s\nStyles: %s\nDuration: %s\nAlex voice: %s\nSam voice: %s",
		flagTTS, flagTone, styleDisplay, flagDuration, flagVoiceAlex, flagVoiceSam,
	)

	var confirmed bool
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Ready to generate?").
				Description(summary).
				Affirmative("Yes, let's go").
				Negative("Cancel").
				Value(&confirmed),
		),
	).Run()
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("generation cancelled")
	}

	return nil
}
