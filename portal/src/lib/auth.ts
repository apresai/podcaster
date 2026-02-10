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
  }
}

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
    async session({ session }) {
      if (session.user?.email) {
        const dbUser = await getUserByEmail(session.user.email);
        if (dbUser) {
          session.user.id = dbUser.userId;
          session.user.role = dbUser.role;
          session.user.status = dbUser.status;
        }
      }
      return session;
    },
  },
  pages: {
    signIn: "/login",
  },
};

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);
