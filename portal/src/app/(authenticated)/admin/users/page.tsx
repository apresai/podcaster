import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listUsers } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { UserTable } from "./user-table";

export default async function AdminUsersPage() {
  const session = await auth();
  if (!session?.user?.id || session.user.role !== "admin") {
    redirect("/dashboard");
  }

  const users = await listUsers();

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold">User management</h1>
        <p className="mt-1 text-muted-foreground">
          Manage user accounts and roles
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>All users ({users.length})</CardTitle>
        </CardHeader>
        <CardContent>
          <UserTable initialUsers={users} />
        </CardContent>
      </Card>
    </div>
  );
}
