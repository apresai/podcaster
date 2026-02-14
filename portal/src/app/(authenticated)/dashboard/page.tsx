import { auth, canCreate } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listUserPodcasts, listAPIKeys, listAllPodcasts } from "@/lib/db";
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
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatCost(cost: number | undefined | null) {
  if (cost === undefined || cost === null) return "\u2014";
  return `$${cost.toFixed(2)}`;
}

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const isSuspended = session.user.status === "suspended";

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

  const isCreator = canCreate(session.user.role);

  if (isCreator) {
    return <CreatorDashboard userId={session.user.id} name={session.user.name} />;
  }

  return <BrowseDashboard name={session.user.name} />;
}

async function CreatorDashboard({ userId, name }: { userId: string; name: string }) {
  const [allPodcasts, keys] = await Promise.all([
    listUserPodcasts(userId, 100),
    listAPIKeys(userId),
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
        <h1 className="text-2xl sm:text-3xl font-bold">Dashboard</h1>
        <p className="mt-1 text-muted-foreground">
          Welcome back, {name}
        </p>
      </div>

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
            <div className="text-2xl font-bold">{podcastCount}</div>
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
            <div className="text-2xl font-bold">{formatCost(totalCostUSD)}</div>
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

async function BrowseDashboard({ name }: { name: string }) {
  const podcasts = await listAllPodcasts(50);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold">Dashboard</h1>
        <p className="mt-1 text-muted-foreground">
          Welcome, {name} &mdash; browse and listen to all podcasts
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
