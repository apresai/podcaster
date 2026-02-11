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
import { WaveformLogo } from "@/components/waveform-logo";

export default function LoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center px-4 bg-muted/30">
      <div className="w-full max-w-sm overflow-hidden rounded-lg border bg-card shadow-sm">
        <div className="h-1 gradient-bg" />
        <Card className="border-0 shadow-none rounded-none">
          <CardHeader className="text-center">
            <div className="flex justify-center mb-2">
              <WaveformLogo size={32} />
            </div>
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
    </div>
  );
}
