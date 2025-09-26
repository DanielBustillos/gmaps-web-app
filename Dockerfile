## Multi-stage Dockerfile for Render
# 1) builder stage compiles Go binaries for Linux
# 2) final stage installs Google Chrome and copies binaries

FROM golang:1.20-bullseye AS builder
WORKDIR /src

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mapsscrap-1 main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o web_server web_server.go

## Final runtime image
FROM debian:bookworm-slim
LABEL org.opencontainers.image.source="https://github.com/DanielBustillos/gmaps-web-app"

ENV DEBIAN_FRONTEND=noninteractive

# Install Chrome dependencies and google-chrome-stable
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget gnupg fonts-liberation libnss3 libx11-xcb1 libxcomposite1 libxcursor1 \
    libxdamage1 libxrandr2 libasound2 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 \
    libgbm1 libpangocairo-1.0-0 libpango-1.0-0 libxcb1 libxss1 libxtst6 libxrender1 xdg-utils \
    --no-install-recommends && \
    wget -q -O /tmp/google-chrome-stable_current_amd64.deb https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && \
    apt-get install -y /tmp/google-chrome-stable_current_amd64.deb && \
    rm -f /tmp/google-chrome-stable_current_amd64.deb && \
    apt-get purge -y --auto-remove wget gnupg && \
    rm -rf /var/lib/apt/lists/*

# Copy compiled binaries from builder
COPY --from=builder /src/mapsscrap-1 /usr/local/bin/mapsscrap-1
COPY --from=builder /src/web_server /usr/local/bin/web_server

WORKDIR /app
EXPOSE 8080
ENV PORT=8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD curl -f http://localhost:8080/ || exit 1

ENTRYPOINT ["/usr/local/bin/web_server"]
