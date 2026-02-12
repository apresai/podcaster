# Roadmap (Historical Archive)

> **Note**: This roadmap was written after Sprint 4. Actual development took a different direction:
> Sprints 5-7 implemented cross-provider voice mixing, TTS model selection, and three-host support.
> Sprints 8-9 added progress bars, show formats, script refinement, and a TUI.
> Sprint 10 deployed the MCP server on AWS Bedrock AgentCore.
> The sprint ideas below were partially implemented but with different scopes and approaches than described here.

Three feature requests for podcaster, each scoped as a sprint. These build on the existing v0.1.0 architecture (ingest → script → TTS → assembly pipeline with interface-based stages).

---

## Sprint 5: Audiobook Mode

### Problem

Podcaster only produces two-host conversational audio. Users with long-form content (articles, documentation, book chapters) often want a straight narration rather than a synthesized dialogue. Converting a technical document into a two-host debate loses the original structure and adds editorial framing that isn't always wanted.

### How It Works

**New `audiobook` command** — a sibling to `generate`, not a flag. Audiobook mode has different enough semantics (single narrator, chapter structure, no dialogue conversion) that overloading `generate` with mode flags would muddy the CLI.

```
podcaster audiobook -i book.pdf -o audiobook.mp3 --voice george --chapters
```

**Pipeline reuse**: The ingest stage (`ingest.Ingester` interface) is shared — URLs, PDFs, and text files work identically. The script generation and TTS stages get new implementations.

**New `AudiobookGenerator`** implementing `script.Generator`:
- Takes the ingested text and produces a `script.Script` where every segment has `speaker: "Narrator"`
- Preserves source structure rather than converting to dialogue — the prompt instructs Claude to clean up the text for spoken delivery (expand abbreviations, smooth transitions, add verbal signposts) without rewriting the content
- Chapter detection: split on headings (markdown `#`, `##`), PDF section breaks, or `---` dividers in plain text. Each chapter becomes a group of segments with an intro marker
- SSML-style pause markers between chapters (translated to silence segments during assembly)
- No scratchpad or dialogue planning — the prompt is simpler than the podcast generator

**Voice selection**: Single `--voice` flag instead of `--voice-alex`/`--voice-sam`. Accepts a voice ID or name from the existing voice registry (`tts.AvailableVoices`). Defaults to George (the current Alex default).

**Script JSON format** — same `script.Script` struct, single speaker throughout:
```json
{
  "title": "Chapter 1: Introduction",
  "summary": "Narration of the introduction chapter",
  "segments": [
    {"speaker": "Narrator", "text": "..."},
    {"speaker": "Narrator", "text": "[PAUSE]"},
    {"speaker": "Narrator", "text": "..."}
  ]
}
```

**Assembly**: The existing FFmpeg assembler handles this unchanged — it concatenates MP3 segments regardless of speaker count. Chapter pauses are just silence segments (generated via FFmpeg `anullsrc` or a pre-made silent MP3).

### Key Decisions

1. **New command vs. flag**: A new `audiobook` cobra command keeps the CLI clean. `generate` stays focused on podcast-style output. Shared logic (ingest, TTS, assembly) lives in the existing packages.
2. **Chapter detection heuristic**: Start simple — split on `#` headings and blank-line-separated sections. PDF chapter detection is harder (no semantic structure in most PDFs); punt to a `--chapters` flag that enables best-effort splitting vs. treating the whole document as one chapter.
3. **Prompt design**: The audiobook prompt is fundamentally different from the podcast prompt — it preserves source text rather than converting it. This is a new prompt template in `internal/script/prompt.go` (or a new file `audiobook_prompt.go`), not a modification of the existing podcast prompt.
4. **`VoiceForSpeaker` update**: Currently hardcoded to "Alex"/"Sam" switch. Needs to handle "Narrator" — either add a case or generalize to a map lookup (which sets up Sprint 7).

### Scope Estimate

- ~4 new/modified files: `internal/cli/audiobook.go` (cobra command), `internal/script/audiobook.go` (generator + prompt), modify `internal/tts/tts.go` (handle "Narrator" speaker), modify `internal/pipeline/pipeline.go` (or new `audiobook_pipeline.go`)
- ~300-400 LoC new code
- No new dependencies

### Dependencies

- None — builds on existing v0.1.0 infrastructure

---

## Sprint 6: ElevenLabs Batch Synthesis

### Problem

ElevenLabs TTS currently makes one HTTP request per script segment. A standard episode (~40 segments) means 40 sequential API calls. A deep dive (~150 segments) means 150 calls. This is slow (sequential to avoid rate limits), expensive in terms of connection overhead, and produces segments that lack prosody continuity — each segment starts from a "cold" vocal state with no awareness of what came before.

### How It Works

**Two-phase approach**: Request Stitching first (quick win, big impact), then Studio Projects API (full batch).

#### Phase 1: Request Stitching

ElevenLabs supports a `previous_request_ids` field on the standard TTS endpoint. When provided, the model uses the audio context from previous requests to maintain prosody continuity — intonation, pacing, and emotional state carry across segments.

**Constraints**: Same-speaker only. You can't stitch across different voices. This means tracking request IDs per speaker independently.

**Implementation**:
- Add `previous_request_ids` field to `elevenLabsRequest` struct (max 3 IDs per the API)
- Track the `request-id` response header from each synthesis call, keyed by speaker name
- In `synthesizeSegments`, before each call, look up the last 3 request IDs for the current segment's speaker and include them
- Store request IDs in a `map[string][]string` (speaker → recent request IDs)

```go
type elevenLabsRequest struct {
    Text               string                 `json:"text"`
    ModelID            string                 `json:"model_id"`
    VoiceSettings      *elevenLabsVoiceParams `json:"voice_settings,omitempty"`
    PreviousRequestIDs []string               `json:"previous_request_ids,omitempty"`
}
```

**Impact**: ~20 lines changed in `internal/tts/elevenlabs.go`. No new files, no new endpoints, no polling. Same cost, same sequential flow, noticeably better audio continuity.

#### Phase 2: Studio Projects API

The ElevenLabs Studio Projects API supports true async batch synthesis with multi-speaker blocks. Instead of 50 individual calls, you submit one project with all segments and poll for completion.

**Workflow**:
1. **Create project** — `POST /v1/studio/projects` with project name and default settings
2. **Add content blocks** — `POST /v1/studio/projects/{id}/content` with text blocks assigned to voice IDs. Each block maps to a script segment.
3. **Start conversion** — `POST /v1/studio/projects/{id}/convert`
4. **Poll for completion** — `GET /v1/studio/projects/{id}` until status is `done`
5. **Download audio** — `GET /v1/studio/projects/{id}/audio` returns the final MP3

**Implementation**: New `ElevenLabsBatchProvider` implementing the existing `tts.BatchProvider` interface:

```go
func (p *ElevenLabsBatchProvider) SynthesizeBatch(
    ctx context.Context,
    segments []script.Segment,
    voices VoiceMap,
) (AudioResult, error)
```

The pipeline already prefers `BatchProvider` over per-segment synthesis (see `pipeline.go:149` — the `if bp, ok := provider.(tts.BatchProvider)` check). So once the batch provider exists, it plugs in automatically.

**Polling strategy**: Start at 2s interval, back off to 10s max. Timeout after 10 minutes. Show progress updates during polling.

**Fallback**: If batch synthesis fails (API error, timeout), fall back to per-segment synthesis with request stitching. Log a warning but don't fail the pipeline.

### Key Decisions

1. **Phase 1 first**: Request Stitching is a ~20 line change with immediate quality improvement. Ship it before tackling the full Projects API.
2. **No Go SDK**: ElevenLabs has no official Go SDK. Continue with raw `net/http` — consistent with the existing implementation and avoids an unmaintained third-party dependency.
3. **Provider selection**: Add `--tts elevenlabs-batch` as a distinct provider name, or auto-detect batch capability? Recommendation: keep the same `elevenlabs` provider name but add a `--batch` flag. The provider decides internally whether to use batch or per-segment.
4. **Cost**: Studio Projects API has the same per-character pricing as individual TTS calls. The benefit is latency and prosody, not cost.
5. **Cleanup**: Projects should be deleted after successful audio download to avoid clutter in the user's ElevenLabs account. Add a `DELETE /v1/studio/projects/{id}` call in the cleanup path.

### Scope Estimate

- **Phase 1 (Request Stitching)**: ~20 LoC changed in `internal/tts/elevenlabs.go`
- **Phase 2 (Studio Projects)**: ~150 LoC in a new `internal/tts/elevenlabs_batch.go`, plus ~30 LoC in provider registration
- Total: ~200 LoC across 2-3 files
- No new dependencies

### Dependencies

- Phase 1: None
- Phase 2: None (Phase 1 is independent — both can ship together or separately)

---

## Sprint 7: Three-Host Podcast

### Problem

Two hosts produces a single conversational dynamic: host drives, analyst probes. Real podcasts often have three or more participants, which enables richer formats — a moderator guiding two guests with different perspectives, a roundtable where three equals debate, or an interview panel with complementary expertise. The current architecture hardcodes two hosts at every layer: `VoiceMap` struct has `Alex`/`Sam` fields, `VoiceForSpeaker` is a two-case switch, prompt engineering assumes exactly two personas, and the CLI has `--voice-alex`/`--voice-sam` flags.

### How It Works

**Generalize from 2 hardcoded hosts to N configurable hosts.**

#### Host Persona System

Replace hardcoded Alex/Sam with a configurable host registry:

```go
type HostPersona struct {
    Name         string // "Alex", "Sam", "Jordan"
    Role         string // "host", "analyst", "moderator", "guest"
    Style        string // Speaking style description for the prompt
    DefaultVoice string // Default ElevenLabs voice ID
}
```

Built-in presets define common configurations:
- **duo** (default, current behavior): Alex (host) + Sam (analyst)
- **panel**: Alex (moderator) + Sam (guest) + Jordan (guest)
- **roundtable**: Alex + Sam + Jordan as equals
- **interview**: Alex (interviewer) + Sam (expert) + Jordan (expert)

Selected via `--format panel` or `--format roundtable`. Custom host definitions via a JSON config file for advanced users.

#### Voice Map Generalization

Replace the struct-based `VoiceMap` with a dynamic map:

```go
// Before (hardcoded)
type VoiceMap struct {
    Alex Voice
    Sam  Voice
}

// After (dynamic)
type VoiceMap map[string]Voice  // speaker name → voice
```

`VoiceForSpeaker` becomes a simple map lookup with a fallback:

```go
func VoiceForSpeaker(speaker string, voices VoiceMap) Voice {
    if v, ok := voices[speaker]; ok {
        return v
    }
    // Fallback to first voice in map
    for _, v := range voices {
        return v
    }
    return Voice{}
}
```

This is a breaking change to the `VoiceMap` type. Every reference to `voices.Alex` and `voices.Sam` needs updating: `pipeline.go` (voice override logic), `elevenlabs.go` (default voices), any other provider implementations (Google, Gemini).

#### CLI Changes

- Replace `--voice-alex` / `--voice-sam` with `--voice <name>=<id>` (repeatable flag)
  ```
  podcaster generate -i input.txt -o out.mp3 --format panel \
      --voice alex=JBFqnCBsd6RMkjVDRZzb \
      --voice sam=EXAVITQu4vr4xnSDxMaL \
      --voice jordan=onwK4e9ZLuTAKqWW03F9
  ```
- `--format` flag: `duo` (default), `panel`, `roundtable`, `interview`
- Backward compatible: `--voice-alex` and `--voice-sam` still work as shortcuts for `--voice alex=<id>` and `--voice sam=<id>`

#### Prompt Engineering

The system prompt in `internal/script/prompt.go` needs a third host persona and updated conversation dynamics rules:

- **Three-way turn-taking**: Prevent any two hosts from having an extended back-and-forth that excludes the third. Rule: no more than 4 consecutive segments between the same two speakers before the third joins.
- **Distinct roles per format**: Panel format has the moderator directing questions, roundtable has natural topic handoffs, interview has structured Q&A flow.
- **Speaking balance**: Each host gets at least 20% of segments (for 3 hosts). The prompt enforces this the same way it currently enforces 30% minimum for 2 hosts.
- **New persona**: Jordan — a third voice with a distinct speaking style. Example: "Jordan (Connector): Bridges ideas across domains. Draws unexpected parallels, shares relevant anecdotes, synthesizes what Alex and Sam are saying into broader insights."

The prompt template becomes dynamic — built from the list of host personas rather than hardcoded for Alex and Sam.

#### Segment Validation

Currently `VoiceForSpeaker` silently falls back to Alex's voice for unknown speakers. With N hosts, validation should:
- Reject scripts with speakers not in the configured host list
- Warn if any host has less than 20% of segments
- Fail if a speaker appears that doesn't have a configured voice

### Key Decisions

1. **`VoiceMap` struct → map**: This is the biggest refactor. It touches every TTS provider and the pipeline. Do it carefully with good test coverage.
2. **Maximum hosts**: Cap at 4 or 5. More than that produces chaotic scripts and the prompt engineering becomes unreliable. Three is the sweet spot for v1.
3. **Third voice default**: Need a third default ElevenLabs voice. Daniel (`onwK4e9ZLuTAKqWW03F9`) is a good candidate — British male, authoritative, distinct from George (Alex) and Sarah (Sam). Or pick a female voice for variety (Charlotte or Lily).
4. **Backward compatibility**: The `--voice-alex`/`--voice-sam` flags should continue working. Map them to the new `--voice` system internally.
5. **Prompt quality**: Three-way conversations are harder to make sound natural. Budget time for prompt iteration and testing. The risk is that generated scripts feel forced or repetitive with three speakers. Mitigate by testing with multiple content types and iterating on the system prompt.

### Scope Estimate

- ~8 modified files: `tts/provider.go` (VoiceMap type), `tts/tts.go` (VoiceForSpeaker), `tts/elevenlabs.go`, `tts/google.go`, `tts/gemini.go` (default voices), `script/prompt.go` (host personas + dynamic prompt), `pipeline/pipeline.go` (voice override logic), `cli/root.go` (new flags)
- ~2 new files: `script/hosts.go` (persona definitions + presets), `cli/format.go` (format flag parsing)
- ~400-500 LoC new/modified
- No new dependencies

### Dependencies

- Sprint 5 (Audiobook Mode) should land first — it introduces the "Narrator" speaker handling in `VoiceForSpeaker`, which this sprint generalizes. Doing Sprint 7 first would require two refactors of the same code.
- Sprint 6 (Batch Synthesis) is independent — request stitching and Studio Projects work regardless of host count.
