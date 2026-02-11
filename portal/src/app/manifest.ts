import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Podcaster",
    short_name: "Podcaster",
    description: "Turn any content into a podcast",
    start_url: "/",
    display: "standalone",
    background_color: "#1a1a1a",
    theme_color: "#f97316",
    icons: [{ src: "/icon.png", sizes: "192x192", type: "image/png" }],
  };
}
