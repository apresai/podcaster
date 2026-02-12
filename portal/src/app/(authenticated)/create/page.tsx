import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listAPIKeys } from "@/lib/db";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { KeyRound } from "lucide-react";
import { CreatePodcastForm } from "./create-form";

export default async function CreatePage() {
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

  const keys = await listAPIKeys(session.user.id);
  const hasActiveKey = keys.some(
    (k) => k.status === "active" && k.encryptedKey
  );

  if (!hasActiveKey) {
    return (
      <div className="space-y-8">
        <div>
          <h1 className="text-3xl font-bold">Create Podcast</h1>
          <p className="mt-1 text-muted-foreground">
            Generate a podcast from any URL or text
          </p>
        </div>
        <Card className="max-w-lg">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="size-5" />
              API Key Required
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              To create podcasts from the web portal, you need an active API
              key. Create one on the API Keys page to get started.
            </p>
            <Link href="/api-keys">
              <Button>Create API Key</Button>
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Create Podcast</h1>
        <p className="mt-1 text-muted-foreground">
          Generate a podcast from any URL or text
        </p>
      </div>
      <CreatePodcastForm />
    </div>
  );
}
