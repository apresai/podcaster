package script

import "fmt"

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
	prompt += fmt.Sprintf("TARGET LENGTH: %s\n\n", segmentGuidance)
	prompt += fmt.Sprintf("SOURCE MATERIAL:\n%s", content)

	return prompt
}

func durationToSegments(duration string) string {
	switch duration {
	case "short":
		return "15-25 segments (~5 minutes of audio)"
	case "long":
		return "60-100 segments (~20 minutes of audio)"
	default: // medium
		return "30-50 segments (~10 minutes of audio)"
	}
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
