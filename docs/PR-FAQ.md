# Podcaster: Turn Any Content into Engaging Audio Conversations

## Press Release

**FOR IMMEDIATE RELEASE**

### Podcaster Lets Anyone Transform Articles, PDFs, and Text into Natural-Sounding Podcast Conversations with a Single Command

*New open-source CLI tool uses AI to generate two-host podcast episodes from any written content*

**Austin, TX** — Today, an independent developer released **Podcaster**, a free command-line tool that converts written content — articles, PDFs, blog posts, research papers, or plain text — into natural-sounding podcast-style audio conversations. With a single command, users can transform dense or dry source material into an engaging dialogue between two AI hosts who explain, debate, and explore the topic in a conversational format.

Podcaster addresses a growing gap: people have more content to consume than time to read it. Audio is the preferred medium for learning on the go, but producing quality podcast content has traditionally required recording studios, editing software, and hours of production time. Podcaster eliminates that entirely.

"I had a backlog of 50+ research papers I needed to get through," said Jordan, an early tester. "I pointed Podcaster at the PDFs and listened to the conversations during my commute. It's like having two smart friends explain the paper to you — they catch nuances I would have missed skimming."

**How it works:**

1. Point Podcaster at a URL, PDF, or text file
2. AI analyzes the content and writes a two-host conversational script
3. The script is converted to speech using studio-quality text-to-speech
4. A polished MP3 is assembled and ready to play

The tool runs entirely from the command line:

```
podcaster generate -i https://example.com/article -o episode.mp3
```

Users can customize the output with flags for topic focus, conversation tone, target duration, and voice selection. A `--script-only` mode lets users review or edit the generated script before producing audio.

Podcaster is built in Go for fast execution and easy installation. It uses Claude for script generation and ElevenLabs for text-to-speech, requiring API keys for both services.

The project is available now on GitHub.

---

## Frequently Asked Questions

### External FAQ (Customer / User Questions)

**Q: What types of content can Podcaster process?**
A: Podcaster accepts three input types: URLs (web articles and blog posts), PDF files (research papers, reports, ebooks), and plain text files. It extracts the readable content, ignoring navigation, ads, and formatting artifacts, then generates a conversation based on the substance.

**Q: How long does it take to generate an episode?**
A: A typical 10-minute episode takes 2–4 minutes to generate, depending on source length. The majority of time is spent on TTS synthesis. Script generation usually completes in under 30 seconds.

**Q: How much does it cost per episode?**
A: Costs depend on episode length and source complexity. A rough estimate for a 10-minute episode: ~$0.05–0.15 for Claude API usage (script generation) and ~$0.30–0.80 for ElevenLabs TTS. Total cost is typically under $1 per episode.

**Q: Can I edit the script before generating audio?**
A: Yes. Use `--script-only` to output just the conversation script as JSON. Edit it however you like, then pass the edited script back to generate audio. This gives full control over what the hosts say.

**Q: What do the AI hosts sound like?**
A: Podcaster uses ElevenLabs' multilingual v2 model, which produces natural, expressive speech. The two hosts (Alex and Sam) have distinct voices and conversational styles. Alex tends to drive the discussion and introduce topics, while Sam asks clarifying questions and adds depth. You can override voice selections with custom ElevenLabs voice IDs.

**Q: Does it work with content in languages other than English?**
A: The TTS model supports multiple languages, but script generation is optimized for English. Non-English source content may work but quality will vary. This is a future improvement area.

**Q: Can I use this for commercial podcasts?**
A: The tool itself is open source. However, you should review the terms of service for both Claude API and ElevenLabs regarding commercial use of generated content. The audio output uses ElevenLabs voices, which have their own licensing terms.

**Q: What audio format does it output?**
A: MP3 at 44.1kHz, 128kbps — standard podcast quality. The output is a single file ready for any podcast player or distribution platform.

---

### Internal FAQ (Engineering / Business Questions)

**Q: Why a CLI tool instead of a web app?**
A: A CLI keeps the scope tight and ships faster. There's no auth, no hosting, no frontend to maintain. Power users — the target audience — are comfortable with the terminal. A web frontend or API wrapper can be layered on top later if demand warrants it.

**Q: Why Go instead of Python or Node?**
A: Go compiles to a single binary with no runtime dependencies, making distribution trivial. It handles concurrent API calls naturally with goroutines. It's also the preferred language for CLI tools in this development environment, aligning with existing infrastructure and Lambda deployment patterns.

**Q: Why Claude for script generation instead of GPT-4 or open-source models?**
A: Claude excels at long-form, nuanced content generation and follows complex persona instructions reliably. The structured output (JSON) support and large context window (200K tokens) mean entire research papers can be processed in a single pass. The `anthropic-sdk-go` provides a native Go SDK.

**Q: Why ElevenLabs instead of AWS Polly or Google TTS?**
A: ElevenLabs produces significantly more natural and expressive speech, especially for conversational content. The quality gap is immediately noticeable. Polly and Google TTS sound robotic in comparison for long-form dialogue. The cost difference is justified by the output quality.

**Q: What happens if the Claude API or ElevenLabs is down?**
A: The pipeline is designed with retry logic and exponential backoff for transient failures. If a service is fully unavailable, the tool exits with a clear error message indicating which service failed and at which stage. The `--script-only` workflow means users can still generate scripts even if TTS is down.

**Q: How do you handle very long source documents?**
A: PDFs and articles are chunked if they exceed context limits. The script generation prompt instructs Claude to focus on the most important themes rather than trying to cover everything. Users can also use the `--topic` flag to focus the conversation on a specific aspect of the source material.

**Q: What are the main technical risks?**
A: (1) TTS cost scaling — long episodes get expensive. Mitigated by duration controls and cost estimation. (2) Script quality variance — AI-generated dialogue can sometimes feel formulaic. Mitigated by detailed persona prompts and scratchpad reasoning. (3) API rate limits — ElevenLabs has per-minute limits on concurrent requests. Mitigated by sequential TTS processing with backoff.

**Q: What's the future roadmap?**
A: Near-term: multiple input sources per episode, background music/intro/outro, episode series support. Medium-term: custom persona definitions, multi-language support, web UI. Long-term: real-time streaming generation, integration with podcast hosting platforms.

**Q: Why not use NotebookLM?**
A: Google's NotebookLM generates podcast-style audio but is a closed platform with no API, no CLI access, no customization of hosts or tone, and no ability to edit scripts before generation. Podcaster gives developers full control over every stage of the pipeline and runs entirely on infrastructure you control.
