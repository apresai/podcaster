"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Play, Pause, Download } from "lucide-react";
import { Button } from "@/components/ui/button";

// Module-level singleton — only one track plays at a time across the page
let globalAudio: HTMLAudioElement | null = null;
let globalOwner: string | null = null;

export function PodcastAudioControls({
  audioUrl,
  title,
}: {
  audioUrl: string;
  title: string;
}) {
  const [isPlaying, setIsPlaying] = useState(false);
  const instanceId = useRef(Math.random().toString(36).slice(2));

  const syncState = useCallback(() => {
    setIsPlaying(
      globalOwner === instanceId.current &&
        globalAudio !== null &&
        !globalAudio.paused
    );
  }, []);

  useEffect(() => {
    const interval = setInterval(syncState, 300);
    return () => clearInterval(interval);
  }, [syncState]);

  const togglePlay = () => {
    if (!globalAudio) {
      globalAudio = new Audio();
    }

    if (globalOwner === instanceId.current && !globalAudio.paused) {
      globalAudio.pause();
      setIsPlaying(false);
      return;
    }

    // Take ownership and play
    globalAudio.pause();
    globalAudio.src = audioUrl;
    globalOwner = instanceId.current;
    globalAudio.play().catch(() => {});

    globalAudio.onended = () => {
      globalOwner = null;
      setIsPlaying(false);
    };
    globalAudio.onpause = () => syncState();

    setIsPlaying(true);
  };

  const filename = `${title.replace(/[^a-zA-Z0-9]+/g, "-").replace(/-+$/, "").toLowerCase()}.mp3`;

  return (
    <span className="inline-flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon-xs"
        onClick={togglePlay}
        aria-label={isPlaying ? "Pause" : "Play"}
      >
        {isPlaying ? (
          <Pause className="size-3" />
        ) : (
          <Play className="size-3" />
        )}
      </Button>
      <Button variant="ghost" size="icon-xs" asChild>
        <a href={audioUrl} download={filename} aria-label="Download">
          <Download className="size-3" />
        </a>
      </Button>
    </span>
  );
}
