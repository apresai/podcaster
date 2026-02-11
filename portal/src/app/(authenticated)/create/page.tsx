import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";
import { listAPIKeys } from "@/lib/db";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import { CreatePodcastForm } from "./create-form";

export default async function CreatePage() {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");

  if (session.user.status === "pending") {
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

  if (session.user.status === "suspended") {
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
    (k) => k.status === "active",
  );

  if (!hasActiveKey) {
    return (
      <div className="space-y-8">
        <div>
          <h1 className="text-3xl font-bold">Create Podcast</h1>
          <p className="mt-1 text-muted-foreground">
            Generate a podcast from any URL or text content
          </p>
        </div>
        <Alert>
          <AlertDescription className="flex items-center justify-between">
            <span>
              You need an API key to generate podcasts. Create one first.
            </span>
            <Link href="/api-keys">
              <Button size="sm">Create API Key</Button>
            </Link>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Create Podcast</h1>
        <p className="mt-1 text-muted-foreground">
          Generate a podcast from any URL or text content
        </p>
      </div>
      <CreatePodcastForm />
    </div>
  );
}
