# Etapa 1: Compilar el ejecutable usando una imagen oficial de Go
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
ENV CGO_ENABLED=0
RUN go build -o web_server web_server.go
RUN go build -o mapsscrap-1 main.go
RUN go build -o phone_scraper enhanced_phone_scraper.go

# Etapa 2: Imagen de ejecución basada en Debian (Chromium nativo)
FROM debian:bookworm-slim
WORKDIR /app

# Instalar dependencias mínimas y Chromium (paquete nativo para la arquitectura)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        wget \
        fonts-liberation \
        libnss3 \
        libxss1 \
        libatk1.0-0 \
        libatk-bridge2.0-0 \
        libasound2 \
        libgbm1 \
        libgtk-3-0 \
        chromium \
    && rm -rf /var/lib/apt/lists/* && \
    # Provide compatibility symlinks so tools expecting google-chrome find chromium
    ln -sf /usr/bin/chromium /usr/bin/google-chrome && \
    ln -sf /usr/bin/chromium /usr/bin/google-chrome-stable

COPY --from=builder /app/web_server .
COPY --from=builder /app/pipeline.sh .
COPY --from=builder /app/phone_scraper .
COPY --from=builder /app/mapsscrap-1 .
COPY --from=builder /app/web ./web
RUN chmod +x web_server pipeline.sh phone_scraper
EXPOSE 8080
CMD ["./web_server"]