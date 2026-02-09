# Refresh Voices

Update the default voice constants in the podcaster source code by fetching live voice lists from TTS providers and letting the user pick new defaults.

## Usage

```
/refresh-voices
```

Examples:
- `/refresh-voices` — Fetch all provider voices and interactively update defaults
- `/refresh-voices elevenlabs` — Only refresh ElevenLabs defaults (after adding a cloned voice)
- `/refresh-voices gemini google` — Refresh Gemini and Google defaults only

## Instructions

1. **Read the current defaults** from all three provider source files:
   - `internal/tts/gemini.go` — constants `geminiDefaultVoice1/2/3`, `DefaultVoices()` method, `geminiAvailableVoices()` function
   - `internal/tts/elevenlabs.go` — constants `elevenLabsDefaultVoice1/2/3` (with inline `// Name` comments), `DefaultVoices()` method, `elevenLabsAvailableVoices()` fallback list
   - `internal/tts/google.go` — constants `googleDefaultVoice1/2/3`, `DefaultVoices()` method, `googleAvailableVoices()` function
   - `internal/tts/express.go` and `internal/tts/vertex.go` — share the same `geminiDefaultVoice1/2/3` constants from `gemini.go`, so updating Gemini defaults also updates vertex-express and gemini-vertex defaults

2. **Fetch live voice lists** by running:
   ```
   go run ./cmd/podcaster list-voices
   ```
   This calls `tts.AvailableVoices()` for each provider. ElevenLabs fetches live from the API (requires `ELEVENLABS_API_KEY`); Gemini and Google return hardcoded lists. If the command fails (e.g., missing API key), report the error and offer to skip that provider.

3. **Present voices to the user**, grouped by provider. For each voice show:
   - Voice ID, Name, Gender, Description
   - Mark current defaults with `** (current default Voice 1/2/3)`
   - If the user passed provider names as arguments, only show those providers

4. **Ask the user** which voices to set as Voice 1, Voice 2, and Voice 3 defaults for each provider they want to update. The user can skip any provider by saying "skip". Accept voice IDs or voice names as input.

5. **Edit the source files** for each provider the user chose to update. For each provider, update ALL of these locations:

   **Gemini** (`internal/tts/gemini.go`):
   - The `geminiDefaultVoice1/2/3` constants (values are just the voice name, e.g., `"Charon"`)
   - The `DefaultVoices()` method — update both `ID` and `Name` fields in the returned `VoiceMap`
   - The `geminiAvailableVoices()` function — move the `DefaultFor: "Voice 1/2/3"` tags to the new default voices

   **ElevenLabs** (`internal/tts/elevenlabs.go`):
   - The `elevenLabsDefaultVoice1/2/3` constants (values are voice IDs like `"xuqUPASjAdyZvCNoMTEj"`) — update the inline `// Name` comments too
   - The `DefaultVoices()` method — update `ID` and `Name` fields in the returned `VoiceMap`
   - The `NewElevenLabsProvider` function — update the `Name` fields in the `VoiceMap` literal
   - The `elevenLabsAvailableVoices()` fallback list — move the `DefaultFor` tags to the new default voices (and add entries for new voices if they don't exist in the fallback)

   **Google** (`internal/tts/google.go`):
   - The `googleDefaultVoice1/2/3` constants (values are full voice IDs like `"en-US-Chirp3-HD-Charon"`)
   - The `DefaultVoices()` method — update `ID` and `Name` fields in the returned `VoiceMap`
   - The `NewGoogleProvider` function — update the `Name` fields in the `VoiceMap` literal
   - The `googleAvailableVoices()` function — move the `DefaultFor` tags to the new default voices

6. **Verify** the changes compile:
   ```
   go build ./...
   ```
   If the build fails, fix any issues before finishing.

7. **Show a summary** of what changed:
   - Old defaults vs new defaults for each updated provider
   - Files modified

## Quality Checks

- Every `DefaultFor` tag that was on an old default voice must be removed
- Every new default voice must have the correct `DefaultFor` tag added
- The `DefaultVoices()` method, the constants, and the `AvailableVoices` function must all agree on which voices are defaults
- ElevenLabs inline comments (e.g., `// Chad`) must match the actual voice name
- Build must pass after edits
