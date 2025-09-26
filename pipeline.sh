#!/bin/bash

# Pipeline para ejecutar mapsscrap y luego extraer teléfonos
# Uso: ./pipeline.sh <lat> <lon> <query> <radius>

set -e  # Salir si cualquier comando falla

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Función para mostrar mensajes con colores
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

# Validar argumentos
if [ $# -ne 4 ]; then
    error "Uso: $0 <latitud> <longitud> <consulta> <radio_km>"
    error "Ejemplo: $0 19.1019061 -98.2810447 \"spa\" 2.0"
    exit 1
fi

LAT=$1
LON=$2
QUERY=$3
RADIUS=$4

log "🚀 Iniciando pipeline de scraping de Google Maps"
log "📍 Coordenadas: $LAT, $LON"
log "🔍 Consulta: $QUERY"
log "📏 Radio: ${RADIUS}km"

# NOTE: Google Chrome debe estar instalado durante el proceso de build (p. ej. en Render).
# No instalamos Chrome en cada ejecución del pipeline para ahorrar tiempo y evitar
# realizar tareas que deben hacerse en el build/deploy.
if ! which google-chrome > /dev/null 2>&1; then
        error "Google Chrome no encontrado en el entorno. Debe instalarse durante el build/deploy."
        error "Añade la instalación de Chrome al Build Command (p. ej. en render.yaml)."
        exit 1
fi

success "✅ Google Chrome disponible"

# Verificar que mapsscrap-1 existe
if [ ! -f "./mapsscrap-1" ]; then
    error "El ejecutable mapsscrap-1 no existe. Ejecuta 'make build-all' primero."
    exit 1
fi

# Verificar permisos de ejecución para mapsscrap-1
chmod +x ./mapsscrap-1

# Paso 1: Ejecutar mapsscrap-1 para obtener lugares
log "📊 Paso 1: Ejecutando mapsscrap-1 para obtener lugares..."
./mapsscrap-1 --lat "$LAT" --lon "$LON" --query "$QUERY" --radius "$RADIUS"

if [ $? -ne 0 ]; then
    error "Error ejecutando mapsscrap-1"
    exit 1
fi

# Encontrar el archivo CSV más reciente generado
SANITIZED_QUERY=$(echo "$QUERY" | sed 's/ /_/g' | sed 's/[\/\\]/_/g')

# Intentar diferentes patrones de nombres de archivos CSV
CSV_PATTERNS=(
    "prospects_${SANITIZED_QUERY}_${RADIUS}km_*.csv"
    "prospects_${SANITIZED_QUERY}_$(echo $RADIUS | cut -d'.' -f1)km_*.csv"
    "prospects_${SANITIZED_QUERY}_*.csv"
    "prospects_*.csv"
)

LATEST_CSV=""
for pattern in "${CSV_PATTERNS[@]}"; do
    FOUND_CSV=$(ls -t $pattern 2>/dev/null | head -n1)
    if [ -n "$FOUND_CSV" ]; then
        LATEST_CSV="$FOUND_CSV"
        break
    fi
done

if [ -z "$LATEST_CSV" ]; then
    error "No se encontró archivo CSV generado."
    log "Patrones buscados:"
    for pattern in "${CSV_PATTERNS[@]}"; do
        log "  - $pattern"
    done
    log "Archivos CSV disponibles:"
    ls -la prospects_*.csv 2>/dev/null || echo "  (ninguno)"
    exit 1
fi

success "Archivo CSV encontrado: $LATEST_CSV"

# Mostrar estadísticas del CSV inicial
TOTAL_PLACES=$(tail -n +2 "$LATEST_CSV" | wc -l | tr -d ' ')
log "📈 Total de lugares encontrados: $TOTAL_PLACES"

if [ "$TOTAL_PLACES" -eq 0 ]; then
    warning "No se encontraron lugares en el CSV. Terminando pipeline."
    exit 0
fi

# Paso 2: Ejecutar phone_scraper para extraer teléfonos
log "📞 Paso 2: Extrayendo números de teléfono de los lugares encontrados..."
./phone_scraper csv --file "$LATEST_CSV"

if [ $? -ne 0 ]; then
    error "Error ejecutando phone_scraper"
    exit 1
fi

# Verificar archivo de salida con teléfonos
OUTPUT_CSV="${LATEST_CSV%%.csv}_with_phones.csv"
if [ -f "$OUTPUT_CSV" ]; then
    success "Pipeline completado exitosamente!"
    success "Archivo final: $OUTPUT_CSV"
    
    # Mostrar estadísticas finales
    TOTAL_WITH_PHONES=$(tail -n +2 "$OUTPUT_CSV" | wc -l | tr -d ' ')
    PHONES_FOUND=$(tail -n +2 "$OUTPUT_CSV" | awk -F',' '{if($9!="") count++} END {print count+0}')
    
    log "📊 Estadísticas finales:"
    log "   Total lugares: $TOTAL_WITH_PHONES"
    log "   Teléfonos extraídos: $PHONES_FOUND"
    
    if [ "$PHONES_FOUND" -gt 0 ]; then
        PERCENTAGE=$(echo "scale=1; $PHONES_FOUND * 100 / $TOTAL_WITH_PHONES" | bc -l 2>/dev/null || echo "N/A")
        log "   Tasa de éxito: ${PERCENTAGE}%"
    fi
    
    log "📂 Archivos generados:"
    log "   - Lugares iniciales: $LATEST_CSV"
    log "   - Con teléfonos: $OUTPUT_CSV"
    
else
    error "No se generó el archivo de salida con teléfonos"
    exit 1
fi

log "✨ Pipeline completado!"