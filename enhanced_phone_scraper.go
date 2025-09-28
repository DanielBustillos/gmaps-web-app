package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

const (
	// Aumenté los workers para paralelizar más (ajusta según recursos disponibles)
	maxPhoneWorkers = 8 // Número máximo de workers concurrentes para teléfonos
	// Reducir timeout por lugar para evitar esperar demasiado en lugares problemáticos
	phoneTimeout = 15 * time.Second
)

// PhoneScraper estructura para el scraper de teléfonos de Google Maps
type PhoneScraper struct {
	browser *rod.Browser
}

// PlaceWithPhone representa un lugar con su información de teléfono
type PlaceWithPhone struct {
	Name      string
	Address   string
	Stars     string
	Reviews   string
	Phone     string
	Hours     string
	Website   string
	GoogleURL string
	ScrapedPhone string // Nuevo campo para el teléfono extraído
}

// NewPhoneScraper crea una nueva instancia del scraper de teléfonos
func NewPhoneScraper() (*PhoneScraper, error) {
	// Configurar el navegador para usar Google Chrome preinstalado
	chromePath := "/usr/bin/google-chrome"
	log.Printf("Verificando la existencia de Google Chrome en: %s", chromePath)
	if _, err := os.Stat(chromePath); os.IsNotExist(err) {
		log.Printf("❌ Google Chrome no encontrado en %s", chromePath)
		return nil, fmt.Errorf("Google Chrome no está instalado en el entorno")
	}

	log.Printf("✅ Google Chrome encontrado en %s", chromePath)
	browser := rod.New().ControlURL(launcher.New().Bin(chromePath).MustLaunch()).MustConnect()
	
	return &PhoneScraper{
		browser: browser,
	}, nil
}

// Close cierra el navegador
func (ps *PhoneScraper) Close() {
	if ps.browser != nil {
		ps.browser.Close()
	}
}

// ExtractPhoneFromGoogleMapsURL extrae el teléfono de una URL específica de Google Maps
func (ps *PhoneScraper) ExtractPhoneFromGoogleMapsURL(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), phoneTimeout)
	defer cancel()

	page := ps.browser.MustPage()
	defer page.Close()

	// Navegar a la URL de Google Maps
	if err := page.Navigate(url); err != nil {
		return "", fmt.Errorf("failed to navigate to URL: %w", err)
	}

	// Esperar a que la página se cargue completamente
	page.MustWaitStable()
	time.Sleep(2 * time.Second)

	// Buscar el teléfono usando múltiples estrategias
	phone, err := ps.findPhoneWithTimeout(page, ctx)
	if err != nil {
		return "", err
	}

	return phone, nil
}

// findPhoneWithTimeout busca el teléfono con timeout
func (ps *PhoneScraper) findPhoneWithTimeout(page *rod.Page, ctx context.Context) (string, error) {
	done := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		phone := ps.findPhoneNumber(page)
		if phone != "" {
			done <- phone
		} else {
			errChan <- fmt.Errorf("no phone found")
		}
	}()

	select {
	case phone := <-done:
		return phone, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("timeout finding phone")
	}
}

// findPhoneNumber busca el número de teléfono usando múltiples estrategias
func (ps *PhoneScraper) findPhoneNumber(page *rod.Page) string {
	// Estrategia 1: Buscar por botón con atributo data-item-id que contiene "phone"
	if phoneElements, err := page.Elements("button[data-item-id*='phone']"); err == nil && len(phoneElements) > 0 {
		for _, element := range phoneElements {
			if dataItemId, err := element.Attribute("data-item-id"); err == nil && dataItemId != nil {
				if strings.Contains(*dataItemId, "phone:tel:") {
					phoneFromAttr := strings.Replace(*dataItemId, "phone:tel:", "", 1)
					return ps.formatPhone(phoneFromAttr)
				}
			}
			
			// También buscar en el texto del botón
			if text, err := element.Text(); err == nil {
				if extractedPhone := ps.extractPhoneFromText(text); extractedPhone != "" {
					return extractedPhone
				}
			}
		}
	}

	// Estrategia 2: Buscar por aria-label que contenga "Teléfono"
	if phoneElements, err := page.Elements("button[aria-label*='Teléfono']"); err == nil && len(phoneElements) > 0 {
		for _, element := range phoneElements {
			if ariaLabel, err := element.Attribute("aria-label"); err == nil && ariaLabel != nil {
				if extractedPhone := ps.extractPhoneFromText(*ariaLabel); extractedPhone != "" {
					return extractedPhone
				}
			}
		}
	}

	// Estrategia 3: Buscar por clase CSS específica del número
	if phoneElements, err := page.Elements(".Io6YTe.fontBodyMedium.kR99db.fdkmkc"); err == nil && len(phoneElements) > 0 {
		for _, element := range phoneElements {
			if text, err := element.Text(); err == nil {
				text = strings.TrimSpace(text)
				if ps.isPhoneNumber(text) {
					return text
				}
			}
		}
	}

	return ""
}

// extractPhoneFromText extrae números de teléfono de un texto
func (ps *PhoneScraper) extractPhoneFromText(text string) string {
	// Patrón regex para números de teléfono mexicanos con +52
	mexicanPhoneRegex := regexp.MustCompile(`\+?52\s?\d{3}\s?\d{3}\s?\d{4}`)
	if match := mexicanPhoneRegex.FindString(text); match != "" {
		return ps.formatPhone(strings.TrimSpace(match))
	}

	// Patrón para números de teléfono de 10 dígitos (formato mexicano sin código de país)
	tenDigitRegex := regexp.MustCompile(`\d{3}\s?\d{3}\s?\d{4}`)
	if match := tenDigitRegex.FindString(text); match != "" {
		return strings.TrimSpace(match)
	}

	return ""
}

// isPhoneNumber verifica si un texto parece ser un número de teléfono
func (ps *PhoneScraper) isPhoneNumber(text string) bool {
	cleaned := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(text, "")
	phonePattern := regexp.MustCompile(`^\+?\d{10,15}$`)
	return phonePattern.MatchString(cleaned)
}

// formatPhone formatea el número de teléfono
func (ps *PhoneScraper) formatPhone(phone string) string {
	cleaned := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(phone, "")
	
	if strings.HasPrefix(cleaned, "+52") {
		cleaned = strings.TrimPrefix(cleaned, "+52")
		if len(cleaned) == 10 {
			return fmt.Sprintf("%s %s %s", cleaned[:3], cleaned[3:6], cleaned[6:])
		}
	}
	
	if len(cleaned) == 10 && !strings.HasPrefix(cleaned, "+") {
		return fmt.Sprintf("%s %s %s", cleaned[:3], cleaned[3:6], cleaned[6:])
	}
	
	return phone
}

// ProcessCSV procesa un archivo CSV y extrae teléfonos para cada lugar
func ProcessCSV(csvPath string) error {
	// Leer el archivo CSV
	places, err := readCSV(csvPath)
	if err != nil {
		return fmt.Errorf("error reading CSV: %w", err)
	}

	if len(places) == 0 {
		return fmt.Errorf("no places found in CSV")
	}

	fmt.Printf("Procesando %d lugares para extraer teléfonos...\n", len(places))

	// Crear scraper
	scraper, err := NewPhoneScraper()
	if err != nil {
		return fmt.Errorf("error creating phone scraper: %w", err)
	}
	defer scraper.Close()

	// Procesar lugares con workers concurrentes
	updatedPlaces := processPlacesWithPhones(scraper, places)

	// Guardar CSV actualizado
	outputPath := strings.Replace(csvPath, ".csv", "_with_phones.csv", 1)
	if err := saveCSVWithPhones(updatedPlaces, outputPath); err != nil {
		return fmt.Errorf("error saving updated CSV: %w", err)
	}

	fmt.Printf("✅ Archivo actualizado guardado: %s\n", outputPath)
	
	// Mostrar estadísticas
	phonesFound := 0
	for _, place := range updatedPlaces {
		if place.ScrapedPhone != "" {
			phonesFound++
		}
	}
	
	fmt.Printf("📊 Estadísticas:\n")
	fmt.Printf("   Total lugares: %d\n", len(updatedPlaces))
	fmt.Printf("   Teléfonos encontrados: %d (%.1f%%)\n", phonesFound, float64(phonesFound)/float64(len(updatedPlaces))*100)

	return nil
}

// readCSV lee un archivo CSV y devuelve una lista de lugares
func readCSV(csvPath string) ([]PlaceWithPhone, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have at least header and one data row")
	}

	var places []PlaceWithPhone
	for i, record := range records[1:] { // Skip header
		if len(record) < 8 {
			log.Printf("Warning: Row %d has insufficient columns, skipping", i+2)
			continue
		}
		
		place := PlaceWithPhone{
			Name:      record[0],
			Address:   record[1],
			Stars:     record[2],
			Reviews:   record[3],
			Phone:     record[4],
			Hours:     record[5],
			Website:   record[6],
			GoogleURL: record[7],
		}
		places = append(places, place)
	}

	return places, nil
}

// processPlacesWithPhones procesa los lugares para extraer teléfonos usando workers
func processPlacesWithPhones(scraper *PhoneScraper, places []PlaceWithPhone) []PlaceWithPhone {
	var mu sync.Mutex
	updatedPlaces := make([]PlaceWithPhone, len(places))
	copy(updatedPlaces, places)

	// Canal de trabajos
	jobs := make(chan int, len(places))

	// Barra de progreso
	bar := progressbar.NewOptions(len(places),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription("[cyan]🔍 Extrayendo teléfonos...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]█[reset]",
			SaucerPadding: "[white]░[reset]",
			BarStart:      "[white]|[reset]",
			BarEnd:        "[white]|[reset]",
		}),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionFullWidth(),
	)

	// Llenar trabajos
	for i := range updatedPlaces {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup

	// Worker pool: cada worker crea UNA pestaña y la reutiliza para muchos lugares
	for w := 0; w < maxPhoneWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Crear y reutilizar una página por worker para evitar overhead de crear/cierrar pestañas por lugar
			page := scraper.browser.MustPage()
			defer page.Close()

			for idx := range jobs {
				place := &updatedPlaces[idx]

				start := time.Now()
				// Solo procesar si no hay teléfono y existe GoogleURL
				if place.Phone == "" && place.GoogleURL != "" {
					// navegar y extraer usando la página reutilizable
					// usar un contexto con timeout para evitar bloqueos largos
					ctx, cancel := context.WithTimeout(context.Background(), phoneTimeout)
					// Navegar
					if err := page.Navigate(place.GoogleURL); err != nil {
						log.Printf("[worker %d] Error navegando a %s: %v", workerID, place.GoogleURL, err)
						cancel()
					} else {
						page.MustWaitStable()
						// Pequeña espera para permitir que contenido dinámico aparezca
						time.Sleep(750 * time.Millisecond)

						phone, err := scraper.findPhoneWithTimeout(page, ctx)
						if err == nil && phone != "" {
							mu.Lock()
							place.ScrapedPhone = phone
							mu.Unlock()
						} else if err != nil {
							log.Printf("[worker %d] No phone for %s: %v", workerID, place.GoogleURL, err)
						}
						cancel()
						// Rate limiting ligero entre requests
						time.Sleep(300 * time.Millisecond)
					}
				}

				elapsed := time.Since(start)
				log.Printf("[worker %d] Procesado index=%d name=%q elapsed=%s", workerID, idx, place.Name, elapsed)

				mu.Lock()
				bar.Add(1)
				mu.Unlock()
			}
		}(w)
	}

	wg.Wait()
	bar.Finish()
	fmt.Println()

	return updatedPlaces
}

// saveCSVWithPhones guarda los lugares con teléfonos en un archivo CSV
func saveCSVWithPhones(places []PlaceWithPhone, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write UTF-8 BOM so Excel recognizes accents correctly
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		file.Close()
		return err
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Escribir header con nueva columna
	header := []string{"Name", "Address", "Stars", "Reviews", "Phone", "Hours", "Website", "GoogleURL", "ScrapedPhone"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Escribir datos
	for _, place := range places {
		record := []string{
			place.Name,
			place.Address,
			place.Stars,
			place.Reviews,
			place.Phone,
			place.Hours,
			place.Website,
			place.GoogleURL,
			place.ScrapedPhone,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// Comandos CLI
var (
	csvFile string
	singleURL string
)

var rootCmd = &cobra.Command{
	Use:   "phone_scraper",
	Short: "Extrae números de teléfono de Google Maps",
	Long:  "Extrae números de teléfono de URLs de Google Maps o procesa archivos CSV",
}

var csvCmd = &cobra.Command{
	Use:   "csv",
	Short: "Procesa un archivo CSV para extraer teléfonos",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ProcessCSV(csvFile)
	},
}

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Extrae teléfono de una URL específica",
	RunE: func(cmd *cobra.Command, args []string) error {
		scraper, err := NewPhoneScraper()
		if err != nil {
			return err
		}
		defer scraper.Close()

		phone, err := scraper.ExtractPhoneFromGoogleMapsURL(singleURL)
		if err != nil {
			log.Printf("Error: %v", err)
			return err
		}

		if phone != "" {
			fmt.Printf("✅ Teléfono encontrado: %s\n", phone)
		} else {
			fmt.Println("❌ No se encontró teléfono")
		}
		return nil
	},
}

func init() {
	csvCmd.Flags().StringVarP(&csvFile, "file", "f", "", "Archivo CSV a procesar")
	csvCmd.MarkFlagRequired("file")

	urlCmd.Flags().StringVarP(&singleURL, "url", "u", "", "URL de Google Maps")
	urlCmd.MarkFlagRequired("url")

	rootCmd.AddCommand(csvCmd)
	rootCmd.AddCommand(urlCmd)
}

func main() {
	// Si no hay argumentos de comando, mantener compatibilidad con versión anterior
	if len(os.Args) == 2 && !strings.HasPrefix(os.Args[1], "-") {
		scraper, err := NewPhoneScraper()
		if err != nil {
			log.Fatalf("Error creating scraper: %v", err)
		}
		defer scraper.Close()

		url := os.Args[1]
		fmt.Printf("Extrayendo teléfono de: %s\n", url)
		
		phone, err := scraper.ExtractPhoneFromGoogleMapsURL(url)
		if err != nil {
			log.Printf("Error: %v", err)
			return
		}
		
		if phone != "" {
			fmt.Printf("✅ Teléfono encontrado: %s\n", phone)
		} else {
			fmt.Println("❌ No se encontró teléfono")
		}
		return
	}

	// Usar cobra para comandos CLI
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}