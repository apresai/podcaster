export function WaveformLogo({ size = 24 }: { size?: number }) {
  const bars = [
    { height: 40, delay: "0s" },
    { height: 70, delay: "0.15s" },
    { height: 100, delay: "0.3s" },
    { height: 60, delay: "0.45s" },
    { height: 80, delay: "0.1s" },
  ];

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      className="text-primary"
      aria-hidden="true"
    >
      {bars.map((bar, i) => {
        const barHeight = (bar.height / 100) * 20;
        return (
          <rect
            key={i}
            x={2 + i * 4.4}
            y={12 - barHeight / 2}
            width="3"
            rx="1.5"
            height={barHeight}
            fill="currentColor"
            style={{
              animation: "waveform 1.2s ease-in-out infinite",
              animationDelay: bar.delay,
              transformOrigin: "center",
            }}
          />
        );
      })}
    </svg>
  );
}
