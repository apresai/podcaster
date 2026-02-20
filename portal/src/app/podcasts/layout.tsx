import { auth } from "@/lib/auth";
import { PublicNav } from "@/components/public-nav";

export default async function PodcastsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();

  return (
    <div className="min-h-screen">
      <PublicNav isLoggedIn={!!session} />
      <main className="mx-auto max-w-6xl px-3 py-4 sm:px-4 sm:py-8">{children}</main>
    </div>
  );
}
