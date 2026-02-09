"use client";

import { signIn } from "next-auth/react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function LoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Podcaster</CardTitle>
          <CardDescription>
            Sign in to manage your podcasts and API keys
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            className="w-full"
            onClick={() => signIn("cognito", { callbackUrl: "/dashboard" })}
          >
            Sign in with Cognito
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
