package script

import (
	"fmt"
	"strings"
)

func buildSystemPrompt(personas []Persona) string {
	switch len(personas) {
	case 1:
		return buildMonologueSystemPrompt(personas[0])
	case 3:
		return buildThreeHostSystemPrompt(personas[0], personas[1], personas[2])
	default:
		return buildTwoHostSystemPrompt(personas[0], personas[1])
	}
}

func buildMonologueSystemPrompt(host Persona) string {
	return fmt.Sprintf(`You are a podcast script writer. You create engaging single-host monologues from written content.

HOST:

%s (%s) — %s
Background: %s
Role: Solo host. Drives the narrative, explains concepts, asks rhetorical questions, and shares insights directly with the listener.
Speaking style: %s
Signature phrases: %s
Expertise: %s

CRITICAL RULE — EDITORIAL INDEPENDENCE:
%s
This host is an independent commentator. They discuss companies and products from the OUTSIDE. Any slip into first-person identification with a subject ("we launched", "our platform") is a serious error.

RULES:
1. The monologue must be based on the source material — do not hallucinate facts
2. Use natural conversational language — contractions, informal phrasing, brief asides to the listener
3. Include a clear introduction, exploration of key themes, and conclusion
4. Each segment should be 1-3 sentences of natural speech (not paragraphs)
5. Transitions between topics should feel organic, not forced
6. CRITICAL: You MUST stay within the target segment count specified. Do NOT exceed the maximum. Prioritize depth on fewer topics over breadth across many.
7. BANNED FILLER PHRASES — Never use these: "That's a great point", "Absolutely", "Exactly", "That's fascinating", "I love that", "So true", "100 percent", "You nailed it", "That's so interesting", "Right, right".
8. SENTENCE VARIETY — Mix short punchy reactions (1-5 words) with longer analytical statements. Alternate between rhetorical questions, declarations, anecdotes, and data points.
9. DISTINCT SPEAKING FINGERPRINT — The host must sound like themselves. Use their signature phrases and speaking patterns described above.
10. ANTI-AI SOUND — Avoid overly smooth transitions and Wikipedia-summary style segments. Include half-finished thoughts, mid-sentence mind-changes, and moments of genuine struggle to articulate an idea.

OUTPUT FORMAT:
Return ONLY valid JSON matching this exact structure (no markdown fences, no extra text):
{
  "title": "Episode title that captures the main topic",
  "summary": "One sentence summary of what the host discusses",
  "segments": [
    {"speaker": "%s", "text": "Hey everyone, welcome back..."},
    {"speaker": "%s", "text": "So today I want to dig into..."}
  ]
}

IMPORTANT: Output raw JSON only. No markdown code fences. No text before or after the JSON.`,
		host.Name, host.FullName, host.Role,
		host.Background,
		host.SpeakingStyle,
		host.Catchphrases,
		host.Expertise,
		host.Independence,
		host.Name, host.Name,
	)
}

func buildTwoHostSystemPrompt(alex, sam Persona) string {
	return fmt.Sprintf(`You are a podcast script writer. You create engaging two-host conversations from written content.

HOSTS:

%s (%s) — %s
Background: %s
Role: %s
Speaking style: %s
Signature phrases: %s
Expertise: %s
Co-host dynamic: %s

%s (%s) — %s
Background: %s
Role: %s
Speaking style: %s
Signature phrases: %s
Expertise: %s
Co-host dynamic: %s

CRITICAL RULE — EDITORIAL INDEPENDENCE:
%s
%s
These hosts are independent commentators. They discuss companies and products from the OUTSIDE. Any slip into first-person identification with a subject ("we launched", "our platform") is a serious error.

RULES:
1. The conversation must be based on the source material — do not hallucinate facts
2. Both hosts must participate throughout — neither should dominate (each gets at least 30%% of segments)
3. %s drives topics forward; %s probes, questions, and adds depth
4. Use natural conversational language — contractions, informal phrasing, brief reactions
5. Include a clear introduction, exploration of key themes, and conclusion
6. Each segment should be 1-3 sentences of natural speech (not paragraphs)
7. Transitions between topics should feel organic, not forced
8. CRITICAL: You MUST stay within the target segment count specified. Do NOT exceed the maximum. Prioritize depth on fewer topics over breadth across many.
9. BANNED FILLER PHRASES — Never use these: "That's a great point", "Absolutely", "Exactly", "That's fascinating", "I love that", "So true", "100 percent", "You nailed it", "That's so interesting", "Right, right". Instead, react with specific, content-tied responses that show genuine engagement with the actual topic.
10. SENTENCE VARIETY — Mix short punchy reactions (1-5 words) with longer analytical statements. Alternate between questions, declarations, anecdotes, and data points. Never repeat the same conversational pattern three times in a row.
11. DISTINCT SPEAKING FINGERPRINTS — Each host must sound like themselves. Use their signature phrases and speaking patterns described above.
12. ANTI-AI SOUND — Avoid overly smooth transitions, perfectly balanced turn-taking, and Wikipedia-summary style segments. Include natural interruptions, half-finished thoughts, mid-sentence mind-changes, and moments where a host genuinely struggles to articulate an idea.

OUTPUT FORMAT:
Return ONLY valid JSON matching this exact structure (no markdown fences, no extra text):
{
  "title": "Episode title that captures the main topic",
  "summary": "One sentence summary of what the hosts discuss",
  "segments": [
    {"speaker": "%s", "text": "Welcome to the show..."},
    {"speaker": "%s", "text": "Thanks %s..."}
  ]
}

IMPORTANT: Output raw JSON only. No markdown code fences. No text before or after the JSON.`,
		alex.Name, alex.FullName, alex.Role,
		alex.Background,
		alex.Role,
		alex.SpeakingStyle,
		alex.Catchphrases,
		alex.Expertise,
		alex.Relationship,
		sam.Name, sam.FullName, sam.Role,
		sam.Background,
		sam.Role,
		sam.SpeakingStyle,
		sam.Catchphrases,
		sam.Expertise,
		sam.Relationship,
		alex.Independence,
		sam.Independence,
		alex.Name, sam.Name,
		alex.Name, sam.Name, alex.Name,
	)
}

func buildThreeHostSystemPrompt(alex, sam, jordan Persona) string {
	return fmt.Sprintf(`You are a podcast script writer. You create engaging three-host roundtable conversations from written content.

HOSTS:

%s (%s) — %s
Background: %s
Role: %s
Speaking style: %s
Signature phrases: %s
Expertise: %s
Co-host dynamic: %s

%s (%s) — %s
Background: %s
Role: %s
Speaking style: %s
Signature phrases: %s
Expertise: %s
Co-host dynamic: %s

%s (%s) — %s
Background: %s
Role: %s
Speaking style: %s
Signature phrases: %s
Expertise: %s
Co-host dynamic: %s

CRITICAL RULE — EDITORIAL INDEPENDENCE:
%s
%s
%s
These hosts are independent commentators. They discuss companies and products from the OUTSIDE. Any slip into first-person identification with a subject ("we launched", "our platform") is a serious error.

RULES:
1. The conversation must be based on the source material — do not hallucinate facts
2. All three hosts must participate throughout — each gets at least 20%% of segments
3. %s drives topics forward; %s probes and adds analytical depth; %s brings contrarian takes and real-world grounding
4. Use natural conversational language — contractions, informal phrasing, brief reactions
5. Include a clear introduction, exploration of key themes, and conclusion
6. Each segment should be 1-3 sentences of natural speech (not paragraphs)
7. Transitions between topics should feel organic, not forced
8. CRITICAL: You MUST stay within the target segment count specified. Do NOT exceed the maximum. Prioritize depth on fewer topics over breadth across many.
9. BANNED FILLER PHRASES — Never use these: "That's a great point", "Absolutely", "Exactly", "That's fascinating", "I love that", "So true", "100 percent", "You nailed it", "That's so interesting", "Right, right". Instead, react with specific, content-tied responses that show genuine engagement with the actual topic.
10. SENTENCE VARIETY — Mix short punchy reactions (1-5 words) with longer analytical statements. Alternate between questions, declarations, anecdotes, and data points. Never repeat the same conversational pattern three times in a row.
11. DISTINCT SPEAKING FINGERPRINTS — Each host must sound like themselves. Use their signature phrases and speaking patterns described above.
12. ANTI-AI SOUND — Avoid overly smooth transitions, perfectly balanced turn-taking, and Wikipedia-summary style segments. Include natural interruptions, half-finished thoughts, mid-sentence mind-changes, and moments where a host genuinely struggles to articulate an idea.
13. THREE-WAY DYNAMICS — Avoid ping-pong between just two hosts. Create moments where the third host jumps in, where two hosts agree against the third, or where all three have different takes. The three-way dynamic should feel like a real roundtable, not two conversations spliced together.

OUTPUT FORMAT:
Return ONLY valid JSON matching this exact structure (no markdown fences, no extra text):
{
  "title": "Episode title that captures the main topic",
  "summary": "One sentence summary of what the hosts discuss",
  "segments": [
    {"speaker": "%s", "text": "Welcome to the show..."},
    {"speaker": "%s", "text": "Thanks %s..."},
    {"speaker": "%s", "text": "Hey everyone, glad to be here..."}
  ]
}

IMPORTANT: Output raw JSON only. No markdown code fences. No text before or after the JSON.`,
		alex.Name, alex.FullName, alex.Role,
		alex.Background,
		alex.Role,
		alex.SpeakingStyle,
		alex.Catchphrases,
		alex.Expertise,
		alex.Relationship,
		sam.Name, sam.FullName, sam.Role,
		sam.Background,
		sam.Role,
		sam.SpeakingStyle,
		sam.Catchphrases,
		sam.Expertise,
		sam.Relationship,
		jordan.Name, jordan.FullName, jordan.Role,
		jordan.Background,
		jordan.Role,
		jordan.SpeakingStyle,
		jordan.Catchphrases,
		jordan.Expertise,
		jordan.Relationship,
		alex.Independence,
		sam.Independence,
		jordan.Independence,
		alex.Name, sam.Name, jordan.Name,
		alex.Name, sam.Name, alex.Name, jordan.Name,
	)
}

func buildUserPrompt(content string, opts GenerateOptions) string {
	segmentGuidance := durationToSegments(opts.Duration)

	format := opts.Format
	if format == "" {
		format = "conversation"
	}
	label := formatLabelForPrompt(format, opts.Voices)

	prompt := fmt.Sprintf(`<scratchpad>
Before writing the script, plan your approach:
1. Identify the 3-5 key themes in the source material
2. Plan the conversation arc: introduction → exploration → key insights → conclusion
3. Note which points deserve deeper discussion vs. brief mentions
4. Estimate how to hit the target segment count: %s
</scratchpad>

Convert the following content into a %s.

`, segmentGuidance, label)

	// Format directive
	prompt += fmt.Sprintf("FORMAT:\n%s\n\n", formatDirective(format))

	if opts.Topic != "" {
		prompt += fmt.Sprintf("FOCUS: Center the conversation on: %s\n\n", opts.Topic)
	}

	prompt += fmt.Sprintf("TONE: %s\n\n", toneDescription(opts.Tone))

	if styleDesc := styleDescription(opts.Styles, format); styleDesc != "" {
		prompt += fmt.Sprintf("STYLE DIRECTIVES:\n%s\n\n", styleDesc)
	}

	prompt += fmt.Sprintf("TARGET LENGTH: %s\n\n", segmentGuidance)
	prompt += fmt.Sprintf("SOURCE MATERIAL:\n%s", content)

	return prompt
}

func durationToSegments(duration string) string {
	switch duration {
	case "short":
		return "Exactly 30 segments (~8 minutes of audio)"
	case "long":
		return "Exactly 100 segments (~35 minutes of audio)"
	case "deep":
		return "Exactly 200 segments (~55 minutes of audio). This is a DEEP DIVE — cover every major point thoroughly, explore tangents, include extended exchanges."
	default: // standard (also covers "medium" alias)
		return "Exactly 60 segments (~18 minutes of audio)"
	}
}

// TargetSegments returns the target segment count for a given duration.
func TargetSegments(duration string) int {
	switch duration {
	case "short":
		return 30
	case "long":
		return 100
	case "deep":
		return 200
	default:
		return 60
	}
}

func styleDescription(styles []string, format string) string {
	if len(styles) == 0 {
		return ""
	}

	descriptions := map[string]string{
		"humor":        "Inject witty banter, clever one-liners, playful comebacks, running jokes, and lighthearted disagreements. Make listeners smile.",
		"wow":          "Use build-up → dramatic reveal structure. Include surprise reactions ('Wait, seriously?'), mind-blown moments, and genuine awe at revelations.",
		"serious":      "Maintain measured gravitas. Include reflective pauses, avoid jokes, convey the weight of stakes ('The implications are profound').",
		"debate":       "Hosts should disagree, push back, present alternatives. Include passionate advocacy and occasional concessions. Real intellectual tension.",
		"storytelling": "Build a narrative arc with suspense and foreshadowing. Use 'What happened next?' moments, vivid scene-setting, and cliffhangers between segments.",
	}

	// Deduplicate: skip style if it's redundant with the format
	redundant := map[string]string{
		"debate":       "debate",
		"storytelling": "storytelling",
	}

	var parts []string
	for _, s := range styles {
		if redundant[s] == format {
			continue
		}
		if desc, ok := descriptions[s]; ok {
			parts = append(parts, fmt.Sprintf("- %s: %s", strings.ToUpper(s), desc))
		}
	}
	return strings.Join(parts, "\n")
}

func toneDescription(tone string) string {
	switch tone {
	case "technical":
		return "Technical and precise. Use domain-specific terminology. Assume the listener has relevant background knowledge. Focus on accuracy and nuance."
	case "educational":
		return "Educational and accessible. Explain concepts clearly for a general audience. Use analogies and examples. Build understanding progressively."
	default: // casual
		return "Casual and conversational. Keep it light and engaging. Use everyday language. Make complex ideas approachable."
	}
}
