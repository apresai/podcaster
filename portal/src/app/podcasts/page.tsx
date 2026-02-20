import { listAllPodcasts } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PodcastAudioControls } from "@/components/podcast-audio";
import { Mic } from "lucide-react";
import { CopyLinkButton } from "@/components/copy-link-button";

function formatDate(iso: string) {
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export const metadata = {
  title: "Podcasts",
  description: "Browse and listen to AI-generated podcasts.",
};

export default async function PodcastsPage() {
  const allPodcasts = await listAllPodcasts(100);
  const podcasts = allPodcasts.filter((p) => p.status === "completed");

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold">Podcasts</h1>
        <p className="mt-1 text-muted-foreground">
          Browse and listen to AI-generated podcasts
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All podcasts</CardTitle>
        </CardHeader>
        <CardContent>
          {podcasts.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">
              No podcasts available yet.
            </p>
          ) : (
            <div className="space-y-3">
              {podcasts.map((p) => (
                <div
                  key={p.podcastId}
                  className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between rounded-lg border p-3 hover:bg-accent/30 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex items-center justify-center size-8 rounded-full bg-primary/10 text-primary shrink-0">
                      <Mic className="size-4" />
                    </div>
                    <div className="space-y-1">
                      <div className="font-medium">{p.title}</div>
                      <div className="text-xs text-muted-foreground">
                        {formatDate(p.createdAt)}
                        {p.model && ` \u00b7 ${p.model}`}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 pl-11 sm:pl-0">
                    <Badge variant="default">{p.status}</Badge>
                    {p.audioUrl && (
                      <>
                        <PodcastAudioControls
                          audioUrl={p.audioUrl}
                          title={p.title || "podcast"}
                        />
                        <CopyLinkButton url={p.audioUrl} />
                      </>
                    )}
                    {p.scriptUrl && (
                      <a
                        href={p.scriptUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary hover:underline"
                      >
                        Script
                      </a>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
