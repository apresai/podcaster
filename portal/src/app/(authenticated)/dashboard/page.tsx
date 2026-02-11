import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listUserPodcasts, listAPIKeys } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { PodcastAudioControls } from "@/components/podcast-audio";
import { Mic, DollarSign, KeyRound } from "lucide-react";
import Link from "next/link";

function statusColor(status: string) {
  switch (status) {
    case "completed":
      return "default";
    case "processing":
    case "in_progress":
      return "secondary";
    case "failed":
      return "destructive";
    default:
      return "outline";
  }
}

function formatDate(iso: string) {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatCost(cost: number | undefined | null) {
  if (cost === undefined || cost === null) return "—";
  return `$${cost.toFixed(2)}`;
}

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const isPending = session.user.status === "pending";
  const isSuspended = session.user.status === "suspended";

  if (isPending) {
    return (
      <div className="max-w-lg mx-auto mt-12">
        <Alert>
          <AlertDescription>
            Your account is pending approval. You will be notified once an
            administrator reviews your request.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  if (isSuspended) {
    return (
      <div className="max-w-lg mx-auto mt-12">
        <Alert variant="destructive">
          <AlertDescription>
            Your account has been suspended. Please contact support for
            assistance.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const [allPodcasts, keys] = await Promise.all([
    listUserPodcasts(session.user.id, 100),
    listAPIKeys(session.user.id),
  ]);

  const currentMonth = new Date().toISOString().slice(0, 7);
  const thisMonthPodcasts = allPodcasts.filter(
    (p) => p.createdAt?.startsWith(currentMonth)
  );
  const podcastCount = thisMonthPodcasts.length;
  const totalCostUSD = thisMonthPodcasts.reduce(
    (sum, p) => sum + (p.estimatedCostUSD ?? 0),
    0
  );
  const podcasts = allPodcasts.slice(0, 5);
  const activeKeys = keys.filter((k) => k.status === "active");

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="mt-1 text-muted-foreground">
          Welcome back, {session.user.name}
        </p>
      </div>

      {/* Usage summary */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="overflow-hidden">
          <div className="h-1 bg-primary" />
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <div className="flex items-center justify-center size-8 rounded-lg bg-primary/10 text-primary">
                <Mic className="size-4" />
              </div>
              Podcasts this month
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {podcastCount}
            </div>
          </CardContent>
        </Card>
        <Card className="overflow-hidden">
          <div className="h-1 bg-primary" />
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <div className="flex items-center justify-center size-8 rounded-lg bg-primary/10 text-primary">
                <DollarSign className="size-4" />
              </div>
              Estimated cost
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCost(totalCostUSD)}
            </div>
          </CardContent>
        </Card>
        <Card className="overflow-hidden">
          <div className="h-1 bg-primary" />
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <div className="flex items-center justify-center size-8 rounded-lg bg-primary/10 text-primary">
                <KeyRound className="size-4" />
              </div>
              Active API keys
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeKeys.length}</div>
            {activeKeys.length === 0 && (
              <Link
                href="/api-keys"
                className="text-sm text-primary hover:underline"
              >
                Create one
              </Link>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent podcasts */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Recent podcasts</CardTitle>
          <Link href="/usage">
            <Button variant="ghost" size="sm">
              View all
            </Button>
          </Link>
        </CardHeader>
        <CardContent>
          {podcasts.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">
              No podcasts yet. Use your API key with Claude Desktop to generate
              your first one.
            </p>
          ) : (
            <div className="space-y-3">
              {podcasts.map((p) => (
                <div
                  key={p.podcastId}
                  className="flex items-center justify-between rounded-lg border p-3 hover:bg-accent/30 transition-colors"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex items-center justify-center size-8 rounded-full bg-primary/10 text-primary shrink-0">
                      <Mic className="size-4" />
                    </div>
                    <div className="space-y-1">
                      <div className="font-medium">{p.title}</div>
                      <div className="text-xs text-muted-foreground">
                        {formatDate(p.createdAt)}
                        {p.model && ` · ${p.model}`}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={statusColor(p.status)}>{p.status}</Badge>
                    {p.audioUrl && (
                      <PodcastAudioControls
                        audioUrl={p.audioUrl}
                        title={p.title || "podcast"}
                      />
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
