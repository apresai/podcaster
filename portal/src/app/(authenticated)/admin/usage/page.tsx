import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listAllPodcasts, listAllUsage } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

function formatDate(iso: string) {
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatCost(cost: number | undefined) {
  if (cost === undefined || cost === null) return "\u2014";
  return `$${cost.toFixed(2)}`;
}

function formatDuration(seconds: number) {
  if (!seconds) return "\u2014";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs}s`;
}

function statusColor(status: string) {
  switch (status) {
    case "completed":
      return "default" as const;
    case "processing":
    case "in_progress":
      return "secondary" as const;
    case "failed":
      return "destructive" as const;
    default:
      return "outline" as const;
  }
}

export default async function AdminUsagePage() {
  const session = await auth();
  if (!session?.user?.id || session.user.role !== "admin") {
    redirect("/dashboard");
  }

  const [usage, podcasts] = await Promise.all([
    listAllUsage(),
    listAllPodcasts(100),
  ]);

  const totalCost = usage.reduce((sum, u) => sum + u.totalCostUSD, 0);
  const totalPodcasts = usage.reduce((sum, u) => sum + u.podcastCount, 0);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">All usage</h1>
        <p className="mt-1 text-muted-foreground">
          Usage across all users
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total podcasts
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalPodcasts}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total estimated cost
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatCost(totalCost)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Unique users
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {new Set(usage.map((u) => u.userId)).size}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Monthly usage by user</CardTitle>
        </CardHeader>
        <CardContent>
          {usage.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">
              No usage data yet.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Month</TableHead>
                  <TableHead>Podcasts</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Cost</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usage.map((u) => (
                  <TableRow key={`${u.userId}-${u.month}`}>
                    <TableCell className="font-mono text-sm">
                      {u.userId.slice(0, 8)}...
                    </TableCell>
                    <TableCell>{u.month}</TableCell>
                    <TableCell>{u.podcastCount}</TableCell>
                    <TableCell>{formatDuration(u.totalDurationSec)}</TableCell>
                    <TableCell>{formatCost(u.totalCostUSD)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Recent podcasts (all users)</CardTitle>
        </CardHeader>
        <CardContent>
          {podcasts.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">
              No podcasts yet.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Cost</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {podcasts.map((p) => (
                  <TableRow key={p.podcastId}>
                    <TableCell className="font-medium max-w-[200px] truncate">
                      {p.title}
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {p.userId ? `${p.userId.slice(0, 8)}...` : "\u2014"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusColor(p.status)}>{p.status}</Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {p.model || "\u2014"}
                    </TableCell>
                    <TableCell className="text-sm">
                      {formatCost(p.estimatedCostUSD)}
                    </TableCell>
                    <TableCell className="text-sm">
                      {formatDate(p.createdAt)}
                    </TableCell>
                    <TableCell className="space-x-2">
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
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
