import Link from "next/link";
import { Button } from "@/components/ui/button";
import { WaveformLogo } from "@/components/waveform-logo";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 px-4">
      <WaveformLogo size={48} />
      <h1 className="text-6xl font-bold text-primary">404</h1>
      <p className="text-lg text-muted-foreground">
        This page doesn&apos;t exist.
      </p>
      <Link href="/">
        <Button>Back to home</Button>
      </Link>
    </div>
  );
}
