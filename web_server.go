package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type PipelineRequest struct {
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Keyword      string  `json:"keyword"`
	Radius       float64 `json:"radius"`
	IncludePhone bool    `json:"includePhone"`
}

type PipelineResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	FileName   string `json:"fileName,omitempty"`
	FilePath   string `json:"filePath,omitempty"`
	PlaceCount int    `json:"placeCount,omitempty"`
	PhoneCount int    `json:"phoneCount,omitempty"`
}

type ProgressMessage struct {
	Type       string `json:"type"`       // "progress", "log", "complete", "error"
	Message    string `json:"message"`
	Percentage int    `json:"percentage,omitempty"`
	Current    int    `json:"current,omitempty"`
	Total      int    `json:"total,omitempty"`
	Stage      string `json:"stage,omitempty"` // "scraping", "phones"
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan ProgressMessage)

func init() {
	// Manejar mensajes de broadcast
	go handleMessages()
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Permitir conexiones desde cualquier origen (solo para desarrollo)
	},
}

func main() {
	r := mux.NewRouter()

	// Servir archivos estáticos
	r.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("./web/"))))
	
	// API endpoints
	r.HandleFunc("/api/execute", handleExecutePipeline).Methods("POST")
	r.HandleFunc("/api/download/{filename}", handleDownloadFile).Methods("GET")
	r.HandleFunc("/api/files", handleListFiles).Methods("GET")
	r.HandleFunc("/api/ws", handleWebSocket)
	
	// Redirigir root a la interfaz web
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/", http.StatusFound)
	})

	fmt.Println("🚀 Servidor web iniciado en http://localhost:8080")
	fmt.Println("📊 Interfaz web disponible en http://localhost:8080/web/")
	
	log.Fatal(http.ListenAndServe(":8080", r))
}

func handleExecutePipeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req PipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		response := PipelineResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Received request: %+v", req)

	// Validar parámetros
	if req.Latitude == 0 || req.Longitude == 0 || req.Keyword == "" || req.Radius <= 0 {
		log.Printf("Invalid parameters: lat=%f, lon=%f, keyword=%s, radius=%f", req.Latitude, req.Longitude, req.Keyword, req.Radius)
		response := PipelineResponse{
			Success: false,
			Message: fmt.Sprintf("Missing or invalid parameters: lat=%f, lon=%f, keyword=%s, radius=%f", req.Latitude, req.Longitude, req.Keyword, req.Radius),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Ejecutar pipeline
	response := executePipeline(req)
	
	// Al finalizar el pipeline, enviar respuesta minimalista sin estadísticas
	responseMinimal := struct {
		FileName string `json:"fileName"`
	}{
		FileName: response.FileName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseMinimal)
}

func executePipeline(req PipelineRequest) PipelineResponse {
	log.Printf("🚀 Iniciando pipeline de scraping con parámetros: %+v", req)
	
	// Convertir parámetros a strings
	latStr := strconv.FormatFloat(req.Latitude, 'f', -1, 64)
	lonStr := strconv.FormatFloat(req.Longitude, 'f', -1, 64)
	radiusStr := strconv.FormatFloat(req.Radius, 'f', -1, 64)

	log.Printf("📋 Preparando comando con: latitud=%s, longitud=%s, palabra=%s, radio=%s km", latStr, lonStr, req.Keyword, radiusStr)

	var cmd *exec.Cmd
	if req.IncludePhone {
		log.Printf("📞 Pipeline completo: scraping + extracción de teléfonos")
		cmd = exec.Command("./pipeline.sh", latStr, lonStr, req.Keyword, radiusStr)
	} else {
		log.Printf("📊 Pipeline básico: solo scraping de lugares")
		cmd = exec.Command("./mapsscrap-1", "--lat", latStr, "--lon", lonStr, "--query", req.Keyword, "--radius", radiusStr)
	}

	// Agregar timeout de 10 minutos para pipelines con teléfonos, 5 para básico
	timeout := 5 * time.Minute
	if req.IncludePhone {
		timeout = 10 * time.Minute
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)

	log.Printf("⏰ Timeout configurado: %.0f minutos", timeout.Minutes())
	log.Printf("🔄 Ejecutando comando: %v", cmd.Args)
	log.Printf("⏳ Esto puede tomar varios minutos, especialmente si incluye teléfonos...")
	
	// Ejecutar con streaming de output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("❌ Error creando pipe: %v", err)
		return PipelineResponse{
			Success: false,
			Message: "Error interno del servidor",
		}
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("❌ Error creando stderr pipe: %v", err)
		return PipelineResponse{
			Success: false,
			Message: "Error interno del servidor",
		}
	}

	if err := cmd.Start(); err != nil {
		log.Printf("❌ Error iniciando comando: %v", err)
		return PipelineResponse{
			Success: false,
			Message: fmt.Sprintf("Error iniciando pipeline: %v", err),
		}
	}

	log.Printf("✅ Comando iniciado exitosamente, PID: %d", cmd.Process.Pid)

	// Leer output en tiempo real y parsear progreso
	go func() {
		scanner := bufio.NewScanner(stdout)
		progressRegex := regexp.MustCompile(`(\d+)%\s*\|([█\s]+)\|\s*\((\d+)/(\d+)`)
		phoneRegex := regexp.MustCompile(`🔄 Procesados:\s*(\d+)/(\d+)`)
		
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("📤 STDOUT: %s", line)
			
			// Detectar barra de progreso del scraper principal
			if matches := progressRegex.FindStringSubmatch(line); len(matches) > 0 {
				percentage, _ := strconv.Atoi(matches[1])
				current, _ := strconv.Atoi(matches[3])
				total, _ := strconv.Atoi(matches[4])
				log.Printf("� Progreso scraping: %d%% (%d/%d)", percentage, current, total)
			}
			
			// Detectar progreso de extracción de teléfonos
			if matches := phoneRegex.FindStringSubmatch(line); len(matches) > 0 {
				current, _ := strconv.Atoi(matches[1])
				total, _ := strconv.Atoi(matches[2])
				percentage := int(float64(current) / float64(total) * 100)
				
				// Crear barra visual
				barLength := 20
				filled := int(float64(percentage) / 100.0 * float64(barLength))
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled)
				
				log.Printf("📞 Extrayendo teléfonos... %d%% |%s| (%d/%d)", percentage, bar, current, total)
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		progressRegex := regexp.MustCompile(`(\d+)%\s*\|([█\s]+)\|\s*\((\d+)/(\d+)`)
		
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("📤 STDERR: %s", line)
			
			// También verificar stderr para barras de progreso
			if matches := progressRegex.FindStringSubmatch(line); len(matches) > 0 {
				percentage, _ := strconv.Atoi(matches[1])
				current, _ := strconv.Atoi(matches[3])
				total, _ := strconv.Atoi(matches[4])
				log.Printf("📊 Progreso: %d%% (%d/%d)", percentage, current, total)
			}
		}
	}()

	// Mostrar progreso cada 30 segundos
	progressTicker := time.NewTicker(30 * time.Second)
	defer progressTicker.Stop()
	
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	startTime := time.Now()
	
	for {
		select {
		case <-progressTicker.C:
			elapsed := time.Since(startTime)
			log.Printf("⏱️  Pipeline ejecutándose... Tiempo transcurrido: %.1f minutos", elapsed.Minutes())
			if req.IncludePhone {
				log.Printf("📞 Procesando teléfonos - esto puede tomar tiempo adicional...")
			}
			
		case err := <-done:
			elapsed := time.Since(startTime)
			log.Printf("🏁 Comando terminado después de %.1f minutos", elapsed.Minutes())
			
			if err != nil {
				log.Printf("❌ Error ejecutando pipeline: %v", err)
				
				// Si es timeout, devolver error específico
				if ctx.Err() == context.DeadlineExceeded {
					log.Printf("⏰ Pipeline cancelado por timeout (%.0f minutos)", timeout.Minutes())
					return PipelineResponse{
						Success: false,
						Message: fmt.Sprintf("El pipeline tardó más de %.0f minutos y fue cancelado. Prueba con un radio menor.", timeout.Minutes()),
					}
				}
				
				return PipelineResponse{
					Success: false,
					Message: fmt.Sprintf("Error en el pipeline: %v", err),
				}
			}

			log.Printf("✅ Pipeline completado exitosamente!")
			
			// Buscar archivo generado
			log.Printf("🔍 Buscando archivo CSV generado...")
			fileName, filePath := findLatestCSV(req.Keyword, req.Radius)
			
			if fileName == "" {
				log.Printf("❌ No se encontró archivo CSV")
				return PipelineResponse{
					Success: false,
					Message: "El pipeline se ejecutó pero no se generó el archivo CSV esperado",
				}
			}

			log.Printf("📄 Archivo encontrado: %s", fileName)
			
			// Analizar contenido
			log.Printf("📊 Analizando contenido del archivo...")
			placeCount, phoneCount := countPlacesInCSV(filePath, req.IncludePhone)
			log.Printf("📈 Análisis completado: %d lugares, %d teléfonos", placeCount, phoneCount)

			return PipelineResponse{
				Success:    true,
				Message:    "Pipeline ejecutado exitosamente",
				FileName:   fileName,
				FilePath:   filePath,
				PlaceCount: placeCount,
				PhoneCount: phoneCount,
			}
		}
	}
}

func findLatestCSV(keyword string, radius float64) (string, string) {
	// Crear patrones de búsqueda
	sanitizedKeyword := strings.ReplaceAll(keyword, " ", "_")
	
	patterns := []string{
		fmt.Sprintf("prospects_%s_%.1fkm_*.csv", sanitizedKeyword, radius),
		fmt.Sprintf("prospects_%s_%.0fkm_*.csv", sanitizedKeyword, radius),
		fmt.Sprintf("prospects_%s_*_with_phones.csv", sanitizedKeyword),
		fmt.Sprintf("prospects_%s_*.csv", sanitizedKeyword),
		"prospects_*_with_phones.csv",
		"prospects_*.csv",
	}
	
	log.Printf("Searching for CSV files with patterns: %v", patterns)
	
	var latestFile string
	var latestTime time.Time
	
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Error with pattern %s: %v", pattern, err)
			continue
		}
		
		log.Printf("Pattern %s found %d files: %v", pattern, len(matches), matches)
		
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				log.Printf("Error getting info for %s: %v", match, err)
				continue
			}
			
			log.Printf("File %s: modified at %v", match, info.ModTime())
			
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = match
				log.Printf("New latest file: %s", latestFile)
			}
		}
		
		if latestFile != "" {
			break
		}
	}
	
	if latestFile != "" {
		log.Printf("Selected file: %s (path: %s)", filepath.Base(latestFile), latestFile)
		return filepath.Base(latestFile), latestFile
	}
	
	log.Printf("No CSV file found")
	return "", ""
}

func countPlacesInCSV(filePath string, includePhone bool) (int, int) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(content), "\n")
	placeCount := len(lines) - 1 // Restar header
	
	if placeCount < 0 {
		placeCount = 0
	}

	phoneCount := 0
	if includePhone {
		// Contar líneas que tienen teléfono en la columna correspondiente
		for i, line := range lines[1:] { // Skip header
			if line == "" {
				continue
			}
			fields := strings.Split(line, ",")
			if len(fields) > 4 && fields[4] != "" && strings.Trim(fields[4], `"`) != "" {
				phoneCount++
			}
			// También verificar ScrapedPhone si existe
			if len(fields) > 8 && fields[8] != "" && strings.Trim(fields[8], `"`) != "" {
				phoneCount++
			}
			_ = i
		}
	}

	return placeCount, phoneCount
}

func handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["filename"]
	
	log.Printf("Download request for file: %s", filename)
	
	// Validar que el archivo existe y es seguro
	if !isValidFilename(filename) {
		log.Printf("Invalid filename: %s", filename)
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	
	filePath := filepath.Join(".", filename)
	log.Printf("Looking for file at path: %s", filePath)
	
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("File not found: %s", filePath)
		
		// Buscar archivos similares para debug
		matches, _ := filepath.Glob("prospects_*.csv")
		log.Printf("Available CSV files: %v", matches)
		
		http.Error(w, fmt.Sprintf("File not found: %s", filename), http.StatusNotFound)
		return
	}
	
	log.Printf("File found, serving: %s", filePath)
	
	// Establecer headers para descarga
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Servir el archivo
	http.ServeFile(w, r, filePath)
	log.Printf("File served successfully: %s", filename)
}

func isValidFilename(filename string) bool {
	// Verificar que el archivo es un CSV y tiene un patrón válido
	if !strings.HasSuffix(filename, ".csv") {
		return false
	}
	
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return false
	}
	
	return strings.HasPrefix(filename, "prospects_")
}

func handleListFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	matches, err := filepath.Glob("prospects_*.csv")
	if err != nil {
		http.Error(w, "Error listing files", http.StatusInternalServerError)
		return
	}
	
	type FileInfo struct {
		Name     string    `json:"name"`
		Size     int64     `json:"size"`
		Modified time.Time `json:"modified"`
	}
	
	var files []FileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:     filepath.Base(match),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}
	
	json.NewEncoder(w).Encode(files)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}
	defer conn.Close()

	// Registrar cliente
	clients[conn] = true
	log.Printf("Cliente WebSocket conectado. Total: %d", len(clients))

	// Mantener conexión activa
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Cliente WebSocket desconectado: %v", err)
			delete(clients, conn)
			break
		}
	}
}