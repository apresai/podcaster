# Generate Persona

Generate a new `Persona` struct value for the podcaster project. Use this when creating a custom podcast host personality for a specific voice or character concept.

## Usage

```
/generate-persona <voice name or character description>
```

Examples:
- `/generate-persona Fenrir` — Generate a persona matching the Fenrir voice (excitable, deep male)
- `/generate-persona a skeptical economist` — Generate a persona for an economist character
- `/generate-persona Zephyr, laid-back science communicator` — Combine voice + character concept

## Instructions

1. Read `internal/script/personas.go` to understand the `Persona` struct and see the existing `DefaultAlexPersona` and `DefaultSamPersona` as format references.

2. Generate a complete `Persona` struct value with ALL fields populated:
   - **Name**: The speaker label used in script segments (e.g., "Alex" or "Sam" — must match an existing speaker name, or the user must update VoiceMap/validation)
   - **FullName**: A plausible full name for the character
   - **Background**: 2-3 sentences of career history and credentials that establish credibility. Make it specific (named publications, companies, years of experience) but fictional.
   - **Role**: One sentence defining their function in the conversation dynamic
   - **SpeakingStyle**: 3-4 sentences describing verbal patterns, sentence structure preferences, pacing, and how they express ideas. Be specific enough that an LLM can replicate the style.
   - **Catchphrases**: 4-6 signature phrases in quotes, comma-separated. These should feel natural and distinctive, not gimmicky.
   - **Expertise**: Comma-separated list of 4-6 subject matter strengths
   - **Relationship**: 2 sentences describing how they interact with their co-host
   - **Independence**: MUST include this exact clause (adjust name only): "You are an independent [role]. You are NOT affiliated with, employed by, or sponsored by any company, product, or person you discuss. NEVER use 'we' or 'our' when referring to any company or organization in the source material. Always maintain third-person distance: 'they', 'the company', 'the team'."

3. Output the persona as a Go variable declaration ready to paste into `personas.go`:

```go
var CustomPersona = Persona{
    Name:          "...",
    FullName:      "...",
    Background:    `...`,
    Role:          "...",
    SpeakingStyle: `...`,
    Catchphrases:  `...`,
    Expertise:     "...",
    Relationship:  "...",
    Independence:  "...",
}
```

## Quality Checks

- The speaking style must be distinct from both DefaultAlexPersona and DefaultSamPersona
- Catchphrases must NOT overlap with existing personas' catchphrases
- The independence clause MUST be present and complete
- Background should feel like a real media professional, not a caricature
- If a voice name is provided (e.g., "Fenrir"), match the persona's energy to the voice description from `internal/tts/gemini.go`, `internal/tts/express.go`, or `internal/tts/elevenlabs.go`
