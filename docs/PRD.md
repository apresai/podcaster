# Podcaster — Product Requirements Document

## Problem Statement

People accumulate written content faster than they can read it. Research papers sit in "read later" folders. Bookmarked articles pile up across browsers. Read-it-later apps like Instapaper and Pocket become guilt-inducing backlogs. The content is valuable — people saved it for a reason — but reading requires dedicated screen time that competes with work, family, and rest.

Audio changes the equation. Commutes, workouts, dog walks, and household chores become consumption windows. But converting written content into listenable audio today is either manual (record yourself reading, hire a narrator) or locked behind closed platforms. Google's NotebookLM proved the concept — people love AI-generated podcast conversations — but it offers no CLI, no API, no script editing, no voice selection, and no way to integrate it into a workflow. You paste content into a web UI and accept whatever it produces.

Developers and power users need a tool that runs locally, generates editable scripts, lets them choose voices, and fits into automated pipelines. They want to turn a URL into a podcast episode with one command, review and tweak the script if they want, and control every aspect of the output.

## Target Users

**Primary: Developers and technical professionals.** They consume 10+ articles a week across blogs, docs, and HN threads but actually finish maybe three. They're comfortable with CLI tools and would pipe `podcaster` into shell scripts or cron jobs if it existed. "Done" means they listen to a generated episode during their commute and retain the key points without ever opening the article.

**Secondary: Researchers and students.** They have stacks of PDFs — papers, textbooks, reports — that they need to absorb but struggle to get through by reading alone. They highlight and re-read the same paragraphs. "Done" means they listen to a 15-minute podcast discussion of a paper and walk into a seminar actually understanding it.

**Tertiary: Content creators.** They've written blog posts, newsletters, or reports and want to offer an audio version without recording themselves. They don't want a robotic text-to-speech reading — they want a conversation that makes their content engaging in audio form. "Done" means they embed a generated episode alongside their written post.

## Product Decisions

### Why two hosts instead of one

A single voice reading content aloud is an audiobook. Two voices discussing content is a podcast. Research on educational audio shows that dialogue increases listener retention, comprehension, and engagement compared to monologue. The back-and-forth between a host who drives the narrative and an analyst who probes and questions creates natural pacing, highlights key points through repetition, and keeps the listener's attention through conversational dynamics. One host explains, the other reacts — and the listener learns from both perspectives.

### Why CLI-first

The target audience lives in the terminal. A CLI is the fastest interface to ship, the easiest to compose with other tools (`xargs`, `cron`, shell scripts), and the most natural for power users. It imposes no runtime dependencies beyond a single binary and FFmpeg. A web UI can layer on top later, but the CLI is the product — not a stepping stone to something else.

## Goals

| # | Goal | Rationale |
|---|------|-----------|
| 1 | Convert any of three input types (URL, PDF, text file) into a two-host podcast episode with a single command | Core value proposition — one command, listenable output |
| 2 | Produce audio where both hosts sound like real people having a real conversation | Quality is the difference between "useful tool" and "toy demo" |
| 3 | Complete generation in under 5 minutes for a standard (~10 min) episode | Longer than 5 minutes breaks the "fire and forget" workflow |
| 4 | Keep per-episode generation cost under $1 for standard-length content | Must be cheap enough to use daily without thinking about it |

## Non-Goals

- Web UI or hosted service (CLI serves the primary audience; web can come later)
- Real-time or streaming audio generation (batch processing is sufficient for v1)
- Video or visual content generation (audio-only scope)
- Podcast hosting or distribution (out of scope — users have their own workflows)
- User accounts or authentication (single-user CLI tool)
- Multi-language support (English only — limits complexity for v1)
- Custom voice training or cloning (ElevenLabs pre-made voices are sufficient)
- Concurrent API requests (sequential processing is simpler and respects rate limits)

## User Stories

### Core Workflow

- **As a developer** who bookmarks 20 articles a week but reads 3, I want to point the tool at a URL and get a podcast episode so I can listen during my commute instead of letting articles pile up
- **As a researcher** who has a stack of PDFs for an upcoming literature review, I want to convert a paper into a two-host discussion so I can absorb the key arguments while walking to the lab
- **As a content creator** who has a finished blog post, I want to provide the text file and get a podcast episode so I can offer an audio version alongside the written piece

### Customization

- **As a user** preparing for a meeting on a specific topic within a long report, I want to focus the conversation on that topic so the episode covers what I actually need
- **As a user** converting a casual blog post vs. a dense technical paper, I want to set the conversation tone so the output matches the source material's register
- **As a user** with a 20-minute commute, I want to set a target duration so episodes fit my listening window without cutting off mid-thought

### Advanced Workflow

- **As a user** who wants to review what the hosts will say before generating audio, I want to output only the script as JSON so I can edit the conversation and then generate audio from my edited version
- **As a user** who has hand-edited a script, I want to generate audio from that script file so I have full control over the final conversation
- **As a user** who wants to override the default voices, I want to specify ElevenLabs voice IDs for each host so I can use voices that match my preference

### Edge Cases and Errors

- **As a user** who accidentally provides a paywalled or 404 URL, I want a clear error message telling me the content couldn't be fetched so I don't waste time debugging
- **As a user** who provides an extremely short input (a tweet-length blurb), I want the tool to either generate a proportionally short episode or tell me the input is too short, rather than producing a padded, repetitive conversation
- **As a user** who provides a very long input (100-page PDF), I want the tool to handle it gracefully — either by processing it fully, summarizing, or telling me it exceeds limits
- **As a user** who specifies an output path that already exists, I want to be warned or have the behavior be predictable (overwrite) so I don't accidentally lose a previous episode
- **As a user** who hits Ctrl+C during generation, I want temp files to be cleaned up so I don't accumulate junk in my temp directory

## Functional Requirements

### P0 — MVP (Core Pipeline)

| ID | Requirement |
|----|-------------|
| FR-1 | Accept URLs as input and extract the readable article content, stripping navigation, ads, and boilerplate |
| FR-2 | Accept PDF files as input and extract text content |
| FR-3 | Accept plain text files as input |
| FR-4 | Auto-detect input type from the provided path or URI |
| FR-5 | Validate that the input exists and is readable before starting the pipeline |
| FR-6 | Reject or handle gracefully inputs that are too short to produce a meaningful conversation (e.g., under 100 words) |
| FR-7 | Handle large inputs (e.g., 100+ page PDFs) without crashing — either process fully or inform the user of limits |
| FR-10 | Generate a two-host conversational script from the extracted content |
| FR-11 | Use two distinct host personas with consistent, differentiated speaking styles |
| FR-12 | Output the script as structured JSON with speaker and text per segment |
| FR-20 | Convert each script segment to natural speech using a text-to-speech API |
| FR-21 | Use distinct voices for each host persona |
| FR-30 | Assemble all speech segments into a single audio file with natural pauses between speaker turns |
| FR-31 | Produce output at standard podcast quality (44.1kHz, 128kbps MP3) |
| FR-32 | Output the final file to the path specified by the `-o` flag |
| FR-33 | If the output file already exists, overwrite it (standard CLI behavior) |
| FR-40 | Primary command: `podcaster generate -i <source> -o <output.mp3>` |
| FR-41 | Display progress output showing current pipeline stage and, during TTS, per-segment progress |
| FR-42 | Exit with clear error messages indicating which stage failed and why |
| FR-43 | Check for required API keys at startup and exit with a clear message if any are missing |
| FR-44 | Check for required system dependencies at startup and exit with install instructions if missing |
| FR-45 | Clean up all temp files on completion, failure, or interrupt (Ctrl+C) |

### P1 — Enhancements

| ID | Requirement |
|----|-------------|
| FR-13 | Support a `--topic` flag to focus the conversation on a specific aspect of the source content |
| FR-14 | Support a `--tone` flag to adjust conversation style (e.g., casual, technical, educational) |
| FR-15 | Support a `--duration` flag to control approximate episode length (e.g., short, medium, long) |
| FR-16 | Support a `--script-only` flag to output only the script JSON, skipping audio generation |
| FR-17 | Support a `--from-script` flag to generate audio from a previously saved or edited script JSON file |
| FR-22 | Support `--voice-alex` and `--voice-sam` flags to override default voice selections |
| FR-46 | Support a `--verbose` flag for detailed logging of each pipeline stage |

## Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1 | Generation time for a standard 10-min episode | < 5 minutes end-to-end |
| NFR-2 | Cost per standard 10-min episode | < $1.00 (API costs combined) |
| NFR-3 | Audio output quality | 44.1kHz, 128kbps MP3 |
| NFR-4 | Distributed as a single binary | < 20MB |
| NFR-5 | Platform support | macOS (primary), Linux (secondary) |
| NFR-6 | API failure handling | Retry transient failures with exponential backoff, max 3 attempts |
| NFR-7 | Startup time | < 1 second to first visible output |
| NFR-8 | Security — API keys must never appear in logs, error output, progress messages, or temp files | Zero key leakage |
| NFR-9 | Privacy — user content is sent to third-party APIs (Anthropic for script generation, ElevenLabs for TTS); this is inherent to the architecture and must be documented | Documented in README |
| NFR-10 | Installability — installable via `go install` from source; single binary with no runtime dependencies beyond FFmpeg | One-step install |

## Success Metrics

| # | Metric | How to verify |
|---|--------|---------------|
| 1 | **Functional completeness** | Generate a complete episode from each input type (URL, PDF, text file) — all three produce a playable MP3 |
| 2 | **Conversation quality** | Author listens to 3 generated episodes from different source types and rates each as "listenable and coherent" — hosts discuss the actual content, not filler |
| 3 | **Generation speed** | Time 5 runs on medium-length inputs (~2000 words); all complete in under 5 minutes |
| 4 | **Pipeline reliability** | Run 10 generations on varied valid inputs; at least 9 succeed without manual intervention |
| 5 | **First-run experience** | A developer with Go and FFmpeg installed can go from `git clone` to a generated episode in under 5 minutes, following only the README |
| 6 | **Cost** | Check API billing after generating 5 standard episodes; average cost per episode is under $1 |

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| TTS API cost increases | Medium | High | Duration controls to limit output length; cost estimation before generation in future version |
| Script quality inconsistency — some inputs produce repetitive or shallow conversations | Medium | Medium | Detailed prompt engineering with persona definitions and reasoning steps; `--script-only` mode for review before committing to TTS costs |
| API rate limiting during TTS (many segments per episode) | Low | Medium | Sequential segment processing; exponential backoff on 429 responses |
| Large or complex PDF extraction failures (scanned images, encrypted, malformed) | Medium | Low | Validate extraction output; surface clear error messages with failure reason |
| Required system dependency (FFmpeg) not installed | Medium | Low | Check at startup; exit with install instructions |
| Third-party API breaking changes or deprecation | Low | High | Interface-based architecture allows swapping providers; pin to specific API versions |
| Internet connectivity required for all generation | Always | Medium | Inherent to cloud API architecture; `--script-only` and `--from-script` allow splitting workflow across connectivity windows |

## Constraints

- **Solo developer + AI agent** — scope must be achievable by one person; no features that require ongoing operational support
- **API cost budget** — no caching layer or infrastructure; cost is purely per-generation API calls, so per-episode cost must stay low
- **English only for v1** — script generation and TTS quality are only validated for English
- **macOS primary** — development and testing on macOS; Linux supported but secondary
- **FFmpeg dependency** — only external runtime dependency; acceptable because it's ubiquitous and stable

## Future Considerations

### Near-Term
- Multiple input sources per episode (combine several articles into one conversation)
- Background music and intro/outro segments
- Episode series support (consistent numbering, shared context)

### Medium-Term
- Custom persona definitions (user-defined host personalities)
- Multi-language script generation and TTS
- Web UI frontend
- Cost estimation before generation (dry-run mode)

### Long-Term
- Real-time streaming generation
- Integration with podcast hosting platforms (RSS feed generation)
- Voice cloning for personalized host voices
- Transcript generation alongside audio
