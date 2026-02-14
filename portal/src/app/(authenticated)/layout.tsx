import { redirect } from "next/navigation";
import { auth } from "@/lib/auth";
import { Nav } from "@/components/nav";

export default async function AuthenticatedLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();
  if (!session) {
    redirect("/login");
  }

  return (
    <div className="min-h-screen">
      <Nav />
      <main className="mx-auto max-w-6xl px-3 py-4 sm:px-4 sm:py-8">{children}</main>
    </div>
  );
}
