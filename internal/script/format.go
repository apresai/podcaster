package script

import "fmt"

// FormatNames returns all valid show format values.
func FormatNames() []string {
	return []string{
		"conversation",
		"interview",
		"deep-dive",
		"explainer",
		"debate",
		"news",
		"storytelling",
		"challenger",
	}
}

// FormatLabel returns a human-readable label for display.
func FormatLabel(format string) string {
	labels := map[string]string{
		"conversation": "Casual Conversation",
		"interview":    "Structured Interview",
		"deep-dive":    "Investigative Deep Dive",
		"explainer":    "Educational Explainer",
		"debate":       "Point-Counterpoint",
		"news":         "News Briefing",
		"storytelling": "Narrative Storytelling",
		"challenger":   "Devil's Advocate",
	}
	if l, ok := labels[format]; ok {
		return l
	}
	return "Casual Conversation"
}

// formatLabelForPrompt returns the label used in the user prompt, adapted to voice count.
func formatLabelForPrompt(format string, voices int) string {
	hostDesc := "two-host"
	switch voices {
	case 1:
		hostDesc = "single-host"
	case 3:
		hostDesc = "three-host"
	}

	templates := map[string]string{
		"conversation": "%s podcast conversation",
		"interview":    "%s structured interview",
		"deep-dive":    "%s investigative deep dive",
		"explainer":    "%s educational explainer",
		"debate":       "%s point-counterpoint debate",
		"news":         "%s news briefing",
		"storytelling": "%s narrative storytelling episode",
		"challenger":   "%s devil's advocate session",
	}
	if t, ok := templates[format]; ok {
		return fmt.Sprintf(t, hostDesc)
	}
	return fmt.Sprintf("%s podcast conversation", hostDesc)
}

// formatDirective returns the structural prompt section for a given show format.
func formatDirective(format string) string {
	directives := map[string]string{
		"conversation": `STRUCTURE: Free-flowing conversation. Hosts riff naturally on the material, go on tangents,
circle back, and build on each other's ideas organically. No rigid segments — the conversation follows curiosity.
Feels like overhearing two smart friends at a coffee shop. Topics emerge and flow rather than being formally introduced.`,

		"interview": `STRUCTURE: Structured interview format. Host 1 acts as the interviewer with prepared questions
organized into clear chapters/sections. Host 2 acts as the subject matter expert providing detailed answers.
The interviewer guides the conversation through: (1) Background/context setting, (2) Key findings or developments,
(3) Deep dive into specific aspects, (4) Implications and what's next. Questions should build on each other
and follow up on interesting points.`,

		"deep-dive": `STRUCTURE: Investigative deep dive. Build the episode like a case being laid out — methodical,
evidence-layered, building to a conclusion. Start with the central question or mystery. Layer in evidence piece
by piece. Use "chapters" that each reveal a new dimension. Let hosts react to revelations in real-time.
Build tension toward a synthesis or conclusion. The pacing should feel like peeling back layers of an onion.`,

		"explainer": `STRUCTURE: Educational explainer format. Start with the core concept at its simplest level,
then progressively add complexity. Use a "wait, so does that mean..." pattern where one host asks clarifying
questions that push the explanation deeper. Heavy use of analogies and real-world examples. Include "mind-blown"
moments where a concept clicks. Structure: (1) Hook/why this matters, (2) Basic concept, (3) How it actually works,
(4) Surprising implications, (5) What this means going forward.`,

		"debate": `STRUCTURE: Point-counterpoint debate format. Hosts take clearly opposing positions on the topic.
Structure: (1) Frame the central question, (2) Host 1 presents Position A with evidence, (3) Host 2 presents
Position B with evidence, (4) Direct rebuttals — each host challenges the other's strongest points,
(5) Finding common ground or acknowledging irreconcilable differences, (6) Synthesis — what listeners should
take away. Disagreements must be substantive and evidence-based, not performative.`,

		"news": `STRUCTURE: News briefing format — tight, focused, single-story deep coverage.
(1) The headline — what happened, stated clearly and concisely, (2) Context — why this matters,
what led to this moment, (3) The facts — key details, data points, quotes from relevant parties,
(4) Analysis — what this means, who's affected, competing interpretations,
(5) What's next — implications, upcoming milestones, what to watch for.
Keep it focused on ONE story. No tangents. Every segment should advance understanding of the core story.`,

		"storytelling": `STRUCTURE: Narrative storytelling format. Build the episode around a story arc:
(1) The hook — start with a compelling moment or question that pulls listeners in,
(2) Setup — introduce the characters, context, and stakes,
(3) Rising tension — complications, twists, escalating stakes,
(4) Climax — the pivotal moment, key revelation, or turning point,
(5) Resolution — what happened next, lessons learned, lasting impact.
Use vivid scene-setting, "What happened next?" cliffhangers between sections, and emotional beats
alongside analytical ones.`,

		"challenger": `STRUCTURE: Devil's advocate format. Host 1 presents the topic and its conventional wisdom.
Host 2 relentlessly challenges every claim, assumption, and conclusion — not to be contrarian, but to
stress-test the ideas. Every assertion must survive scrutiny. Host 2 should ask "But what about...",
"How do you explain...", "Isn't it more likely that...". Host 1 must defend with evidence, concede when
the challenge is valid, and strengthen their argument through the pressure. The goal is truth through
adversarial collaboration, not winning.`,
	}
	if d, ok := directives[format]; ok {
		return d
	}
	return directives["conversation"]
}

// IsValidFormat returns true if the format name is recognized.
func IsValidFormat(format string) bool {
	for _, f := range FormatNames() {
		if f == format {
			return true
		}
	}
	return false
}
