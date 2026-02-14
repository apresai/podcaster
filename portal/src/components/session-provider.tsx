"use client";

import { SessionProvider as NextAuthSessionProvider, useSession, signIn } from "next-auth/react";
import { useEffect } from "react";

function SessionGuard({ children }: { children: React.ReactNode }) {
  const { data: session } = useSession();

  useEffect(() => {
    if ((session as { error?: string } | null)?.error === "RefreshAccessTokenError") {
      signIn("cognito");
    }
  }, [(session as { error?: string } | null)?.error]);

  return <>{children}</>;
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextAuthSessionProvider refetchInterval={5 * 60} refetchOnWindowFocus={true}>
      <SessionGuard>{children}</SessionGuard>
    </NextAuthSessionProvider>
  );
}
