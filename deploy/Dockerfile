# Stage 1: Build Go binary
FROM --platform=linux/arm64 golang:1.24-bookworm AS builder

WORKDIR /build

# Copy the SDK dependency (referenced via replace directive)
COPY deploy/sdk/ /deps/apresai.dev/sdk/

# Copy go.mod/sum first for layer caching
COPY go.mod go.sum ./

# Patch replace directive to point to copied SDK
RUN sed -i 's|=> ../apresai.dev/sdk|=> /deps/apresai.dev/sdk|' go.mod

RUN go mod download

# Copy source
COPY . .
# Re-apply replace patch (COPY overwrites go.mod)
RUN sed -i 's|=> ../apresai.dev/sdk|=> /deps/apresai.dev/sdk|' go.mod

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /mcp-server ./cmd/mcp-server

# Stage 2: Minimal runtime with FFmpeg
FROM --platform=linux/arm64 debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Run as non-root
RUN useradd -r -s /bin/false podcaster && \
    mkdir -p /app && chown podcaster:podcaster /app
USER podcaster

COPY --from=builder /mcp-server /usr/local/bin/mcp-server

WORKDIR /app

EXPOSE 8000

ENTRYPOINT ["mcp-server"]
