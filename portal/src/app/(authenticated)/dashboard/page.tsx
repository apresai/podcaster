import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listUserPodcasts, listAPIKeys, getMonthlyUsage } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
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

function formatCost(cost: number | undefined) {
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

  const [podcasts, keys, usage] = await Promise.all([
    listUserPodcasts(session.user.id, 5),
    listAPIKeys(session.user.id),
    getMonthlyUsage(
      session.user.id,
      new Date().toISOString().slice(0, 7)
    ),
  ]);

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
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Podcasts this month
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {usage?.podcastCount ?? 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Estimated cost
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCost(usage?.totalCostUSD)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
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
                  className="flex items-center justify-between rounded-lg border p-3"
                >
                  <div className="space-y-1">
                    <div className="font-medium">{p.title}</div>
                    <div className="text-xs text-muted-foreground">
                      {formatDate(p.createdAt)}
                      {p.model && ` · ${p.model}`}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={statusColor(p.status)}>{p.status}</Badge>
                    {p.audioUrl && (
                      <a
                        href={p.audioUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary hover:underline"
                      >
                        Listen
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
