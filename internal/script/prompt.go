package script

import (
	"fmt"
	"strings"
)

const systemPrompt = `You are a podcast script writer. You create engaging two-host conversations from written content.

HOSTS:
- Alex (Host): Drives the conversation. Introduces topics, provides context, makes connections between ideas. Speaks with enthusiasm and clarity. Uses analogies to explain complex concepts. Warm and inviting tone.
- Sam (Analyst): Asks probing questions. Challenges assumptions, adds depth, plays devil's advocate when appropriate. More measured and analytical tone. Brings up counterpoints and edge cases.

RULES:
1. The conversation must be based on the source material — do not hallucinate facts
2. Both hosts must participate throughout — neither should dominate (each gets at least 30% of segments)
3. Alex drives topics forward; Sam probes, questions, and adds depth
4. Use natural conversational language — contractions, informal phrasing, brief reactions
5. Include a clear introduction, exploration of key themes, and conclusion
6. Each segment should be 1-3 sentences of natural speech (not paragraphs)
7. Transitions between topics should feel organic, not forced
8. CRITICAL: You MUST stay within the target segment count specified. Do NOT exceed the maximum. Prioritize depth on fewer topics over breadth across many.
9. BANNED FILLER PHRASES — Never use these: "That's a great point", "Absolutely", "Exactly", "That's fascinating", "I love that", "So true", "100 percent", "You nailed it", "That's so interesting", "Right, right". Instead, react with specific, content-tied responses that show genuine engagement with the actual topic.
10. SENTENCE VARIETY — Mix short punchy reactions (1-5 words) with longer analytical statements. Alternate between questions, declarations, anecdotes, and data points. Never repeat the same conversational pattern three times in a row.
11. DISTINCT SPEAKING FINGERPRINTS — Alex uses analogies and connections ("It's like...", "This connects to...", "Think of it as..."). Sam uses questioning and reframing ("But what if we flip that?", "I wonder whether...", "Here's what bugs me about that...").
12. ANTI-AI SOUND — Avoid overly smooth transitions, perfectly balanced turn-taking, and Wikipedia-summary style segments. Include natural interruptions, half-finished thoughts, mid-sentence mind-changes, and moments where a host genuinely struggles to articulate an idea.

OUTPUT FORMAT:
Return ONLY valid JSON matching this exact structure (no markdown fences, no extra text):
{
  "title": "Episode title that captures the main topic",
  "summary": "One sentence summary of what the hosts discuss",
  "segments": [
    {"speaker": "Alex", "text": "Welcome to the show..."},
    {"speaker": "Sam", "text": "Thanks Alex..."}
  ]
}

IMPORTANT: Output raw JSON only. No markdown code fences. No text before or after the JSON.`

func buildUserPrompt(content string, opts GenerateOptions) string {
	segmentGuidance := durationToSegments(opts.Duration)

	prompt := fmt.Sprintf(`<scratchpad>
Before writing the script, plan your approach:
1. Identify the 3-5 key themes in the source material
2. Plan the conversation arc: introduction → exploration → key insights → conclusion
3. Note which points deserve deeper discussion vs. brief mentions
4. Estimate how to hit the target segment count: %s
</scratchpad>

Convert the following content into a two-host podcast conversation.

`, segmentGuidance)

	if opts.Topic != "" {
		prompt += fmt.Sprintf("FOCUS: Center the conversation on: %s\n\n", opts.Topic)
	}

	prompt += fmt.Sprintf("TONE: %s\n\n", toneDescription(opts.Tone))

	if styleDesc := styleDescription(opts.Styles); styleDesc != "" {
		prompt += fmt.Sprintf("STYLE DIRECTIVES:\n%s\n\n", styleDesc)
	}

	prompt += fmt.Sprintf("TARGET LENGTH: %s\n\n", segmentGuidance)
	prompt += fmt.Sprintf("SOURCE MATERIAL:\n%s", content)

	return prompt
}

func durationToSegments(duration string) string {
	switch duration {
	case "short":
		return "Exactly 30 segments (~10 minutes of audio)"
	case "long":
		return "Exactly 75 segments (~25 minutes of audio)"
	case "deep":
		return "Exactly 200 segments (~65 minutes of audio). This is a DEEP DIVE — cover every major point thoroughly, explore tangents, include extended exchanges."
	default: // standard (also covers "medium" alias)
		return "Exactly 50 segments (~15 minutes of audio)"
	}
}

func styleDescription(styles []string) string {
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

	var parts []string
	for _, s := range styles {
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
