"use client";

import { useState } from "react";
import { Link2, Check } from "lucide-react";
import { Button } from "@/components/ui/button";

export function CopyLinkButton({ url }: { url: string }) {
  const [copied, setCopied] = useState(false);

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement("textarea");
      textarea.value = url;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <Button
      variant="ghost"
      size="icon-xs"
      onClick={copyLink}
      aria-label={copied ? "Copied" : "Copy MP3 link"}
      title={copied ? "Copied!" : "Copy MP3 link"}
    >
      {copied ? (
        <Check className="size-4 sm:size-3 text-green-500" />
      ) : (
        <Link2 className="size-4 sm:size-3" />
      )}
    </Button>
  );
}
