import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { SessionProvider } from "@/components/session-provider";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    default: "Podcaster",
    template: "%s | Podcaster",
  },
  description:
    "Turn any content into a podcast with AI-powered scripts and 30+ voices.",
  metadataBase: new URL("https://podcasts.apresai.dev"),
  openGraph: {
    title: "Podcaster",
    description:
      "Turn any content into a podcast with AI-powered scripts and 30+ voices.",
    url: "https://podcasts.apresai.dev",
    siteName: "Podcaster",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Podcaster",
    description:
      "Turn any content into a podcast with AI-powered scripts and 30+ voices.",
  },
  robots: { index: true, follow: true },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <SessionProvider>{children}</SessionProvider>
      </body>
    </html>
  );
}
