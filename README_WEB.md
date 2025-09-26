# Maps Scraper - Interfaz Web

## 🌟 Descripción

Sistema completo para scraping de Google Maps con una interfaz web elegante y minimalista. Extrae información de negocios incluyendo nombres, direcciones, calificaciones, reseñas, horarios, sitios web y números de teléfono.

## 🚀 Características

- **Interfaz Web Moderna**: Diseño minimalista y elegante con Tailwind CSS
- **Pipeline Automatizado**: Ejecuta el scraping principal y extracción de teléfonos en secuencia
- **Terminal en Tiempo Real**: Visualización del progreso en una terminal simulada
- **Descarga Automática**: Los resultados se descargan automáticamente al completarse
- **Configuración Flexible**: Ajusta ubicación, palabra clave, radio y opción de teléfonos
- **Responsive**: Funciona en desktop y móvil

## 📋 Requisitos

- Go 1.19 o superior
- Navegador web moderno
- Conexión a internet

## ⚡ Inicio Rápido

### 1. Instalar dependencias

```bash
go mod tidy
```

### 2. Compilar binarios

```bash
make build-all
```

### 3. Iniciar servidor web

```bash
make web
# O directamente:
./start_web.sh
```

### 4. Usar la interfaz web

1. Abre tu navegador en `http://localhost:8080`
2. Configura los parámetros de búsqueda:
   - **Latitud/Longitud**: Coordenadas del centro de búsqueda
   - **Palabra clave**: Término a buscar (ej: "spa", "restaurant", "hotel")
   - **Radio**: Distancia en kilómetros (recomendado: 1-5 km)
   - **Incluir teléfonos**: Marca para extraer números de teléfono
3. Haz clic en "🚀 Ejecutar Pipeline"
4. Observa el progreso en tiempo real en la terminal
5. Descarga automática del archivo CSV al completarse

## 🛠️ Uso por Línea de Comandos

### Pipeline completo

```bash
./pipeline.sh <latitud> <longitud> "<palabra_clave>" <radio>

# Ejemplo:
./pipeline.sh 19.1019061 -98.2810447 "spa" 1.0
```

### Solo scraper principal

```bash
./mapsscrap-1 --lat 19.1019061 --lon -98.2810447 --query "spa" --radius 1.0
```

### Solo extracción de teléfonos

```bash
./phone_scraper csv --file prospects_spa_1km_2025-09-26_12-20-46.csv
```

## 📁 Estructura de Archivos

```
mapsscrap-main/
├── web/
│   └── index.html          # Interfaz web principal
├── main.go                 # Scraper principal de Google Maps
├── enhanced_phone_scraper.go # Extractor de teléfonos
├── web_server.go          # Servidor web backend
├── pipeline.sh            # Script de pipeline automatizado
├── start_web.sh           # Script de inicio del servidor web
├── Makefile              # Comandos de construcción
├── go.mod               # Dependencias de Go
└── README_WEB.md        # Este archivo
```

## 🔧 API Endpoints

### POST /api/execute
Ejecuta el pipeline de scraping

**Request Body:**
```json
{
    "latitude": 19.1019061,
    "longitude": -98.2810447,
    "keyword": "spa",
    "radius": 1.0,
    "includePhone": true
}
```

**Response:**
```json
{
    "success": true,
    "message": "Pipeline ejecutado exitosamente",
    "fileName": "prospects_spa_1km_2025-09-26_12-20-46_with_phones.csv",
    "placeCount": 118,
    "phoneCount": 82
}
```

### GET /api/download/{filename}
Descarga el archivo CSV generado

## 📊 Formato de Salida CSV

Los archivos CSV incluyen las siguientes columnas:

| Columna | Descripción |
|---------|-------------|
| Name | Nombre del negocio |
| Address | Dirección completa |
| Stars | Calificación (1-5 estrellas) |
| Reviews | Número de reseñas |
| Phone | Teléfono original (si disponible) |
| Hours | Horarios de atención |
| Website | Sitio web |
| GoogleURL | URL de Google Maps |
| ScrapedPhone | Teléfono extraído (si se solicita) |

## 🎨 Personalización

### Modificar la Interfaz Web

Edita `web/index.html` para personalizar:
- Colores y estilos (usando Tailwind CSS)
- Textos y mensajes
- Animaciones y efectos
- Campos del formulario

### Ajustar el Pipeline

Modifica `pipeline.sh` para:
- Cambiar parámetros por defecto
- Agregar validaciones adicionales
- Personalizar mensajes de log

### Configurar el Servidor

Edita `web_server.go` para:
- Cambiar el puerto (por defecto: 8080)
- Agregar autenticación
- Modificar CORS policies
- Añadir logging adicional

## 🐛 Troubleshooting

### El servidor no inicia
```bash
# Verificar que Go está instalado
go version

# Instalar dependencias
go mod tidy

# Compilar manualmente
go build -o web_server web_server.go
```

### Error "Go no está instalado"
Instala Go desde: https://golang.org/download/

### Los binarios no se ejecutan
```bash
# Recompilar todos los binarios
make clean
make build-all
```

### La interfaz web no se ve correctamente
- Verifica que el directorio `web/` existe
- Asegúrate de que `index.html` está en `web/index.html`
- Comprueba la consola del navegador para errores de JavaScript

## 📈 Rendimiento

### Recomendaciones

- **Radio pequeño**: Usa 1-5 km para mejores resultados
- **Conexión estable**: Asegúrate de tener buena conexión a internet
- **Memoria suficiente**: Para extraer muchos lugares
- **Paciencia**: La extracción de teléfonos puede tomar tiempo

### Limitaciones

- Google Maps puede bloquear requests excesivos
- Los resultados varían según la disponibilidad de datos
- La extracción de teléfonos es más lenta que el scraping básico

## 🔒 Consideraciones Legales

- Respeta los términos de servicio de Google Maps
- Usa los datos de manera ética y legal
- No hagas scraping excesivo para evitar bloqueos
- Considera implementar delays entre requests

## 🆘 Soporte

Para problemas o mejoras:
1. Revisa los logs en la terminal del servidor
2. Verifica la consola del navegador para errores de JavaScript
3. Comprueba que todos los archivos estén en sus ubicaciones correctas

## 📝 Licencia

[Especificar licencia aquí]