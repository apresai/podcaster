"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
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
import type { User } from "@/lib/db";

function formatDate(iso: string | undefined) {
  if (!iso) return "\u2014";
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function statusBadge(status: string) {
  switch (status) {
    case "active":
      return "default" as const;
    case "pending":
      return "secondary" as const;
    case "suspended":
      return "destructive" as const;
    default:
      return "outline" as const;
  }
}

function roleBadge(role: string) {
  switch (role) {
    case "admin":
      return "default" as const;
    case "creator":
      return "secondary" as const;
    default:
      return "outline" as const;
  }
}

export function UserTable({ initialUsers }: { initialUsers: User[] }) {
  const [users, setUsers] = useState(initialUsers);
  const [confirmAction, setConfirmAction] = useState<{
    userId: string;
    action: "approve" | "suspend" | "promote" | "demote";
  } | null>(null);

  async function handleAction(userId: string, action: "approve" | "suspend") {
    await fetch(`/api/admin/users/${userId}/${action}`, { method: "POST" });
    setUsers((prev) =>
      prev.map((u) =>
        u.userId === userId
          ? { ...u, status: action === "approve" ? "active" : "suspended" }
          : u
      )
    );
    setConfirmAction(null);
  }

  async function handleRoleChange(userId: string, role: "user" | "creator") {
    await fetch(`/api/admin/users/${userId}/role`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ role }),
    });
    setUsers((prev) =>
      prev.map((u) => (u.userId === userId ? { ...u, role } : u))
    );
    setConfirmAction(null);
  }

  return (
    <>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Email</TableHead>
            <TableHead>Name</TableHead>
            <TableHead>Status</TableHead>
            <TableHead className="hidden sm:table-cell">Role</TableHead>
            <TableHead className="hidden sm:table-cell">Created</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => (
            <TableRow key={user.userId}>
              <TableCell className="font-medium">{user.email}</TableCell>
              <TableCell>{user.name}</TableCell>
              <TableCell>
                <Badge variant={statusBadge(user.status)}>{user.status}</Badge>
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                <Badge variant={roleBadge(user.role)}>{user.role}</Badge>
              </TableCell>
              <TableCell className="hidden sm:table-cell text-sm">
                {formatDate(user.createdAt)}
              </TableCell>
              <TableCell>
                <div className="flex gap-2">
                  {user.status === "pending" && (
                    <Dialog
                      open={
                        confirmAction?.userId === user.userId &&
                        confirmAction?.action === "approve"
                      }
                      onOpenChange={(open) =>
                        setConfirmAction(
                          open
                            ? { userId: user.userId, action: "approve" }
                            : null
                        )
                      }
                    >
                      <DialogTrigger asChild>
                        <Button size="sm">Approve</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>Approve user</DialogTitle>
                          <DialogDescription>
                            Approve {user.email} for access to Podcaster? They
                            will get read-only access. You can promote them to
                            creator later.
                          </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button
                            variant="outline"
                            onClick={() => setConfirmAction(null)}
                          >
                            Cancel
                          </Button>
                          <Button
                            onClick={() =>
                              handleAction(user.userId, "approve")
                            }
                          >
                            Approve
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  )}
                  {user.status === "active" && user.role === "user" && (
                    <Dialog
                      open={
                        confirmAction?.userId === user.userId &&
                        confirmAction?.action === "promote"
                      }
                      onOpenChange={(open) =>
                        setConfirmAction(
                          open
                            ? { userId: user.userId, action: "promote" }
                            : null
                        )
                      }
                    >
                      <DialogTrigger asChild>
                        <Button size="sm" variant="outline">Promote</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>Promote to creator</DialogTitle>
                          <DialogDescription>
                            Promote {user.email} to creator? They will be able
                            to create API keys and generate podcasts.
                          </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button
                            variant="outline"
                            onClick={() => setConfirmAction(null)}
                          >
                            Cancel
                          </Button>
                          <Button
                            onClick={() =>
                              handleRoleChange(user.userId, "creator")
                            }
                          >
                            Promote
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  )}
                  {user.status === "active" && user.role === "creator" && (
                    <Dialog
                      open={
                        confirmAction?.userId === user.userId &&
                        confirmAction?.action === "demote"
                      }
                      onOpenChange={(open) =>
                        setConfirmAction(
                          open
                            ? { userId: user.userId, action: "demote" }
                            : null
                        )
                      }
                    >
                      <DialogTrigger asChild>
                        <Button size="sm" variant="outline">Demote</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>Demote to user</DialogTitle>
                          <DialogDescription>
                            Demote {user.email} to read-only user? They will
                            lose the ability to create podcasts and API keys.
                          </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button
                            variant="outline"
                            onClick={() => setConfirmAction(null)}
                          >
                            Cancel
                          </Button>
                          <Button
                            variant="destructive"
                            onClick={() =>
                              handleRoleChange(user.userId, "user")
                            }
                          >
                            Demote
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  )}
                  {user.status === "active" && user.role !== "admin" && (
                    <Dialog
                      open={
                        confirmAction?.userId === user.userId &&
                        confirmAction?.action === "suspend"
                      }
                      onOpenChange={(open) =>
                        setConfirmAction(
                          open
                            ? { userId: user.userId, action: "suspend" }
                            : null
                        )
                      }
                    >
                      <DialogTrigger asChild>
                        <Button variant="ghost" size="sm">
                          Suspend
                        </Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>Suspend user</DialogTitle>
                          <DialogDescription>
                            Suspend {user.email}? They will lose access to
                            Podcaster and their API keys will stop working.
                          </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                          <Button
                            variant="outline"
                            onClick={() => setConfirmAction(null)}
                          >
                            Cancel
                          </Button>
                          <Button
                            variant="destructive"
                            onClick={() =>
                              handleAction(user.userId, "suspend")
                            }
                          >
                            Suspend
                          </Button>
                        </DialogFooter>
                      </DialogContent>
                    </Dialog>
                  )}
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </>
  );
}
