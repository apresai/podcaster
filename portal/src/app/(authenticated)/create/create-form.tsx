"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Progress } from "@/components/ui/progress";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// --- Option definitions ---

const FORMATS = [
  { value: "conversation", label: "Conversation", desc: "Two hosts chat naturally" },
  { value: "interview", label: "Interview", desc: "Host interviews a guest expert" },
  { value: "deep-dive", label: "Deep Dive", desc: "Thorough exploration of a topic" },
  { value: "explainer", label: "Explainer", desc: "Break down complex concepts" },
  { value: "debate", label: "Debate", desc: "Two hosts argue opposing views" },
  { value: "news", label: "News", desc: "News broadcast style" },
  { value: "storytelling", label: "Storytelling", desc: "Narrative-driven discussion" },
  { value: "challenger", label: "Challenger", desc: "One host challenges the other" },
];

const MODELS = [
  { value: "haiku", label: "Haiku 4.5" },
  { value: "sonnet", label: "Sonnet 4.5" },
  { value: "gemini-flash", label: "Gemini Flash" },
  { value: "gemini-pro", label: "Gemini Pro" },
];

const TTS_PROVIDERS = [
  { value: "gemini", label: "Gemini (AI Studio)" },
  { value: "vertex-express", label: "Vertex Express" },
  { value: "gemini-vertex", label: "Vertex AI" },
  { value: "elevenlabs", label: "ElevenLabs" },
  { value: "google", label: "Google Cloud TTS" },
];

const DURATIONS = [
  { value: "short", label: "Short (~8 min)" },
  { value: "standard", label: "Standard (~18 min)" },
  { value: "long", label: "Long (~35 min)" },
  { value: "deep", label: "Deep (~55 min)" },
];

const TONES = [
  { value: "casual", label: "Casual" },
  { value: "technical", label: "Technical" },
  { value: "educational", label: "Educational" },
];

const STYLES = ["humor", "wow", "serious", "debate", "storytelling"];

// --- Types ---

interface FormState {
  inputMode: "url" | "text";
  inputUrl: string;
  inputText: string;
  format: string;
  model: string;
  tts: string;
  duration: string;
  tone: string;
  voices: number;
  topic: string;
  style: string[];
  voice1: string;
  voice2: string;
  voice3: string;
  ttsModel: string;
  ttsSpeed: string;
  ttsStability: string;
  ttsPitch: string;
}

interface Voice {
  id: string;
  name: string;
  gender?: string;
  description?: string;
}

interface PodcastStatus {
  podcast_id?: string;
  status?: string;
  progress_percent?: number;
  stage_message?: string;
  title?: string;
  summary?: string;
  audio_url?: string;
  script_url?: string;
  error?: string;
}

const defaultForm: FormState = {
  inputMode: "url",
  inputUrl: "",
  inputText: "",
  format: "conversation",
  model: "haiku",
  tts: "gemini",
  duration: "standard",
  tone: "casual",
  voices: 2,
  topic: "",
  style: [],
  voice1: "",
  voice2: "",
  voice3: "",
  ttsModel: "",
  ttsSpeed: "",
  ttsStability: "",
  ttsPitch: "",
};

export function CreatePodcastForm() {
  const [form, setForm] = useState<FormState>(defaultForm);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [podcastId, setPodcastId] = useState<string | null>(null);
  const [status, setStatus] = useState<PodcastStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [voices, setVoices] = useState<Voice[]>([]);
  const [loadingVoices, setLoadingVoices] = useState(false);

  const update = useCallback(
    (partial: Partial<FormState>) =>
      setForm((prev) => ({ ...prev, ...partial })),
    [],
  );

  // Fetch voices when TTS provider changes
  useEffect(() => {
    const controller = new AbortController();
    setLoadingVoices(true);
    setVoices([]);
    update({ voice1: "", voice2: "", voice3: "" });

    fetch(`/api/voices?provider=${form.tts}`, { signal: controller.signal })
      .then((res) => res.json())
      .then((data) => {
        if (Array.isArray(data?.voices)) setVoices(data.voices);
        else if (Array.isArray(data)) setVoices(data);
      })
      .catch(() => {})
      .finally(() => setLoadingVoices(false));

    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.tts]);

  // Poll status
  useEffect(() => {
    if (!podcastId) return;

    let active = true;
    const poll = async () => {
      try {
        const res = await fetch(`/api/generate/${podcastId}`);
        const data = await res.json();
        if (!active) return;
        setStatus(data);
        if (data.status === "completed" || data.status === "failed") {
          setGenerating(false);
        }
      } catch {
        // keep polling on network errors
      }
    };

    poll();
    const interval = setInterval(poll, 3000);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [podcastId]);

  async function handleGenerate() {
    setGenerating(true);
    setError(null);
    setPodcastId(null);
    setStatus(null);

    const body: Record<string, unknown> = {};
    if (form.inputMode === "url") body.input_url = form.inputUrl;
    else body.input_text = form.inputText;

    if (form.model !== "haiku") body.model = form.model;
    if (form.tts !== "gemini") body.tts = form.tts;
    if (form.format !== "conversation") body.format = form.format;
    if (form.duration !== "standard") body.duration = form.duration;
    if (form.tone !== "casual") body.tone = form.tone;
    if (form.voices !== 2) body.voices = form.voices;
    if (form.topic) body.topic = form.topic;
    if (form.style.length > 0) body.style = form.style.join(",");
    if (form.voice1) body.voice1 = form.voice1;
    if (form.voice2) body.voice2 = form.voice2;
    if (form.voice3) body.voice3 = form.voice3;
    if (form.ttsModel) body.tts_model = form.ttsModel;
    if (form.ttsSpeed) body.tts_speed = parseFloat(form.ttsSpeed);
    if (form.ttsStability) body.tts_stability = parseFloat(form.ttsStability);
    if (form.ttsPitch) body.tts_pitch = parseFloat(form.ttsPitch);

    try {
      const res = await fetch("/api/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Generation failed");
      setPodcastId(data.podcast_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Generation failed");
      setGenerating(false);
    }
  }

  const hasInput =
    (form.inputMode === "url" && form.inputUrl.trim()) ||
    (form.inputMode === "text" && form.inputText.trim());

  const showSpeed = form.tts === "elevenlabs" || form.tts === "google";
  const showStability = form.tts === "elevenlabs";
  const showPitch = form.tts === "google";

  return (
    <div className="space-y-6">
      {/* Section 1: Content Input */}
      <Card>
        <CardHeader>
          <CardTitle>Content</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs
            value={form.inputMode}
            onValueChange={(v) => update({ inputMode: v as "url" | "text" })}
          >
            <TabsList>
              <TabsTrigger value="url">URL</TabsTrigger>
              <TabsTrigger value="text">Paste Text</TabsTrigger>
            </TabsList>
            <TabsContent value="url" className="mt-4">
              <Input
                placeholder="https://example.com/article"
                value={form.inputUrl}
                onChange={(e) => update({ inputUrl: e.target.value })}
              />
            </TabsContent>
            <TabsContent value="text" className="mt-4">
              <Textarea
                placeholder="Paste your article, blog post, or other content here..."
                rows={8}
                value={form.inputText}
                onChange={(e) => update({ inputText: e.target.value })}
              />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* Section 2: Basic Options */}
      <Card>
        <CardHeader>
          <CardTitle>Options</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-3 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Format</Label>
              <Select
                value={form.format}
                onValueChange={(v) => update({ format: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FORMATS.map((f) => (
                    <SelectItem key={f.value} value={f.value}>
                      {f.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {FORMATS.find((f) => f.value === form.format)?.desc}
              </p>
            </div>

            <div className="space-y-2">
              <Label>Script Model</Label>
              <Select
                value={form.model}
                onValueChange={(v) => update({ model: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MODELS.map((m) => (
                    <SelectItem key={m.value} value={m.value}>
                      {m.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>TTS Provider</Label>
              <Select
                value={form.tts}
                onValueChange={(v) => update({ tts: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TTS_PROVIDERS.map((t) => (
                    <SelectItem key={t.value} value={t.value}>
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Duration</Label>
              <Select
                value={form.duration}
                onValueChange={(v) => update({ duration: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DURATIONS.map((d) => (
                    <SelectItem key={d.value} value={d.value}>
                      {d.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Tone</Label>
              <Select
                value={form.tone}
                onValueChange={(v) => update({ tone: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TONES.map((t) => (
                    <SelectItem key={t.value} value={t.value}>
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Section 3: Advanced Options */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Advanced Options</CardTitle>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? "Hide" : "Show"}
            </Button>
          </div>
        </CardHeader>
        {showAdvanced && (
          <CardContent className="space-y-6">
            {/* Topic */}
            <div className="space-y-2">
              <Label>Topic Focus</Label>
              <Input
                placeholder="e.g. key findings, security implications"
                value={form.topic}
                onChange={(e) => update({ topic: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Guide the discussion toward specific aspects
              </p>
            </div>

            {/* Style toggles */}
            <div className="space-y-2">
              <Label>Style</Label>
              <div className="flex flex-wrap gap-2">
                {STYLES.map((s) => (
                  <Button
                    key={s}
                    type="button"
                    variant={form.style.includes(s) ? "default" : "outline"}
                    size="sm"
                    onClick={() =>
                      update({
                        style: form.style.includes(s)
                          ? form.style.filter((x) => x !== s)
                          : [...form.style, s],
                      })
                    }
                  >
                    {s}
                  </Button>
                ))}
              </div>
            </div>

            {/* Voices count */}
            <div className="grid gap-4 md:grid-cols-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>Number of Voices</Label>
                <Select
                  value={String(form.voices)}
                  onValueChange={(v) => update({ voices: parseInt(v) })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">1 voice</SelectItem>
                    <SelectItem value="2">2 voices</SelectItem>
                    <SelectItem value="3">3 voices</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {/* Voice pickers */}
              {Array.from({ length: form.voices }, (_, i) => {
                const key = `voice${i + 1}` as "voice1" | "voice2" | "voice3";
                return (
                  <div key={key} className="space-y-2">
                    <Label>Voice {i + 1}</Label>
                    <Select
                      value={form[key] || "auto"}
                      onValueChange={(v) =>
                        update({ [key]: v === "auto" ? "" : v })
                      }
                      disabled={loadingVoices}
                    >
                      <SelectTrigger>
                        <SelectValue
                          placeholder={
                            loadingVoices ? "Loading..." : "Auto"
                          }
                        />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="auto">Auto</SelectItem>
                        {voices.map((v) => (
                          <SelectItem key={v.id} value={v.id}>
                            {v.name}
                            {v.gender ? ` (${v.gender})` : ""}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                );
              })}
            </div>

            {/* TTS tuning */}
            {(showSpeed || showStability || showPitch) && (
              <div className="grid gap-4 md:grid-cols-3 sm:grid-cols-2">
                {showSpeed && (
                  <div className="space-y-2">
                    <Label>
                      Speed{" "}
                      <span className="text-muted-foreground font-normal">
                        ({form.tts === "elevenlabs" ? "0.7–1.2" : "0.25–2.0"})
                      </span>
                    </Label>
                    <Input
                      type="number"
                      step="0.1"
                      min={form.tts === "elevenlabs" ? 0.7 : 0.25}
                      max={form.tts === "elevenlabs" ? 1.2 : 2.0}
                      placeholder="Default"
                      value={form.ttsSpeed}
                      onChange={(e) => update({ ttsSpeed: e.target.value })}
                    />
                  </div>
                )}
                {showStability && (
                  <div className="space-y-2">
                    <Label>
                      Stability{" "}
                      <span className="text-muted-foreground font-normal">
                        (0.0–1.0)
                      </span>
                    </Label>
                    <Input
                      type="number"
                      step="0.1"
                      min={0}
                      max={1}
                      placeholder="Default"
                      value={form.ttsStability}
                      onChange={(e) =>
                        update({ ttsStability: e.target.value })
                      }
                    />
                  </div>
                )}
                {showPitch && (
                  <div className="space-y-2">
                    <Label>
                      Pitch{" "}
                      <span className="text-muted-foreground font-normal">
                        (-20 to 20)
                      </span>
                    </Label>
                    <Input
                      type="number"
                      step="1"
                      min={-20}
                      max={20}
                      placeholder="Default"
                      value={form.ttsPitch}
                      onChange={(e) => update({ ttsPitch: e.target.value })}
                    />
                  </div>
                )}
              </div>
            )}

            {/* TTS model override */}
            <div className="space-y-2">
              <Label>TTS Model Override</Label>
              <Input
                placeholder="Leave empty for default"
                value={form.ttsModel}
                onChange={(e) => update({ ttsModel: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Override the TTS model ID (provider-specific)
              </p>
            </div>
          </CardContent>
        )}
      </Card>

      {/* Generate button */}
      <Button
        size="lg"
        className="w-full"
        onClick={handleGenerate}
        disabled={generating || !hasInput}
      >
        {generating ? "Generating..." : "Generate Podcast"}
      </Button>

      {/* Error */}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Progress */}
      {status && (
        <Card>
          <CardHeader>
            <CardTitle>
              {status.status === "completed"
                ? "Podcast Ready"
                : status.status === "failed"
                  ? "Generation Failed"
                  : "Generating..."}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {status.status !== "completed" && status.status !== "failed" && (
              <div>
                <div className="flex justify-between text-sm text-muted-foreground mb-2">
                  <span>{status.stage_message || "Starting..."}</span>
                  <span>{status.progress_percent ?? 0}%</span>
                </div>
                <Progress value={status.progress_percent ?? 0} />
              </div>
            )}

            {status.title && (
              <p className="font-medium">{status.title}</p>
            )}

            {status.status === "completed" && status.audio_url && (
              <div className="space-y-3">
                <audio
                  controls
                  src={status.audio_url}
                  className="w-full"
                />
                <div className="flex gap-2">
                  <Button asChild variant="outline">
                    <a href={status.audio_url} download>
                      Download MP3
                    </a>
                  </Button>
                  {status.script_url && (
                    <Button asChild variant="ghost">
                      <a
                        href={status.script_url}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        View Script
                      </a>
                    </Button>
                  )}
                </div>
              </div>
            )}

            {status.status === "failed" && (
              <Alert variant="destructive">
                <AlertDescription>
                  {status.error || "Generation failed. Please try again."}
                </AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
