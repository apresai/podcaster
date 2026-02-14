import { auth, canCreate } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listAPIKeys } from "@/lib/db";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { APIKeyManager } from "./key-manager";

export default async function APIKeysPage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  if (!canCreate(session.user.role)) {
    redirect("/dashboard");
  }

  const keys = await listAPIKeys(session.user.id);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl sm:text-3xl font-bold">API Keys</h1>
        <p className="mt-1 text-muted-foreground">
          Manage your API keys for accessing the Podcaster MCP server
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Your keys</CardTitle>
        </CardHeader>
        <CardContent>
          <APIKeyManager initialKeys={keys} />
        </CardContent>
      </Card>
    </div>
  );
}
