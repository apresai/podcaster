import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export default function Home() {
  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="border-b">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
          <span className="font-semibold text-lg">Podcaster</span>
          <Link href="/login">
            <Button variant="outline" size="sm">
              Sign in
            </Button>
          </Link>
        </div>
      </header>

      {/* Hero */}
      <section className="mx-auto max-w-6xl px-4 py-24 text-center">
        <h1 className="text-5xl font-bold tracking-tight">
          Turn any content into a podcast
        </h1>
        <p className="mt-4 text-xl text-muted-foreground max-w-2xl mx-auto">
          Paste a URL, upload a document, or provide text — Podcaster generates
          a natural two-host conversation with AI voices.
        </p>
        <div className="mt-8">
          <Link href="/login">
            <Button size="lg">Get started</Button>
          </Link>
        </div>
      </section>

      {/* How it works */}
      <section className="border-t bg-muted/30 py-20">
        <div className="mx-auto max-w-6xl px-4">
          <h2 className="text-center text-3xl font-bold">How it works</h2>
          <div className="mt-12 grid gap-8 md:grid-cols-3">
            <Card>
              <CardContent className="pt-6">
                <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold">
                  1
                </div>
                <h3 className="font-semibold text-lg">Sign up</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  Create an account and get approved. You will receive an API
                  key to authenticate your requests.
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold">
                  2
                </div>
                <h3 className="font-semibold text-lg">Get an API key</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  Generate an API key from your dashboard. Use it with Claude
                  Desktop or any MCP-compatible client.
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="pt-6">
                <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold">
                  3
                </div>
                <h3 className="font-semibold text-lg">Generate podcasts</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  Ask your AI assistant to generate a podcast from any URL or
                  text. It handles everything automatically.
                </p>
              </CardContent>
            </Card>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="py-20">
        <div className="mx-auto max-w-6xl px-4">
          <h2 className="text-center text-3xl font-bold">Features</h2>
          <div className="mt-12 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {[
              {
                title: "AI-powered scripts",
                desc: "Claude or Gemini generates natural, engaging two-host conversations from your content.",
              },
              {
                title: "Multiple voices",
                desc: "Choose from 30+ AI voices across Gemini, ElevenLabs, and Google Cloud TTS.",
              },
              {
                title: "8 show formats",
                desc: "Conversation, interview, deep-dive, explainer, debate, news, storytelling, and challenger.",
              },
              {
                title: "Any content source",
                desc: "URLs, PDFs, plain text files, or clipboard content — Podcaster handles them all.",
              },
              {
                title: "MCP integration",
                desc: "Works with Claude Desktop and any MCP-compatible AI assistant out of the box.",
              },
              {
                title: "CDN delivery",
                desc: "Generated podcasts are hosted on CloudFront for fast, reliable playback worldwide.",
              },
            ].map((feature) => (
              <Card key={feature.title}>
                <CardContent className="pt-6">
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

      {/* Footer */}
      <footer className="border-t py-8">
        <div className="mx-auto max-w-6xl px-4 text-center text-sm text-muted-foreground">
          Podcaster by Apres AI
        </div>
      </footer>
    </div>
  );
}
