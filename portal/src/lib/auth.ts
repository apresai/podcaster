import NextAuth from "next-auth";
import type { NextAuthConfig } from "next-auth";
import Cognito from "next-auth/providers/cognito";
import { createUser, getUserByEmail } from "./db";
import { randomBytes } from "crypto";

declare module "next-auth" {
  interface Session {
    user: {
      id: string;
      email: string;
      name: string;
      role: string;
      status: string;
      image?: string | null;
    };
    error?: string;
  }
}

declare module "@auth/core/jwt" {
  interface JWT {
    access_token?: string;
    refresh_token?: string;
    expires_at?: number;
    error?: string;
  }
}

let cachedTokenEndpoint: string | null = null;

async function getTokenEndpoint(): Promise<string> {
  if (cachedTokenEndpoint) return cachedTokenEndpoint;
  const issuer = process.env.COGNITO_ISSUER!;
  const res = await fetch(`${issuer}/.well-known/openid-configuration`);
  const config = await res.json();
  cachedTokenEndpoint = config.token_endpoint as string;
  return cachedTokenEndpoint;
}

const SESSION_MAX_AGE = 90 * 24 * 60 * 60; // 90 days

export const authConfig: NextAuthConfig = {
  trustHost: true,
  providers: [
    Cognito({
      clientId: process.env.COGNITO_CLIENT_ID!,
      clientSecret: process.env.COGNITO_CLIENT_SECRET!,
      issuer: process.env.COGNITO_ISSUER!,
      checks: ["none"],
    }),
  ],
  session: {
    strategy: "jwt",
    maxAge: SESSION_MAX_AGE,
  },
  cookies: {
    sessionToken: {
      name: process.env.NODE_ENV === "production"
        ? "__Secure-authjs.session-token"
        : "authjs.session-token",
      options: {
        httpOnly: true,
        sameSite: "lax",
        path: "/",
        secure: process.env.NODE_ENV === "production",
        maxAge: SESSION_MAX_AGE,
      },
    },
  },
  callbacks: {
    async signIn({ user }) {
      if (!user.email) return false;
      const existing = await getUserByEmail(user.email);
      if (!existing) {
        const userId = randomBytes(16).toString("hex");
        await createUser({
          userId,
          email: user.email,
          name: user.name || user.email.split("@")[0],
        });
      }
      return true;
    },
    async jwt({ token, account }) {
      // Initial sign-in: capture tokens from Cognito
      if (account) {
        return {
          ...token,
          access_token: account.access_token,
          refresh_token: account.refresh_token,
          expires_at: account.expires_at,
        };
      }

      // Subsequent requests: check if access token is still valid
      if (token.expires_at && Date.now() < token.expires_at * 1000) {
        return token;
      }

      // Access token expired — attempt refresh
      if (!token.refresh_token) {
        return { ...token, error: "RefreshAccessTokenError" };
      }

      try {
        const tokenEndpoint = await getTokenEndpoint();
        const res = await fetch(tokenEndpoint, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: new URLSearchParams({
            grant_type: "refresh_token",
            client_id: process.env.COGNITO_CLIENT_ID!,
            client_secret: process.env.COGNITO_CLIENT_SECRET!,
            refresh_token: token.refresh_token,
          }),
        });

        const data = await res.json();
        if (!res.ok) {
          return { ...token, error: "RefreshAccessTokenError" };
        }

        return {
          ...token,
          access_token: data.access_token,
          expires_at: Math.floor(Date.now() / 1000) + data.expires_in,
          // Cognito doesn't rotate refresh tokens, keep existing
          error: undefined,
        };
      } catch {
        return { ...token, error: "RefreshAccessTokenError" };
      }
    },
    async session({ session, token }) {
      if (session.user?.email) {
        const dbUser = await getUserByEmail(session.user.email);
        if (dbUser) {
          session.user.id = dbUser.userId;
          session.user.name = dbUser.name;
          session.user.role = dbUser.role;
          session.user.status = dbUser.status;
        }
      }
      if (token.error) {
        session.error = token.error;
      }
      return session;
    },
  },
  pages: {
    signIn: "/login",
  },
};

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);

export function canCreate(role: string | undefined): boolean {
  return role === "creator" || role === "admin";
}
