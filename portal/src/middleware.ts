export { auth as middleware } from "@/lib/auth";

export const config = {
  matcher: [
    "/dashboard/:path*",
    "/api-keys/:path*",
    "/usage/:path*",
    "/docs/:path*",
    "/admin/:path*",
    "/api/keys/:path*",
    "/api/usage/:path*",
    "/api/admin/:path*",
  ],
};
