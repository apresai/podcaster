import Link from "next/link";
import Image from "next/image";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { WaveformLogo } from "@/components/waveform-logo";
import { UserPlus, Key, Headphones, ListMusic } from "lucide-react";

const features = [
  {
    title: "AI-powered scripts",
    desc: "Claude or Gemini generates natural, engaging two-host conversations from your content.",
    image: "/feat-ai-script.webp",
  },
  {
    title: "Multiple voices",
    desc: "Choose from 30+ AI voices across Gemini, ElevenLabs, and Google Cloud TTS.",
    image: "/feat-voices.webp",
  },
  {
    title: "8 show formats",
    desc: "Conversation, interview, deep-dive, explainer, debate, news, storytelling, and challenger.",
    image: "/feat-formats.webp",
  },
  {
    title: "Any content source",
    desc: "URLs, PDFs, plain text files, or clipboard content — Podcaster handles them all.",
    image: "/feat-content.webp",
  },
  {
    title: "MCP integration",
    desc: "Works with Claude Desktop and any MCP-compatible AI assistant out of the box.",
    image: "/feat-mcp.webp",
  },
  {
    title: "CDN delivery",
    desc: "Generated podcasts are hosted on CloudFront for fast, reliable playback worldwide.",
    image: "/feat-cdn.webp",
  },
];

const steps = [
  {
    num: "01",
    title: "Sign up",
    desc: "Create an account and get approved. You will receive an API key to authenticate your requests.",
    icon: UserPlus,
  },
  {
    num: "02",
    title: "Get an API key",
    desc: "Generate an API key from your dashboard. Use it with Claude Desktop or any MCP-compatible client.",
    icon: Key,
  },
  {
    num: "03",
    title: "Generate podcasts",
    desc: "Ask your AI assistant to generate a podcast from any URL or text. It handles everything automatically.",
    icon: Headphones,
  },
];

const stats = [
  { value: "30+", label: "AI voices" },
  { value: "8", label: "Show formats" },
  { value: "~4 min", label: "Generation time" },
];

export default function Home() {
  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="fixed top-0 z-50 w-full border-b border-border/40 bg-card/80 backdrop-blur-md">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
          <div className="flex items-center gap-2">
            <WaveformLogo size={22} />
            <span className="font-semibold text-lg gradient-text">
              Podcaster
            </span>
          </div>
          <div className="flex items-center gap-3">
            <Link href="/podcasts">
              <Button size="sm" variant="ghost" className="text-white/80 hover:text-white hover:bg-white/10">
                <ListMusic className="size-4 mr-1.5" />
                Browse
              </Button>
            </Link>
            <Link href="/login">
              <Button size="sm">Sign in</Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="relative overflow-hidden chalk-bg pt-14">
        {/* Background image */}
        <Image
          src="/hero-bg.webp"
          alt=""
          fill
          className="object-cover opacity-20"
          priority
        />
        {/* Gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-b from-[var(--chalk)] via-transparent to-[var(--chalk)]" />

        <div className="relative mx-auto max-w-6xl px-4 py-28 md:py-36 text-center">
          {/* Badge */}
          <div
            className="animate-fade-in inline-flex items-center gap-2 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-sm text-primary mb-6"
            style={{ animationDelay: "0.1s", animationFillMode: "both" }}
          >
            <WaveformLogo size={14} />
            Powered by AI
          </div>

          <h1
            className="animate-fade-in text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight text-white"
            style={{ animationDelay: "0.2s", animationFillMode: "both" }}
          >
            Turn any content into
            <br />
            <span className="gradient-text">a podcast</span>
          </h1>

          <p
            className="animate-fade-in mt-5 text-lg md:text-xl text-white/60 max-w-2xl mx-auto"
            style={{ animationDelay: "0.35s", animationFillMode: "both" }}
          >
            Paste a URL, upload a document, or provide text — Podcaster
            generates a natural two-host conversation with AI voices.
          </p>

          <div
            className="animate-fade-in mt-8 flex flex-col sm:flex-row items-center justify-center gap-3"
            style={{ animationDelay: "0.5s", animationFillMode: "both" }}
          >
            <Link href="/login">
              <Button size="lg" className="px-8">
                Get started
              </Button>
            </Link>
            <Link href="/podcasts">
              <Button size="lg" className="px-8">
                Browse podcasts
              </Button>
            </Link>
          </div>

          {/* Decorative waveform bars */}
          <div className="mt-16 flex items-end justify-center gap-1 opacity-20">
            {[40, 65, 90, 55, 80, 45, 70, 95, 50, 75, 60, 85].map((h, i) => (
              <div
                key={i}
                className="w-1 rounded-full bg-primary"
                style={{
                  height: `${h * 0.6}px`,
                  animation: "waveform 1.2s ease-in-out infinite",
                  animationDelay: `${i * 0.1}s`,
                  transformOrigin: "bottom",
                }}
              />
            ))}
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="py-20 md:py-28">
        <div className="mx-auto max-w-6xl px-4">
          <h2 className="text-center text-3xl font-bold">How it works</h2>
          <p className="mt-3 text-center text-muted-foreground">
            Three simple steps to your first podcast
          </p>

          <div className="mt-14 grid gap-8 md:grid-cols-3">
            {steps.map((step) => (
              <Card
                key={step.num}
                className="border-t-4 border-t-primary overflow-hidden"
              >
                <CardContent className="pt-6">
                  <div className="flex items-center gap-3 mb-4">
                    <div className="flex items-center justify-center size-10 rounded-lg bg-primary/10 text-primary">
                      <step.icon className="size-5" />
                    </div>
                    <span className="font-mono text-sm text-muted-foreground">
                      {step.num}
                    </span>
                  </div>
                  <h3 className="font-semibold text-lg">{step.title}</h3>
                  <p className="mt-2 text-sm text-muted-foreground">
                    {step.desc}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-20 md:py-28 bg-muted/30">
        <div className="mx-auto max-w-6xl px-4">
          <h2 className="text-center text-3xl font-bold">Features</h2>
          <p className="mt-3 text-center text-muted-foreground">
            Everything you need to create professional podcasts
          </p>

          <div className="mt-14 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <Card
                key={feature.title}
                className="group overflow-hidden transition-all duration-300 hover:-translate-y-1 hover:shadow-lg hover:ring-1 hover:ring-primary/20"
              >
                <div className="relative h-40 bg-[var(--chalk)] flex items-center justify-center overflow-hidden">
                  <Image
                    src={feature.image}
                    alt={feature.title}
                    width={128}
                    height={128}
                    className="object-contain transition-transform duration-300 group-hover:scale-110"
                  />
                </div>
                <CardContent className="pt-4">
                  <h3 className="font-semibold">{feature.title}</h3>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {feature.desc}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Stats banner */}
      <section className="chalk-bg py-16">
        <div className="mx-auto max-w-6xl px-4">
          <div className="grid gap-8 md:grid-cols-3 text-center">
            {stats.map((stat) => (
              <div key={stat.label}>
                <div className="text-4xl font-bold text-primary">
                  {stat.value}
                </div>
                <div className="mt-1 text-white/60 text-sm">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t py-8">
        <div className="mx-auto max-w-6xl px-4 flex items-center justify-center gap-2 text-sm text-muted-foreground">
          <WaveformLogo size={16} />
          <span>
            <span className="text-primary font-medium">Podcaster</span> — Built
            by Apres AI
          </span>
        </div>
      </footer>
    </div>
  );
}
