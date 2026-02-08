package cli

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/apresai/podcaster/internal/tts"
)

// menuItem represents a single configurable option in the TUI.
type menuItem struct {
	label    string
	value    string
	options  []menuOption
	required bool
	editing  bool
	cursor   int // cursor within options when editing
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
)

// menu item indices — dynamic voice items shift based on voice count
const (
	idxInput        = 0
	idxOutput       = 1
	idxTopic        = 2
	idxModel        = 3
	idxTTS          = 4
	idxTTSModel     = 5
	idxTTSSpeed     = 6
	idxTTSStability = 7
	idxTTSPitch     = 8
	idxTone         = 9
	idxDuration     = 10
	idxStyle        = 11
	idxVoices       = 12
	idxVoice1       = 13
	idxVoice2       = 14
	// idxVoice3 = 15 when voiceCount >= 3
	// idxGenerate = last item
)

func defaultOutputFilename() string {
	return time.Now().Format("podcast-20060102-1504.mp3")
}

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
			if v.DefaultFor == "Voice 1" && p.name == flagTTS {
				defaultV1 = value
			}
			if v.DefaultFor == "Voice 2" && p.name == flagTTS {
				defaultV2 = value
			}
			if v.DefaultFor == "Voice 3" && p.name == flagTTS {
				defaultV3 = value
			}
		}
	}
	return
}

// ttsModelOptions returns the TTS model choices for a given provider.
func ttsModelOptions(provider string) []menuOption {
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
			{label: "Pro TTS (best quality, nuanced) (default)", value: "gemini-2.5-pro-tts"},
			{label: "Flash TTS (fast, good quality)", value: "gemini-2.5-flash-tts"},
		}
	default:
		return []menuOption{
			{label: "Chirp 3 HD (fixed)", value: ""},
		}
	}
}

// defaultTTSModel returns the default TTS model for a provider.
func defaultTTSModel(provider string) string {
	switch provider {
	case "elevenlabs":
		return "eleven_v3"
	case "gemini":
		return "gemini-2.5-pro-tts"
	default:
		return ""
	}
}

// ttsSpeedOptions returns speed presets for a given provider.
func ttsSpeedOptions(provider string) []menuOption {
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

func buildMenuItems(voiceCount int) []menuItem {
	voiceOpts, defaultV1, defaultV2, defaultV3 := buildAllVoiceOptions()

	// Use existing flag values or sensible defaults
	outputVal := flagOutput
	if outputVal == "" {
		outputVal = defaultOutputFilename()
	}

	ttsModelVal := flagTTSModel
	if ttsModelVal == "" {
		ttsModelVal = defaultTTSModel(flagTTS)
	}

	items := []menuItem{
		{
			label:    "Input",
			value:    flagInput,
			required: true,
		},
		{
			label: "Output",
			value: outputVal,
		},
		{
			label: "Topic",
			value: flagTopic,
		},
		{
			label: "Model",
			value: flagModel,
			options: []menuOption{
				{label: "Haiku 4.5 (fast, affordable) (default)", value: "haiku"},
				{label: "Sonnet 4.5 (balanced)", value: "sonnet"},
				{label: "Gemini Flash (fast)", value: "gemini-flash"},
				{label: "Gemini Pro (powerful)", value: "gemini-pro"},
			},
		},
		{
			label: "Default Provider",
			value: flagTTS,
			options: []menuOption{
				{label: "Gemini (multi-speaker batch) (default)", value: "gemini"},
				{label: "ElevenLabs (premium voices)", value: "elevenlabs"},
				{label: "Google Cloud TTS (Chirp 3 HD)", value: "google"},
			},
		},
		{
			label:   "TTS Model",
			value:   ttsModelVal,
			options: ttsModelOptions(flagTTS),
		},
		{
			label:   "TTS Speed",
			value:   formatFloat(flagTTSSpeed),
			options: ttsSpeedOptions(flagTTS),
		},
		{
			label:   "TTS Stability",
			value:   formatFloat(flagTTSStability),
			options: ttsStabilityOptions(flagTTS),
		},
		{
			label:   "TTS Pitch",
			value:   formatFloat(flagTTSPitch),
			options: ttsPitchOptions(flagTTS),
		},
		{
			label: "Tone",
			value: flagTone,
			options: []menuOption{
				{label: "Casual - light and engaging (default)", value: "casual"},
				{label: "Technical - precise, domain-specific", value: "technical"},
				{label: "Educational - accessible, builds understanding", value: "educational"},
			},
		},
		{
			label: "Duration",
			value: flagDuration,
			options: []menuOption{
				{label: "Short (~30 segments, ~10 min)", value: "short"},
				{label: "Standard (~50 segments, ~15 min) (default)", value: "standard"},
				{label: "Long (~75 segments, ~25 min)", value: "long"},
				{label: "Deep Dive (~200 segments, ~65 min)", value: "deep"},
			},
		},
		{
			label: "Styles",
			value: flagStyle,
		},
		{
			label: "Voices",
			value: fmt.Sprintf("%d", voiceCount),
			options: []menuOption{
				{label: "1 - Solo monologue (Alex)", value: "1"},
				{label: "2 - Two hosts (Alex & Sam) (default)", value: "2"},
				{label: "3 - Roundtable (Alex, Sam & Jordan)", value: "3"},
			},
		},
		{
			label:   "Voice 1 (Alex)",
			value:   defaultV1,
			options: voiceOpts,
		},
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
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}

	case "enter", " ":
		if m.cursor == m.generateIdx() {
			// Validate required fields
			if m.items[idxInput].value == "" {
				m.err = fmt.Errorf("Input is required")
				return m, nil
			}
			m.confirmed = true
			return m, tea.Quit
		}

		// Input/Output are text fields: open inline editor
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

func (m tuiModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	idx := m.cursor
	item := &m.items[idx]

	// Text input for Input/Output
	if m.isTextInput(idx) {
		switch msg.String() {
		case "enter":
			item.editing = false
			m.state = stateMenu
			// Auto-advance to next item
			if m.cursor < len(m.items)-1 {
				m.cursor++
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

		// If default provider changed, update TTS model/speed/stability/pitch options and voice defaults
		if idx == idxTTS {
			flagTTS = item.value // update so buildAllVoiceOptions picks new defaults

			// Rebuild TTS Model options for new provider
			m.items[idxTTSModel].options = ttsModelOptions(item.value)
			m.items[idxTTSModel].value = defaultTTSModel(item.value)
			m.items[idxTTSModel].cursor = 0

			// Rebuild Speed/Stability/Pitch options for new provider, reset to defaults
			m.items[idxTTSSpeed].options = ttsSpeedOptions(item.value)
			m.items[idxTTSSpeed].value = ""
			m.items[idxTTSSpeed].cursor = 0
			m.items[idxTTSStability].options = ttsStabilityOptions(item.value)
			m.items[idxTTSStability].value = ""
			m.items[idxTTSStability].cursor = 0
			m.items[idxTTSPitch].options = ttsPitchOptions(item.value)
			m.items[idxTTSPitch].value = ""
			m.items[idxTTSPitch].cursor = 0

			// Update voice defaults
			_, dv1, dv2, dv3 := buildAllVoiceOptions()
			m.items[idxVoice1].value = dv1
			m.items[idxVoice1].cursor = 0
			for j, opt := range m.items[idxVoice1].options {
				if opt.value == dv1 {
					m.items[idxVoice1].cursor = j
					break
				}
			}
			m.items[idxVoice2].value = dv2
			m.items[idxVoice2].cursor = 0
			for j, opt := range m.items[idxVoice2].options {
				if opt.value == dv2 {
					m.items[idxVoice2].cursor = j
					break
				}
			}
			if m.voiceCount >= 3 && len(m.items) > idxVoice2+1 {
				v3Idx := idxVoice2 + 1
				m.items[v3Idx].value = dv3
				m.items[v3Idx].cursor = 0
				for j, opt := range m.items[v3Idx].options {
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
	// Preserve current values
	savedInput := m.items[idxInput].value
	savedOutput := m.items[idxOutput].value
	savedTopic := m.items[idxTopic].value
	savedModel := m.items[idxModel].value
	savedTTS := m.items[idxTTS].value
	savedTTSModel := m.items[idxTTSModel].value
	savedSpeed := m.items[idxTTSSpeed].value
	savedStability := m.items[idxTTSStability].value
	savedPitch := m.items[idxTTSPitch].value
	savedTone := m.items[idxTone].value
	savedDuration := m.items[idxDuration].value
	savedStyle := m.items[idxStyle].value

	// Rebuild items with new voice count
	m.items = buildMenuItems(m.voiceCount)

	// Restore values
	m.items[idxInput].value = savedInput
	m.items[idxOutput].value = savedOutput
	m.items[idxTopic].value = savedTopic
	m.items[idxModel].value = savedModel
	m.items[idxTTS].value = savedTTS
	m.items[idxTTSModel].value = savedTTSModel
	m.items[idxTTSSpeed].value = savedSpeed
	m.items[idxTTSStability].value = savedStability
	m.items[idxTTSPitch].value = savedPitch
	m.items[idxTone].value = savedTone
	m.items[idxDuration].value = savedDuration
	m.items[idxStyle].value = savedStyle
	m.items[idxVoices].value = fmt.Sprintf("%d", m.voiceCount)

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
			switch i {
			case idxTopic:
				placeholder = "(optional — focus on specific aspect)"
			case idxTTSSpeed, idxTTSStability, idxTTSPitch:
				// Option pickers show "Default" label from first option
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

		// Show expanded options when editing
		if item.editing && len(item.options) > 0 && !m.isTextInput(i) {
			for j, opt := range item.options {
				if j == item.cursor {
					b.WriteString(selectedOptionStyle.Render("> " + opt.label) + "\n")
				} else {
					b.WriteString(optionStyle.Render("  " + opt.label) + "\n")
				}
			}
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
	flagModel = final.items[idxModel].value
	flagTTS = final.items[idxTTS].value
	flagTTSModel = final.items[idxTTSModel].value
	flagTTSSpeed = parseFloat(final.items[idxTTSSpeed].value)
	flagTTSStability = parseFloat(final.items[idxTTSStability].value)
	flagTTSPitch = parseFloat(final.items[idxTTSPitch].value)
	flagTone = final.items[idxTone].value
	flagDuration = final.items[idxDuration].value
	if final.items[idxStyle].value != "" {
		flagStyle = strings.ReplaceAll(final.items[idxStyle].value, " ", "")
	}
	// Voice values already include provider:voiceID format from TUI
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
