"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Progress } from "@/components/ui/progress";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Mic, ChevronDown, ChevronUp, Download, Loader2 } from "lucide-react";

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

interface PodcastStatus {
  status: string;
  progress_percent?: number;
  stage_message?: string;
  audio_url?: string;
  title?: string;
  error?: string;
  podcast_id?: string;
}

interface Voice {
  name?: string;
  voice_id?: string;
  id?: string;
}

const FORMATS = [
  { value: "conversation", label: "Conversation" },
  { value: "interview", label: "Interview" },
  { value: "deep-dive", label: "Deep Dive" },
  { value: "explainer", label: "Explainer" },
  { value: "debate", label: "Debate" },
  { value: "news", label: "News" },
  { value: "storytelling", label: "Storytelling" },
  { value: "challenger", label: "Challenger" },
];

const MODELS = [
  { value: "haiku", label: "Haiku (fast)" },
  { value: "sonnet", label: "Sonnet (quality)" },
  { value: "gemini-flash", label: "Gemini Flash" },
  { value: "gemini-pro", label: "Gemini Pro" },
  { value: "nova-lite", label: "Nova Lite (cheapest)" },
];

const TTS_PROVIDERS = [
  { value: "gemini", label: "Gemini" },
  { value: "vertex-express", label: "Vertex Express" },
  { value: "gemini-vertex", label: "Vertex AI" },
  { value: "elevenlabs", label: "ElevenLabs" },
  { value: "google", label: "Google Cloud TTS" },
  { value: "polly", label: "AWS Polly" },
];

const DURATIONS = [
  { value: "short", label: "Short (~3-4 min)" },
  { value: "standard", label: "Standard (~8-10 min)" },
  { value: "long", label: "Long (~15 min)" },
  { value: "deep", label: "Deep (~30-35 min)" },
];

const TONES = [
  { value: "casual", label: "Casual" },
  { value: "technical", label: "Technical" },
  { value: "educational", label: "Educational" },
];

const STYLES = ["humor", "wow", "serious", "debate", "storytelling"];

export function CreatePodcastForm() {
  const [form, setForm] = useState<FormState>({
    inputMode: "url",
    inputUrl: "",
    inputText: "",
    format: "conversation",
    model: "haiku",
    tts: "gemini",
    duration: "short",
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
  });

  const [showAdvanced, setShowAdvanced] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [podcastId, setPodcastId] = useState<string | null>(null);
  const [status, setStatus] = useState<PodcastStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [voices, setVoices] = useState<Voice[]>([]);

  // Load voices when TTS provider changes
  useEffect(() => {
    const controller = new AbortController();
    setVoices([]);
    setForm((f) => ({ ...f, voice1: "", voice2: "", voice3: "" }));

    fetch(`/api/voices?provider=${form.tts}`, { signal: controller.signal })
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) {
          setVoices(data);
        } else if (data.voices && Array.isArray(data.voices)) {
          setVoices(data.voices);
        }
      })
      .catch((e) => {
        if (e.name !== "AbortError") {
          console.error("Failed to load voices:", e);
        }
      });

    return () => controller.abort();
  }, [form.tts]);

  // Poll for podcast status
  useEffect(() => {
    if (!podcastId) return;

    const interval = setInterval(async () => {
      try {
        const res = await fetch(`/api/generate/${podcastId}`);
        const data = await res.json();
        setStatus(data);

        if (data.status === "completed" || data.status === "complete" || data.status === "failed") {
          clearInterval(interval);
        }
      } catch {
        // Keep polling on transient errors
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [podcastId]);

  const update = useCallback(
    <K extends keyof FormState>(key: K, value: FormState[K]) => {
      setForm((f) => ({ ...f, [key]: value }));
    },
    []
  );

  const toggleStyle = useCallback((s: string) => {
    setForm((f) => ({
      ...f,
      style: f.style.includes(s) ? f.style.filter((x) => x !== s) : [...f.style, s],
    }));
  }, []);

  const handleSubmit = async () => {
    setError(null);
    setSubmitting(true);
    setPodcastId(null);
    setStatus(null);

    try {
      const args: Record<string, unknown> = {};

      if (form.inputMode === "url") {
        if (!form.inputUrl.trim()) {
          setError("Please enter a URL");
          setSubmitting(false);
          return;
        }
        args.input_url = form.inputUrl.trim();
      } else {
        if (!form.inputText.trim()) {
          setError("Please enter some text");
          setSubmitting(false);
          return;
        }
        args.input_text = form.inputText.trim();
      }

      args.format = form.format;
      args.model = form.model;
      args.tts = form.tts;
      args.duration = form.duration;
      args.tone = form.tone;
      args.voices = form.voices;

      if (form.topic) args.topic = form.topic;
      if (form.style.length > 0) args.style = form.style;
      if (form.voice1) args.voice1 = form.voice1;
      if (form.voice2) args.voice2 = form.voice2;
      if (form.voice3 && form.voices >= 3) args.voice3 = form.voice3;
      if (form.ttsModel) args.tts_model = form.ttsModel;
      if (form.ttsSpeed) args.tts_speed = parseFloat(form.ttsSpeed);
      if (form.ttsStability) args.tts_stability = parseFloat(form.ttsStability);
      if (form.ttsPitch) args.tts_pitch = parseFloat(form.ttsPitch);

      const res = await fetch("/api/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(args),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || "Failed to start generation");
        setSubmitting(false);
        return;
      }

      const id = data.podcast_id || data.podcastId;
      if (id) {
        setPodcastId(id);
        setStatus({ status: "processing", progress_percent: 0, stage_message: "Starting..." });
      } else {
        setError("No podcast ID returned");
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setSubmitting(false);
    }
  };

  const isGenerating = podcastId && status && status.status !== "completed" && status.status !== "failed";
  const voiceId = (v: Voice) => v.voice_id || v.id || v.name || "";
  const voiceLabel = (v: Voice) => v.name || v.voice_id || v.id || "Unknown";

  return (
    <div className="space-y-6">
      {/* Section 1: Content Input */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Mic className="size-5" />
            Content
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs
            value={form.inputMode}
            onValueChange={(v) => update("inputMode", v as "url" | "text")}
          >
            <TabsList>
              <TabsTrigger value="url">URL</TabsTrigger>
              <TabsTrigger value="text">Paste Text</TabsTrigger>
            </TabsList>
            <TabsContent value="url" className="mt-4">
              <div className="space-y-2">
                <Label htmlFor="input-url">Article or page URL</Label>
                <Input
                  id="input-url"
                  placeholder="https://example.com/article"
                  value={form.inputUrl}
                  onChange={(e) => update("inputUrl", e.target.value)}
                  disabled={!!isGenerating}
                />
              </div>
            </TabsContent>
            <TabsContent value="text" className="mt-4">
              <div className="space-y-2">
                <Label htmlFor="input-text">Content to convert</Label>
                <Textarea
                  id="input-text"
                  placeholder="Paste your article, essay, or notes here..."
                  rows={8}
                  value={form.inputText}
                  onChange={(e) => update("inputText", e.target.value)}
                  disabled={!!isGenerating}
                />
              </div>
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
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <div className="space-y-2">
              <Label>Format</Label>
              <Select
                value={form.format}
                onValueChange={(v) => update("format", v)}
                disabled={!!isGenerating}
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
            </div>

            <div className="space-y-2">
              <Label>Script Model</Label>
              <Select
                value={form.model}
                onValueChange={(v) => update("model", v)}
                disabled={!!isGenerating}
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
                onValueChange={(v) => update("tts", v)}
                disabled={!!isGenerating}
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
                onValueChange={(v) => update("duration", v)}
                disabled={!!isGenerating}
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
                onValueChange={(v) => update("tone", v)}
                disabled={!!isGenerating}
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
          <button
            type="button"
            className="flex w-full items-center justify-between"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            <CardTitle>Advanced Options</CardTitle>
            {showAdvanced ? (
              <ChevronUp className="size-5 text-muted-foreground" />
            ) : (
              <ChevronDown className="size-5 text-muted-foreground" />
            )}
          </button>
        </CardHeader>
        {showAdvanced && (
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="topic">Focus Topic</Label>
              <Input
                id="topic"
                placeholder="e.g. key findings, business implications"
                value={form.topic}
                onChange={(e) => update("topic", e.target.value)}
                disabled={!!isGenerating}
              />
            </div>

            <div className="space-y-2">
              <Label>Style</Label>
              <div className="flex flex-wrap gap-2">
                {STYLES.map((s) => (
                  <Badge
                    key={s}
                    variant={form.style.includes(s) ? "default" : "outline"}
                    className="cursor-pointer select-none"
                    onClick={() => !isGenerating && toggleStyle(s)}
                  >
                    {s}
                  </Badge>
                ))}
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>Number of Voices</Label>
                <Select
                  value={String(form.voices)}
                  onValueChange={(v) => update("voices", parseInt(v))}
                  disabled={!!isGenerating}
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
            </div>

            {voices.length > 0 && (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <div className="space-y-2">
                  <Label>Voice 1</Label>
                  <Select
                    value={form.voice1}
                    onValueChange={(v) => update("voice1", v)}
                    disabled={!!isGenerating}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Default" />
                    </SelectTrigger>
                    <SelectContent>
                      {voices.map((v) => (
                        <SelectItem key={voiceId(v)} value={voiceId(v)}>
                          {voiceLabel(v)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {form.voices >= 2 && (
                  <div className="space-y-2">
                    <Label>Voice 2</Label>
                    <Select
                      value={form.voice2}
                      onValueChange={(v) => update("voice2", v)}
                      disabled={!!isGenerating}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Default" />
                      </SelectTrigger>
                      <SelectContent>
                        {voices.map((v) => (
                          <SelectItem key={voiceId(v)} value={voiceId(v)}>
                            {voiceLabel(v)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
                {form.voices >= 3 && (
                  <div className="space-y-2">
                    <Label>Voice 3</Label>
                    <Select
                      value={form.voice3}
                      onValueChange={(v) => update("voice3", v)}
                      disabled={!!isGenerating}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Default" />
                      </SelectTrigger>
                      <SelectContent>
                        {voices.map((v) => (
                          <SelectItem key={voiceId(v)} value={voiceId(v)}>
                            {voiceLabel(v)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="tts-model">TTS Model Override</Label>
              <Input
                id="tts-model"
                placeholder="Optional model name override"
                value={form.ttsModel}
                onChange={(e) => update("ttsModel", e.target.value)}
                disabled={!!isGenerating}
              />
            </div>

            {(form.tts === "elevenlabs" || form.tts === "google") && (
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="tts-speed">
                    Speed{" "}
                    <span className="text-muted-foreground text-xs">
                      ({form.tts === "elevenlabs" ? "0.7–1.2" : "0.25–2.0"})
                    </span>
                  </Label>
                  <Input
                    id="tts-speed"
                    type="number"
                    step="0.1"
                    placeholder={form.tts === "elevenlabs" ? "1.0" : "1.0"}
                    value={form.ttsSpeed}
                    onChange={(e) => update("ttsSpeed", e.target.value)}
                    disabled={!!isGenerating}
                  />
                </div>

                {form.tts === "elevenlabs" && (
                  <div className="space-y-2">
                    <Label htmlFor="tts-stability">
                      Stability{" "}
                      <span className="text-muted-foreground text-xs">(0–1.0)</span>
                    </Label>
                    <Input
                      id="tts-stability"
                      type="number"
                      step="0.1"
                      placeholder="0.5"
                      value={form.ttsStability}
                      onChange={(e) => update("ttsStability", e.target.value)}
                      disabled={!!isGenerating}
                    />
                  </div>
                )}

                {form.tts === "google" && (
                  <div className="space-y-2">
                    <Label htmlFor="tts-pitch">
                      Pitch{" "}
                      <span className="text-muted-foreground text-xs">(-20 to 20)</span>
                    </Label>
                    <Input
                      id="tts-pitch"
                      type="number"
                      step="1"
                      placeholder="0"
                      value={form.ttsPitch}
                      onChange={(e) => update("ttsPitch", e.target.value)}
                      disabled={!!isGenerating}
                    />
                  </div>
                )}
              </div>
            )}
          </CardContent>
        )}
      </Card>

      {/* Generate Button */}
      {!podcastId && (
        <Button
          size="lg"
          className="w-full"
          onClick={handleSubmit}
          disabled={submitting}
        >
          {submitting ? (
            <>
              <Loader2 className="size-4 animate-spin mr-2" />
              Starting...
            </>
          ) : (
            <>
              <Mic className="size-4 mr-2" />
              Generate Podcast
            </>
          )}
        </Button>
      )}

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
            <CardTitle className="flex items-center justify-between">
              <span>
                {status.status === "completed" || status.status === "complete"
                  ? status.title || "Podcast Ready"
                  : status.status === "failed"
                    ? "Generation Failed"
                    : "Generating..."}
              </span>
              <Badge
                variant={
                  status.status === "completed" || status.status === "complete"
                    ? "default"
                    : status.status === "failed"
                      ? "destructive"
                      : "secondary"
                }
              >
                {status.status}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {status.status !== "completed" && status.status !== "failed" && (
              <>
                <Progress value={Math.round((status.progress_percent || 0) * 100)} />
                <p className="text-sm text-muted-foreground">
                  {status.stage_message || "Processing..."}
                </p>
              </>
            )}

            {(status.status === "completed" || status.status === "complete") && status.audio_url && (
              <div className="space-y-4">
                <audio controls className="w-full" src={status.audio_url} />
                <a
                  href={status.audio_url}
                  download
                  className="inline-flex items-center gap-2 text-sm text-primary hover:underline"
                >
                  <Download className="size-4" />
                  Download MP3
                </a>
              </div>
            )}

            {status.status === "failed" && (
              <Alert variant="destructive">
                <AlertDescription>
                  {status.error || "An unknown error occurred during generation."}
                </AlertDescription>
              </Alert>
            )}

            {(status.status === "completed" || status.status === "complete" || status.status === "failed") && (
              <Button
                variant="outline"
                onClick={() => {
                  setPodcastId(null);
                  setStatus(null);
                  setError(null);
                }}
              >
                Create Another
              </Button>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
