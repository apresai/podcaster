export default function robots() {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/dashboard", "/api-keys", "/usage", "/admin"],
    },
    sitemap: "https://podcasts.apresai.dev/sitemap.xml",
  };
}
