"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { APIKey } from "@/lib/db";

function formatDate(iso: string) {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

export function APIKeyManager({ initialKeys }: { initialKeys: APIKey[] }) {
  const [keys, setKeys] = useState(initialKeys);
  const [name, setName] = useState("");
  const [newKey, setNewKey] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [revokePrefix, setRevokePrefix] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleCreate() {
    if (!name.trim()) return;
    setCreating(true);
    try {
      const res = await fetch("/api/keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: name.trim() }),
      });
      const data = await res.json();
      setNewKey(data.fullKey);
      setKeys((prev) => [
        {
          prefix: data.prefix,
          userId: "",
          keyHash: "",
          name: name.trim(),
          status: "active",
          createdAt: new Date().toISOString(),
        },
        ...prev,
      ]);
      setName("");
    } finally {
      setCreating(false);
    }
  }

  async function handleRevoke(prefix: string) {
    await fetch(`/api/keys/${prefix}`, { method: "DELETE" });
    setKeys((prev) =>
      prev.map((k) => (k.prefix === prefix ? { ...k, status: "revoked" } : k))
    );
    setRevokePrefix(null);
  }

  async function copyKey() {
    if (newKey) {
      await navigator.clipboard.writeText(newKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  return (
    <div className="space-y-4">
      {/* New key display */}
      {newKey && (
        <div className="rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-900 dark:bg-green-950">
          <p className="text-sm font-medium mb-2">
            Your new API key (shown only once):
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded bg-white px-3 py-2 text-sm font-mono dark:bg-black break-all">
              {newKey}
            </code>
            <Button variant="outline" size="sm" onClick={copyKey}>
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
          <Button
            variant="ghost"
            size="sm"
            className="mt-2"
            onClick={() => setNewKey(null)}
          >
            Dismiss
          </Button>
        </div>
      )}

      {/* Create button */}
      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
          if (!open) setName("");
        }}
      >
        <DialogTrigger asChild>
          <Button>Create API key</Button>
        </DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create API key</DialogTitle>
            <DialogDescription>
              Give your key a name so you can identify it later.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="key-name">Name</Label>
            <Input
              id="key-name"
              placeholder="e.g. Claude Desktop"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  handleCreate();
                  setCreateOpen(false);
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button
              onClick={() => {
                handleCreate();
                setCreateOpen(false);
              }}
              disabled={creating || !name.trim()}
            >
              {creating ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Keys table */}
      {keys.length > 0 ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Key</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Last used</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {keys.map((key) => (
              <TableRow key={key.prefix}>
                <TableCell className="font-mono text-sm">
                  pk_{key.prefix}...
                </TableCell>
                <TableCell>{key.name}</TableCell>
                <TableCell>
                  <Badge
                    variant={
                      key.status === "active" ? "default" : "secondary"
                    }
                  >
                    {key.status}
                  </Badge>
                </TableCell>
                <TableCell className="text-sm">
                  {formatDate(key.createdAt)}
                </TableCell>
                <TableCell className="text-sm">
                  {key.lastUsedAt ? formatDate(key.lastUsedAt) : "Never"}
                </TableCell>
                <TableCell>
                  {key.status === "active" && (
                    <Dialog
                      open={revokePrefix === key.prefix}
                      onOpenChange={(open) =>
                        setRevokePrefix(open ? key.prefix : null)
                      }
                    >
                      <DialogTrigger asChild>
                        <Button variant="ghost" size="sm">
                          Revoke
                        </Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>Revoke API key</DialogTitle>
                          <DialogDescription>
                            This will permanently revoke the key{" "}
                            <span className="font-mono">
                              pk_{key.prefix}...
                            </span>
                            . Any clients using this key will lose access.
                          </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button
                            variant="outline"
                            onClick={() => setRevokePrefix(null)}
                          >
                            Cancel
                          </Button>
                          <Button
                            variant="destructive"
                            onClick={() => handleRevoke(key.prefix)}
                          >
                            Revoke
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      ) : (
        <p className="text-sm text-muted-foreground py-4">
          No API keys yet. Create one to get started.
        </p>
      )}
    </div>
  );
}
