package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// PhoneScraper estructura para el scraper de teléfonos de Google Maps
type PhoneScraper struct {
	browser *rod.Browser
}

// NewPhoneScraper crea una nueva instancia del scraper de teléfonos
func NewPhoneScraper() (*PhoneScraper, error) {
	launch := launcher.New().
		Headless(true).
		Devtools(false)

	url, err := launch.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(url).MustConnect()
	
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		return "", fmt.Errorf("error finding phone: %w", err)
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
			errChan <- fmt.Errorf("no se encontró número de teléfono")
		}
	}()

	select {
	case phone := <-done:
		return phone, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("timeout buscando teléfono")
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

	// Estrategia 4: Buscar en cualquier elemento que contenga un número de teléfono
	if allElements, err := page.Elements("*"); err == nil {
		for _, element := range allElements {
			if text, err := element.Text(); err == nil && text != "" {
				if extractedPhone := ps.extractPhoneFromText(text); extractedPhone != "" {
					return extractedPhone
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

	// Patrón más general para cualquier secuencia de números que parezca teléfono
	generalPhoneRegex := regexp.MustCompile(`\d{3}[\s\-]?\d{3}[\s\-]?\d{4}`)
	if match := generalPhoneRegex.FindString(text); match != "" {
		return strings.TrimSpace(match)
	}

	return ""
}

// isPhoneNumber verifica si un texto parece ser un número de teléfono
func (ps *PhoneScraper) isPhoneNumber(text string) bool {
	// Eliminar espacios y caracteres especiales comunes en números de teléfono
	cleaned := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(text, "")
	
	// Verificar si contiene solo números y posiblemente un +
	phonePattern := regexp.MustCompile(`^\+?\d{10,15}$`)
	return phonePattern.MatchString(cleaned)
}

// formatPhone formatea el número de teléfono
func (ps *PhoneScraper) formatPhone(phone string) string {
	// Limpiar el teléfono de espacios y caracteres especiales
	cleaned := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(phone, "")
	
	// Si empieza con +52, es un número mexicano
	if strings.HasPrefix(cleaned, "+52") {
		cleaned = strings.TrimPrefix(cleaned, "+52")
		if len(cleaned) == 10 {
			return fmt.Sprintf("%s %s %s", cleaned[:3], cleaned[3:6], cleaned[6:])
		}
	}
	
	// Si es un número de 10 dígitos sin código de país
	if len(cleaned) == 10 && !strings.HasPrefix(cleaned, "+") {
		return fmt.Sprintf("%s %s %s", cleaned[:3], cleaned[3:6], cleaned[6:])
	}
	
	return phone
}

// main función principal para probar el scraper de teléfonos
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Uso: go run phone_scraper.go <URL_de_Google_Maps>")
	}

	url := os.Args[1]
	
	scraper, err := NewPhoneScraper()
	if err != nil {
		log.Fatalf("Error creando scraper: %v", err)
	}
	defer scraper.Close()

	fmt.Printf("Extrayendo teléfono de: %s\n", url)
	
	phone, err := scraper.ExtractPhoneFromGoogleMapsURL(url)
	if err != nil {
		log.Printf("Error obteniendo teléfono: %v", err)
		return
	}
	
	if phone != "" {
		fmt.Printf("✅ Teléfono encontrado: %s\n", phone)
	} else {
		fmt.Println("❌ No se encontró número de teléfono")
	}
}
