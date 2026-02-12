package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/apresai/podcaster/internal/pipeline"
	"github.com/apresai/podcaster/internal/script"
	"github.com/apresai/podcaster/internal/tts"
)

// menuItem represents a single configurable option in the TUI.
type menuItem struct {
	label     string
	value     string
	options   []menuOption
	required  bool
	editing   bool
	cursor    int  // cursor within options when editing
	separator bool // if true, render as a section divider (not selectable)
}

type menuOption struct {
	label string
	value string
}

// menuState tracks which phase the TUI is in.
type menuState int

const (
	stateMenu menuState = iota
	stateEditing
	stateStylePicker
	stateInputPicker
)

// tuiModel is the Bubble Tea model for the interactive menu.
type tuiModel struct {
	items       []menuItem
	cursor      int
	state       menuState
	width       int
	err         error
	confirmed   bool
	cancelled   bool
	styles      map[string]bool // for multi-select style picker
	styleCursor int
	voiceCount  int // 1-3
	inputCursor int // cursor for input type picker
}

// style constants
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	menuLabelStyle = lipgloss.NewStyle().
			Width(18).
			Align(lipgloss.Right).
			MarginRight(2)

	menuValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))

	menuValueDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555")).
				Italic(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	requiredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	optionStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedOptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#04B575")).
				Bold(true).
				PaddingLeft(2)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	buttonStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 3)

	buttonDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Padding(0, 3)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	headerBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#7D56F4")).
			MarginBottom(1).
			PaddingBottom(0)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)
)

// New TUI layout: content first, audio second
const (
	idxInput    = 0
	idxOutput   = 1
	idxTopic    = 2
	idxFormat   = 3
	idxTone     = 4
	idxDuration = 5
	idxStyle    = 6
	idxModel    = 7
	idxVoices   = 8
	idxVoice1   = 9
	idxVoice2   = 10
	// idxVoice3 = 11 when voiceCount >= 3
	// idxSeparator = 11 or 12 (non-selectable)
	// idxProvider = 12 or 13
	// idxTTSModel = 13 or 14
	// etc.
)

// maxVisibleOptions is the max number of options shown before scrolling kicks in.
const maxVisibleOptions = 10

// formatFloat returns an empty string for zero (meaning "default"), otherwise a compact string.
func formatFloat(f float64) string {
	if f == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", f)
}

// parseFloat parses a string to float64, returning 0 for empty strings.
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// buildVoiceOptionsForProvider returns voice options filtered by provider.
// If provider is "auto" or empty, returns all providers with prefixes.
func buildVoiceOptionsForProvider(provider string) (opts []menuOption, defaultV1, defaultV2, defaultV3 string) {
	if provider == "auto" || provider == "" {
		return buildAllVoiceOptions()
	}

	// Single provider — no prefix needed
	voices, err := tts.AvailableVoices(provider)
	if err != nil {
		return buildAllVoiceOptions()
	}

	prefixMap := map[string]string{"gemini": "GEM", "elevenlabs": "ELV", "google": "GOO", "polly": "POL"}
	prefix := prefixMap[provider]

	for _, v := range voices {
		label := fmt.Sprintf("[%s] %s - %s (%s)", prefix, v.Name, v.Description, v.Gender)
		value := provider + ":" + v.ID
		opts = append(opts, menuOption{label: label, value: value})
		if v.DefaultFor == "Voice 1" {
			defaultV1 = value
		}
		if v.DefaultFor == "Voice 2" {
			defaultV2 = value
		}
		if v.DefaultFor == "Voice 3" {
			defaultV3 = value
		}
	}
	return
}

// buildAllVoiceOptions returns voice options from all TTS providers, grouped
// by provider with [GEM]/[ELV]/[GOO] prefixes. Values use provider:voiceID format.
func buildAllVoiceOptions() (opts []menuOption, defaultV1, defaultV2, defaultV3 string) {
	type providerInfo struct {
		name   string
		prefix string
	}
	providers := []providerInfo{
		{"gemini", "GEM"},
		{"elevenlabs", "ELV"},
		{"google", "GOO"},
		{"polly", "POL"},
	}

	effectiveTTS := flagTTS
	if effectiveTTS == "auto" {
		effectiveTTS = "gemini"
	}

	for _, p := range providers {
		voices, err := tts.AvailableVoices(p.name)
		if err != nil {
			continue
		}
		for _, v := range voices {
			label := fmt.Sprintf("[%s] %s - %s (%s)", p.prefix, v.Name, v.Description, v.Gender)
			value := p.name + ":" + v.ID
			opts = append(opts, menuOption{label: label, value: value})
			if v.DefaultFor == "Voice 1" && p.name == effectiveTTS {
				defaultV1 = value
			}
			if v.DefaultFor == "Voice 2" && p.name == effectiveTTS {
				defaultV2 = value
			}
			if v.DefaultFor == "Voice 3" && p.name == effectiveTTS {
				defaultV3 = value
			}
		}
	}
	return
}

// ttsModelOptions returns the TTS model choices for a given provider.
func ttsModelOptions(provider string) []menuOption {
	if provider == "auto" {
		provider = "gemini"
	}
	switch provider {
	case "elevenlabs":
		return []menuOption{
			{label: "v3 (highest quality, expressive) (default)", value: "eleven_v3"},
			{label: "Multilingual v2 (consistent, numbers)", value: "eleven_multilingual_v2"},
			{label: "Turbo v2.5 (fast, balanced)", value: "eleven_turbo_v2_5"},
			{label: "Flash v2.5 (fastest, 75ms)", value: "eleven_flash_v2_5"},
		}
	case "gemini":
		return []menuOption{
			{label: "Pro TTS (best quality, nuanced) (default)", value: "gemini-2.5-pro-preview-tts"},
			{label: "Flash TTS (fast, good quality)", value: "gemini-2.5-flash-preview-tts"},
		}
	case "gemini-vertex", "vertex-express":
		return []menuOption{
			{label: "Flash TTS (fast, good quality) (default)", value: "gemini-2.5-flash-tts"},
			{label: "Pro TTS (best quality, nuanced)", value: "gemini-2.5-pro-tts"},
		}
	case "polly":
		return []menuOption{
			{label: "Generative (fixed)", value: ""},
		}
	default:
		return []menuOption{
			{label: "Chirp 3 HD (fixed)", value: ""},
		}
	}
}

// defaultTTSModel returns the default TTS model for a provider.
func defaultTTSModel(provider string) string {
	if provider == "auto" {
		provider = "gemini"
	}
	switch provider {
	case "elevenlabs":
		return "eleven_v3"
	case "gemini":
		return "gemini-2.5-pro-preview-tts"
	case "gemini-vertex", "vertex-express":
		return "gemini-2.5-flash-tts"
	default:
		return ""
	}
}

// ttsSpeedOptions returns speed presets for a given provider.
func ttsSpeedOptions(provider string) []menuOption {
	if provider == "auto" {
		provider = "gemini"
	}
	switch provider {
	case "elevenlabs":
		return []menuOption{
			{label: "1.0 (default)", value: ""},
			{label: "0.80 (slower)", value: "0.80"},
			{label: "0.90 (slightly slow)", value: "0.90"},
			{label: "1.10 (slightly fast)", value: "1.10"},
			{label: "1.20 (faster)", value: "1.20"},
		}
	case "google":
		return []menuOption{
			{label: "1.0 (default)", value: ""},
			{label: "0.50 (very slow)", value: "0.50"},
			{label: "0.75 (slower)", value: "0.75"},
			{label: "0.90 (slightly slow)", value: "0.90"},
			{label: "1.10 (slightly fast)", value: "1.10"},
			{label: "1.25 (faster)", value: "1.25"},
			{label: "1.50 (fast)", value: "1.50"},
		}
	default:
		return []menuOption{
			{label: "Not supported (Gemini)", value: ""},
		}
	}
}

// ttsStabilityOptions returns stability presets (ElevenLabs only).
func ttsStabilityOptions(provider string) []menuOption {
	if provider == "auto" {
		provider = "gemini"
	}
	if provider != "elevenlabs" {
		return []menuOption{
			{label: "Not supported (" + provider + ")", value: ""},
		}
	}
	return []menuOption{
		{label: "0.50 (default)", value: ""},
		{label: "0.20 (very expressive)", value: "0.20"},
		{label: "0.35 (expressive)", value: "0.35"},
		{label: "0.50 (balanced)", value: "0.50"},
		{label: "0.65 (stable)", value: "0.65"},
		{label: "0.80 (very stable)", value: "0.80"},
	}
}

// ttsPitchOptions returns pitch presets (Google only).
func ttsPitchOptions(provider string) []menuOption {
	if provider == "auto" {
		provider = "gemini"
	}
	if provider != "google" {
		return []menuOption{
			{label: "Not supported (" + provider + ")", value: ""},
		}
	}
	return []menuOption{
		{label: "0.0 (default)", value: ""},
		{label: "-8.0 (much lower)", value: "-8.00"},
		{label: "-4.0 (lower)", value: "-4.00"},
		{label: "-2.0 (slightly lower)", value: "-2.00"},
		{label: "+2.0 (slightly higher)", value: "2.00"},
		{label: "+4.0 (higher)", value: "4.00"},
		{label: "+8.0 (much higher)", value: "8.00"},
	}
}

// formatOptions returns the show format choices.
func formatOptions() []menuOption {
	var opts []menuOption
	for _, name := range script.FormatNames() {
		label := script.FormatLabel(name)
		if name == "conversation" {
			label += " (default)"
		}
		opts = append(opts, menuOption{label: label, value: name})
	}
	return opts
}

func buildMenuItems(voiceCount int) []menuItem {
	effectiveProvider := flagTTS
	if effectiveProvider == "" {
		effectiveProvider = "auto"
	}

	voiceOpts, defaultV1, defaultV2, defaultV3 := buildVoiceOptionsForProvider(effectiveProvider)

	ttsModelVal := flagTTSModel
	if ttsModelVal == "" {
		ttsModelVal = defaultTTSModel(effectiveProvider)
	}

	formatVal := flagFormat
	if formatVal == "" {
		formatVal = "conversation"
	}

	items := []menuItem{
		// 0: Input
		{
			label:    "Input",
			value:    flagInput,
			required: true,
		},
		// 1: Output
		{
			label: "Output",
			value: flagOutput,
		},
		// 2: Topic
		{
			label: "Topic",
			value: flagTopic,
		},
		// 3: Format
		{
			label:   "Format",
			value:   formatVal,
			options: formatOptions(),
		},
		// 4: Tone
		{
			label: "Tone",
			value: flagTone,
			options: []menuOption{
				{label: "Casual - light and engaging (default)", value: "casual"},
				{label: "Technical - precise, domain-specific", value: "technical"},
				{label: "Educational - accessible, builds understanding", value: "educational"},
			},
		},
		// 5: Duration
		{
			label: "Duration",
			value: flagDuration,
			options: []menuOption{
				{label: "Short (~30 segments, ~8 min)", value: "short"},
				{label: "Standard (~60 segments, ~18 min) (default)", value: "standard"},
				{label: "Long (~100 segments, ~35 min)", value: "long"},
				{label: "Deep Dive (~200 segments, ~55 min)", value: "deep"},
			},
		},
		// 6: Styles
		{
			label: "Styles",
			value: flagStyle,
		},
		// 7: Model
		{
			label: "Model",
			value: flagModel,
			options: []menuOption{
				{label: "Haiku 4.5 (fast, affordable) (default)", value: "haiku"},
				{label: "Sonnet 4.5 (balanced)", value: "sonnet"},
				{label: "Gemini 3 Flash (fast)", value: "gemini-flash"},
				{label: "Gemini 3 Pro (powerful)", value: "gemini-pro"},
				{label: "Nova 2 Lite (cheapest, AWS)", value: "nova-lite"},
			},
		},
		// 8: Voices
		{
			label: "Voices",
			value: fmt.Sprintf("%d", voiceCount),
			options: []menuOption{
				{label: "1 - Solo monologue (Alex)", value: "1"},
				{label: "2 - Two hosts (Alex & Sam) (default)", value: "2"},
				{label: "3 - Roundtable (Alex, Sam & Jordan)", value: "3"},
			},
		},
		// 9: Voice 1
		{
			label:   "Voice 1 (Alex)",
			value:   defaultV1,
			options: voiceOpts,
		},
		// 10: Voice 2
		{
			label:   "Voice 2 (Sam)",
			value:   defaultV2,
			options: voiceOpts,
		},
	}

	// Add Voice 3 if needed
	if voiceCount >= 3 {
		items = append(items, menuItem{
			label:   "Voice 3 (Jordan)",
			value:   defaultV3,
			options: voiceOpts,
		})
	}

	// Audio Settings separator
	items = append(items, menuItem{
		label:     "Audio Settings",
		separator: true,
	})

	// Provider
	items = append(items, menuItem{
		label: "Provider",
		value: effectiveProvider,
		options: []menuOption{
			{label: "Auto (from voice selection) (default)", value: "auto"},
			{label: "Gemini (multi-speaker batch)", value: "gemini"},
			{label: "Vertex Express (API key, higher quotas)", value: "vertex-express"},
			{label: "ElevenLabs (premium voices)", value: "elevenlabs"},
			{label: "Google Cloud TTS (Chirp 3 HD)", value: "google"},
			{label: "AWS Polly (Generative voices)", value: "polly"},
		},
	})

	// TTS Model
	items = append(items, menuItem{
		label:   "TTS Model",
		value:   ttsModelVal,
		options: ttsModelOptions(effectiveProvider),
	})

	// TTS Speed
	items = append(items, menuItem{
		label:   "TTS Speed",
		value:   formatFloat(flagTTSSpeed),
		options: ttsSpeedOptions(effectiveProvider),
	})

	// TTS Stability
	items = append(items, menuItem{
		label:   "TTS Stability",
		value:   formatFloat(flagTTSStability),
		options: ttsStabilityOptions(effectiveProvider),
	})

	// TTS Pitch
	items = append(items, menuItem{
		label:   "TTS Pitch",
		value:   formatFloat(flagTTSPitch),
		options: ttsPitchOptions(effectiveProvider),
	})

	// Generate button at the end
	items = append(items, menuItem{
		label: ">>> Generate <<<",
	})

	// Pre-select cursor position for options
	for i := range items {
		if len(items[i].options) > 0 {
			for j, opt := range items[i].options {
				if opt.value == items[i].value {
					items[i].cursor = j
					break
				}
			}
		}
	}

	return items
}

func initialTUIModel() tuiModel {
	vc := flagVoices
	if vc < 1 || vc > 3 {
		vc = 2
	}
	return tuiModel{
		items:      buildMenuItems(vc),
		cursor:     idxInput,
		state:      stateMenu,
		styles:     map[string]bool{},
		voiceCount: vc,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) generateIdx() int {
	return len(m.items) - 1
}

func (m tuiModel) isTextInput(idx int) bool {
	return idx == idxInput || idx == idxOutput || idx == idxTopic
}

// providerIdx returns the index of the Provider field.
func (m tuiModel) providerIdx() int {
	// Provider is after the separator, which is after voice items
	base := idxVoice2 + 1
	if m.voiceCount >= 3 {
		base++
	}
	return base + 1 // +1 for separator
}

// ttsModelIdx returns the index of the TTS Model field.
func (m tuiModel) ttsModelIdx() int {
	return m.providerIdx() + 1
}

// ttsSpeedIdx returns the index of the TTS Speed field.
func (m tuiModel) ttsSpeedIdx() int {
	return m.providerIdx() + 2
}

// ttsStabilityIdx returns the index of the TTS Stability field.
func (m tuiModel) ttsStabilityIdx() int {
	return m.providerIdx() + 3
}

// ttsPitchIdx returns the index of the TTS Pitch field.
func (m tuiModel) ttsPitchIdx() int {
	return m.providerIdx() + 4
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateMenu:
			return m.updateMenu(msg)
		case stateEditing:
			return m.updateEditing(msg)
		case stateStylePicker:
			return m.updateStylePicker(msg)
		case stateInputPicker:
			return m.updateInputPicker(msg)
		}
	}
	return m, nil
}

func (m tuiModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelled = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			// Skip separators
			if m.items[m.cursor].separator {
				if m.cursor > 0 {
					m.cursor--
				}
			}
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			// Skip separators
			if m.items[m.cursor].separator {
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			}
		}

	case "enter", " ":
		// Skip separators
		if m.items[m.cursor].separator {
			return m, nil
		}

		if m.cursor == m.generateIdx() {
			// Validate required fields
			if m.items[idxInput].value == "" {
				m.err = fmt.Errorf("Input is required")
				return m, nil
			}
			m.confirmed = true
			return m, tea.Quit
		}

		// Input field opens input type picker
		if m.cursor == idxInput {
			m.state = stateInputPicker
			m.inputCursor = 0
			m.err = nil
			return m, nil
		}

		// Output/Topic are text fields
		if m.isTextInput(m.cursor) {
			m.state = stateEditing
			m.items[m.cursor].editing = true
			m.err = nil
			return m, nil
		}

		// Styles uses multi-select
		if m.cursor == idxStyle {
			m.state = stateStylePicker
			m.styleCursor = 0
			m.err = nil
			return m, nil
		}

		// All others: open option selector
		if len(m.items[m.cursor].options) > 0 {
			m.state = stateEditing
			m.items[m.cursor].editing = true
			m.err = nil
			return m, nil
		}
	}
	return m, nil
}

var inputPickerOptions = []menuOption{
	{label: "Enter URL", value: "url"},
	{label: "Enter file path", value: "file"},
	{label: "Paste from clipboard", value: "clipboard"},
}

func (m tuiModel) updateInputPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		opt := inputPickerOptions[m.inputCursor]
		switch opt.value {
		case "url", "file":
			// Transition to text editing for URL or file path
			m.state = stateEditing
			m.items[idxInput].editing = true
			m.items[idxInput].value = ""
			return m, nil
		case "clipboard":
			// Read clipboard and set value
			content, err := readClipboard()
			if err != nil {
				m.err = fmt.Errorf("clipboard read failed: %v", err)
				m.state = stateMenu
				return m, nil
			}
			if strings.TrimSpace(content) == "" {
				m.err = fmt.Errorf("clipboard is empty")
				m.state = stateMenu
				return m, nil
			}
			// Save to temp file
			path, err := saveToTempFile(content)
			if err != nil {
				m.err = fmt.Errorf("save clipboard content: %v", err)
				m.state = stateMenu
				return m, nil
			}
			words := len(strings.Fields(content))
			m.items[idxInput].value = path
			m.items[idxInput].label = fmt.Sprintf("Input (clipboard: %d words)", words)
			m.state = stateMenu
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		}

	case "esc":
		m.state = stateMenu
		return m, nil

	case "up", "k":
		if m.inputCursor > 0 {
			m.inputCursor--
		}

	case "down", "j":
		if m.inputCursor < len(inputPickerOptions)-1 {
			m.inputCursor++
		}
	}
	return m, nil
}

func (m tuiModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := m.cursor
	item := &m.items[idx]

	// Text input for Input/Output/Topic
	if m.isTextInput(idx) {
		switch msg.String() {
		case "enter":
			item.editing = false
			m.state = stateMenu
			// Auto-advance to next item
			if m.cursor < len(m.items)-1 {
				m.cursor++
				if m.items[m.cursor].separator {
					m.cursor++
				}
			}
			return m, nil
		case "esc":
			item.editing = false
			m.state = stateMenu
			return m, nil
		case "backspace":
			if len(item.value) > 0 {
				item.value = item.value[:len(item.value)-1]
			}
			return m, nil
		case "ctrl+u":
			item.value = ""
			return m, nil
		default:
			// Accept typed characters and pasted text
			if msg.Type == tea.KeyRunes {
				item.value += string(msg.Runes)
			}
			return m, nil
		}
	}

	// Option selector for other fields
	switch msg.String() {
	case "enter", " ":
		if item.cursor >= 0 && item.cursor < len(item.options) {
			item.value = item.options[item.cursor].value
		}
		item.editing = false
		m.state = stateMenu

		// If provider changed, rebuild TTS options and voice lists
		provIdx := m.providerIdx()
		if idx == provIdx {
			provider := item.value
			flagTTS = provider

			// Rebuild TTS Model options
			ttsIdx := m.ttsModelIdx()
			m.items[ttsIdx].options = ttsModelOptions(provider)
			m.items[ttsIdx].value = defaultTTSModel(provider)
			m.items[ttsIdx].cursor = 0

			// Rebuild Speed/Stability/Pitch options
			speedIdx := m.ttsSpeedIdx()
			m.items[speedIdx].options = ttsSpeedOptions(provider)
			m.items[speedIdx].value = ""
			m.items[speedIdx].cursor = 0
			stabIdx := m.ttsStabilityIdx()
			m.items[stabIdx].options = ttsStabilityOptions(provider)
			m.items[stabIdx].value = ""
			m.items[stabIdx].cursor = 0
			pitchIdx := m.ttsPitchIdx()
			m.items[pitchIdx].options = ttsPitchOptions(provider)
			m.items[pitchIdx].value = ""
			m.items[pitchIdx].cursor = 0

			// Rebuild voice options for new provider
			voiceOpts, dv1, dv2, dv3 := buildVoiceOptionsForProvider(provider)
			m.items[idxVoice1].options = voiceOpts
			m.items[idxVoice1].value = dv1
			m.items[idxVoice1].cursor = 0
			for j, opt := range voiceOpts {
				if opt.value == dv1 {
					m.items[idxVoice1].cursor = j
					break
				}
			}
			m.items[idxVoice2].options = voiceOpts
			m.items[idxVoice2].value = dv2
			m.items[idxVoice2].cursor = 0
			for j, opt := range voiceOpts {
				if opt.value == dv2 {
					m.items[idxVoice2].cursor = j
					break
				}
			}
			if m.voiceCount >= 3 {
				v3Idx := idxVoice2 + 1
				m.items[v3Idx].options = voiceOpts
				m.items[v3Idx].value = dv3
				m.items[v3Idx].cursor = 0
				for j, opt := range voiceOpts {
					if opt.value == dv3 {
						m.items[v3Idx].cursor = j
						break
					}
				}
			}
		}

		// If voices count changed, rebuild the menu
		if idx == idxVoices {
			newCount := 2
			switch item.value {
			case "1":
				newCount = 1
			case "3":
				newCount = 3
			}
			if newCount != m.voiceCount {
				m.voiceCount = newCount
				m.rebuildForVoiceCount()
			}
		}

		// Auto-advance
		if m.cursor < len(m.items)-1 {
			m.cursor++
			if m.items[m.cursor].separator {
				m.cursor++
			}
		}
		return m, nil

	case "esc":
		item.editing = false
		m.state = stateMenu
		return m, nil

	case "up", "k":
		if item.cursor > 0 {
			item.cursor--
		}

	case "down", "j":
		if item.cursor < len(item.options)-1 {
			item.cursor++
		}
	}
	return m, nil
}

var styleOptions = []menuOption{
	{label: "Humor - witty banter, jokes, playful comebacks", value: "humor"},
	{label: "Wow - dramatic reveals, mind-blown moments", value: "wow"},
	{label: "Serious - gravitas, reflective, no jokes", value: "serious"},
	{label: "Debate - hosts disagree, push back, tension", value: "debate"},
	{label: "Storytelling - narrative arc, suspense, foreshadowing", value: "storytelling"},
}

func (m tuiModel) updateStylePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Commit selections
		var selected []string
		for _, opt := range styleOptions {
			if m.styles[opt.value] {
				selected = append(selected, opt.value)
			}
		}
		m.items[idxStyle].value = strings.Join(selected, ", ")
		m.state = stateMenu
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, nil

	case "esc":
		m.state = stateMenu
		return m, nil

	case " ", "x":
		// Toggle current style
		opt := styleOptions[m.styleCursor]
		m.styles[opt.value] = !m.styles[opt.value]

	case "up", "k":
		if m.styleCursor > 0 {
			m.styleCursor--
		}

	case "down", "j":
		if m.styleCursor < len(styleOptions)-1 {
			m.styleCursor++
		}
	}
	return m, nil
}

func (m *tuiModel) rebuildForVoiceCount() {
	// Preserve current values by index name
	saved := map[string]string{
		"input":    m.items[idxInput].value,
		"output":   m.items[idxOutput].value,
		"topic":    m.items[idxTopic].value,
		"format":   m.items[idxFormat].value,
		"tone":     m.items[idxTone].value,
		"duration": m.items[idxDuration].value,
		"style":    m.items[idxStyle].value,
		"model":    m.items[idxModel].value,
	}

	// Save TTS settings by dynamic index
	provIdx := m.providerIdx()
	if provIdx < len(m.items) {
		saved["provider"] = m.items[provIdx].value
	}
	ttsIdx := m.ttsModelIdx()
	if ttsIdx < len(m.items) {
		saved["ttsmodel"] = m.items[ttsIdx].value
	}
	speedIdx := m.ttsSpeedIdx()
	if speedIdx < len(m.items) {
		saved["speed"] = m.items[speedIdx].value
	}
	stabIdx := m.ttsStabilityIdx()
	if stabIdx < len(m.items) {
		saved["stability"] = m.items[stabIdx].value
	}
	pitchIdx := m.ttsPitchIdx()
	if pitchIdx < len(m.items) {
		saved["pitch"] = m.items[pitchIdx].value
	}

	// Rebuild items with new voice count
	m.items = buildMenuItems(m.voiceCount)

	// Restore values
	m.items[idxInput].value = saved["input"]
	m.items[idxOutput].value = saved["output"]
	m.items[idxTopic].value = saved["topic"]
	m.items[idxFormat].value = saved["format"]
	m.items[idxTone].value = saved["tone"]
	m.items[idxDuration].value = saved["duration"]
	m.items[idxStyle].value = saved["style"]
	m.items[idxModel].value = saved["model"]
	m.items[idxVoices].value = fmt.Sprintf("%d", m.voiceCount)

	// Restore TTS settings
	newProvIdx := m.providerIdx()
	if newProvIdx < len(m.items) && saved["provider"] != "" {
		m.items[newProvIdx].value = saved["provider"]
	}
	newTTSIdx := m.ttsModelIdx()
	if newTTSIdx < len(m.items) && saved["ttsmodel"] != "" {
		m.items[newTTSIdx].value = saved["ttsmodel"]
	}
	newSpeedIdx := m.ttsSpeedIdx()
	if newSpeedIdx < len(m.items) {
		m.items[newSpeedIdx].value = saved["speed"]
	}
	newStabIdx := m.ttsStabilityIdx()
	if newStabIdx < len(m.items) {
		m.items[newStabIdx].value = saved["stability"]
	}
	newPitchIdx := m.ttsPitchIdx()
	if newPitchIdx < len(m.items) {
		m.items[newPitchIdx].value = saved["pitch"]
	}

	// Re-select cursor positions for option items
	for i := range m.items {
		if len(m.items[i].options) > 0 {
			for j, opt := range m.items[i].options {
				if opt.value == m.items[i].value {
					m.items[i].cursor = j
					break
				}
			}
		}
	}

	// Ensure cursor is still in range
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

// scrollWindow computes the visible range of options for a scrollable picker.
func scrollWindow(cursor, total, visible int) (start, end int) {
	if total <= visible {
		return 0, total
	}
	// Center cursor in visible window
	half := visible / 2
	start = cursor - half
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > total {
		end = total
		start = end - visible
	}
	return start, end
}

func (m tuiModel) View() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render("Podcaster")
	header := headerBorder.Render(title)
	b.WriteString(header)
	b.WriteString("\n")

	genIdx := m.generateIdx()

	for i, item := range m.items {
		isActive := m.cursor == i

		// Separator
		if item.separator {
			b.WriteString("\n")
			b.WriteString("  " + separatorStyle.Render("─── "+item.label+" ───"))
			b.WriteString("\n")
			continue
		}

		// Generate button
		if i == genIdx {
			b.WriteString("\n")
			if isActive {
				b.WriteString("  " + buttonStyle.Render(" Generate "))
			} else {
				b.WriteString("  " + buttonDimStyle.Render(" Generate "))
			}
			b.WriteString("\n")
			continue
		}

		// Cursor indicator
		cursor := "  "
		if isActive {
			cursor = cursorStyle.Render("> ")
		}

		// Label
		label := item.label
		if item.required {
			label = label + requiredStyle.Render("*")
		}
		renderedLabel := menuLabelStyle.Render(label)

		// Value display
		var renderedValue string
		if item.editing && m.isTextInput(i) {
			// Show text input with blinking cursor
			renderedValue = menuValueStyle.Render(item.value + "_")
		} else if item.value == "" {
			// Show contextual placeholder text
			placeholder := "(not set)"
			switch {
			case i == idxTopic:
				placeholder = "(optional — focus on specific aspect)"
			case i == idxOutput:
				placeholder = "(auto: based on episode title)"
			case i == m.ttsSpeedIdx() || i == m.ttsStabilityIdx() || i == m.ttsPitchIdx():
				if len(item.options) > 0 {
					placeholder = item.options[0].label
				}
			}
			renderedValue = menuValueDimStyle.Render(placeholder)
		} else {
			displayVal := item.value
			// Show friendly label for option-based items
			for _, opt := range item.options {
				if opt.value == item.value {
					displayVal = opt.label
					break
				}
			}
			renderedValue = menuValueStyle.Render(displayVal)
		}

		b.WriteString(cursor + renderedLabel + " " + renderedValue + "\n")

		// Show expanded options when editing (with scrolling for large lists)
		if item.editing && len(item.options) > 0 && !m.isTextInput(i) {
			if len(item.options) > maxVisibleOptions {
				start, end := scrollWindow(item.cursor, len(item.options), maxVisibleOptions)
				if start > 0 {
					b.WriteString(dimStyle.Render(fmt.Sprintf("    \u2191 %d more", start)) + "\n")
				}
				for j := start; j < end; j++ {
					opt := item.options[j]
					if j == item.cursor {
						b.WriteString(selectedOptionStyle.Render("> "+opt.label) + "\n")
					} else {
						b.WriteString(optionStyle.Render("  "+opt.label) + "\n")
					}
				}
				if end < len(item.options) {
					b.WriteString(dimStyle.Render(fmt.Sprintf("    \u2193 %d more", len(item.options)-end)) + "\n")
				}
			} else {
				for j, opt := range item.options {
					if j == item.cursor {
						b.WriteString(selectedOptionStyle.Render("> "+opt.label) + "\n")
					} else {
						b.WriteString(optionStyle.Render("  "+opt.label) + "\n")
					}
				}
			}
		}
	}

	// Input picker overlay
	if m.state == stateInputPicker {
		b.WriteString("\n")
		for j, opt := range inputPickerOptions {
			prefix := "  "
			if j == m.inputCursor {
				prefix = cursorStyle.Render("> ")
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", prefix, opt.label))
		}
	}

	// Style picker overlay
	if m.state == stateStylePicker {
		b.WriteString("\n")
		for j, opt := range styleOptions {
			checked := " "
			if m.styles[opt.value] {
				checked = "x"
			}
			prefix := "  "
			if j == m.styleCursor {
				prefix = cursorStyle.Render("> ")
			}
			b.WriteString(fmt.Sprintf("  %s[%s] %s\n", prefix, checked, opt.label))
		}
	}

	// Error message
	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  Error: "+m.err.Error()) + "\n")
	}

	// Help text
	switch m.state {
	case stateMenu:
		b.WriteString(helpStyle.Render("  j/k or arrows to navigate | enter to edit | q to quit"))
	case stateEditing:
		if m.isTextInput(m.cursor) {
			b.WriteString(helpStyle.Render("  type value | enter to confirm | esc to cancel | ctrl+u to clear"))
		} else {
			b.WriteString(helpStyle.Render("  j/k or arrows to pick | enter to select | esc to cancel"))
		}
	case stateStylePicker:
		b.WriteString(helpStyle.Render("  j/k or arrows to navigate | space to toggle | enter to confirm | esc to cancel"))
	case stateInputPicker:
		b.WriteString(helpStyle.Render("  j/k or arrows to pick | enter to select | esc to cancel"))
	}
	b.WriteString("\n")

	return b.String()
}

func runInteractiveSetup() error {
	m := initialTUIModel()

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	final := result.(tuiModel)
	if final.cancelled {
		return fmt.Errorf("cancelled")
	}
	if !final.confirmed {
		return fmt.Errorf("generation cancelled")
	}

	// Apply selections to flags
	flagInput = final.items[idxInput].value
	flagOutput = final.items[idxOutput].value
	flagTopic = final.items[idxTopic].value
	flagFormat = final.items[idxFormat].value
	flagTone = final.items[idxTone].value
	flagDuration = final.items[idxDuration].value
	flagModel = final.items[idxModel].value

	if final.items[idxStyle].value != "" {
		flagStyle = strings.ReplaceAll(final.items[idxStyle].value, " ", "")
	}

	// Provider
	provIdx := final.providerIdx()
	providerVal := final.items[provIdx].value
	if providerVal == "auto" {
		// Infer from voice selections
		providerVal = "gemini" // default fallback
		v1 := final.items[idxVoice1].value
		if p, _ := tts.ParseVoiceSpec(v1); p != "" {
			providerVal = p
		}
	}
	flagTTS = providerVal

	// TTS settings
	ttsIdx := final.ttsModelIdx()
	flagTTSModel = final.items[ttsIdx].value
	flagTTSSpeed = parseFloat(final.items[final.ttsSpeedIdx()].value)
	flagTTSStability = parseFloat(final.items[final.ttsStabilityIdx()].value)
	flagTTSPitch = parseFloat(final.items[final.ttsPitchIdx()].value)

	// Voice values include provider:voiceID format from TUI
	flagVoice1 = final.items[idxVoice1].value
	flagVoice2 = final.items[idxVoice2].value
	flagVoices = final.voiceCount

	// Voice 3 if present
	if final.voiceCount >= 3 {
		v3Idx := idxVoice2 + 1
		if v3Idx < len(final.items)-1 {
			flagVoice3 = final.items[v3Idx].value
		}
	}

	return nil
}

// readClipboard reads the system clipboard (macOS).
func readClipboard() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// saveToTempFile saves content to a temp file in podcaster-output/tempfiles/.
func saveToTempFile(content string) (string, error) {
	dir := filepath.Join(pipeline.OutputBaseDir, "tempfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create tempfiles dir: %w", err)
	}
	name := fmt.Sprintf("input-%s.txt", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return path, nil
}
