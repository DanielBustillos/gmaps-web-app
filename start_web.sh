#!/bin/bash

# Script para iniciar el servidor web del Maps Scraper

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log "🌐 Iniciando Maps Scraper Web Server..."

# Verificar que Go está instalado
if ! command -v go &> /dev/null; then
    error "Go no está instalado. Por favor instala Go desde https://golang.org/download/"
    exit 1
fi

# Verificar dependencias
log "📦 Verificando dependencias..."
go mod tidy

# Compilar binarios necesarios si no existen
if [ ! -f "mapsscrap-1" ]; then
    log "🔨 Compilando mapsscrap-1..."
    go build -o mapsscrap-1 main.go
    success "mapsscrap-1 compilado exitosamente"
fi

if [ ! -f "phone_scraper" ]; then
    log "🔨 Compilando phone_scraper..."
    go build -o phone_scraper enhanced_phone_scraper.go
    success "phone_scraper compilado exitosamente"
fi

# Compilar y ejecutar servidor web
log "🚀 Compilando y ejecutando servidor web..."
go build -o web_server web_server.go

# Verificar que el directorio web existe
if [ ! -d "web" ]; then
    error "Directorio 'web' no encontrado"
    exit 1
fi

log "🌐 Servidor web iniciado"
log "📱 Interfaz disponible en: http://localhost:8080"
log "🛑 Presiona Ctrl+C para detener el servidor"

# Ejecutar servidor web
./web_server