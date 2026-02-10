import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listMonthlyUsage, listUserPodcasts } from "@/lib/db";
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
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatDuration(seconds: number) {
  if (!seconds) return "—";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs}s`;
}

function formatCost(cost: number | undefined) {
  if (cost === undefined || cost === null) return "—";
  return `$${cost.toFixed(2)}`;
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

export default async function UsagePage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  const [usage, podcasts] = await Promise.all([
    listMonthlyUsage(session.user.id),
    listUserPodcasts(session.user.id, 50),
  ]);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Usage</h1>
        <p className="mt-1 text-muted-foreground">
          Your monthly usage and podcast history
        </p>
      </div>

      {/* Monthly usage */}
      <Card>
        <CardHeader>
          <CardTitle>Monthly usage</CardTitle>
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
                  <TableHead>Month</TableHead>
                  <TableHead>Podcasts</TableHead>
                  <TableHead>Total duration</TableHead>
                  <TableHead>Estimated cost</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usage.map((u) => (
                  <TableRow key={u.month}>
                    <TableCell className="font-medium">{u.month}</TableCell>
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

      {/* Podcast history */}
      <Card>
        <CardHeader>
          <CardTitle>Podcast history</CardTitle>
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
                  <TableHead>Status</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>TTS</TableHead>
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
                    <TableCell>
                      <Badge variant={statusColor(p.status)}>{p.status}</Badge>
                    </TableCell>
                    <TableCell className="text-sm">
                      {p.model || "—"}
                    </TableCell>
                    <TableCell className="text-sm">
                      {p.ttsProvider || "—"}
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
