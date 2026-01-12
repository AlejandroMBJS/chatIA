package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// BrowserScraper scraper basado en browser headless (rod/chromium)
type BrowserScraper struct {
	config   *ScraperConfig
	browser  *rod.Browser
	launcher *launcher.Launcher
	mutex    sync.Mutex
	isReady  bool
}

// NewBrowserScraper crea un nuevo browser scraper
func NewBrowserScraper(config *ScraperConfig) (*BrowserScraper, error) {
	if config == nil {
		config = DefaultScraperConfig()
	}

	bs := &BrowserScraper{
		config: config,
	}

	if err := bs.init(); err != nil {
		return nil, err
	}

	return bs, nil
}

// init inicializa el browser
func (bs *BrowserScraper) init() error {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()

	if bs.isReady {
		return nil
	}

	// Configurar launcher
	bs.launcher = launcher.New().
		Headless(bs.config.BrowserHeadless).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage").
		Set("disable-setuid-sandbox").
		Set("disable-extensions").
		Set("disable-background-networking").
		Set("disable-sync").
		Set("disable-translate").
		Set("disable-default-apps").
		Set("mute-audio")

	// Lanzar browser
	u, err := bs.launcher.Launch()
	if err != nil {
		return fmt.Errorf("error lanzando browser: %w", err)
	}

	// Conectar a browser
	bs.browser = rod.New().ControlURL(u)
	if err := bs.browser.Connect(); err != nil {
		return fmt.Errorf("error conectando a browser: %w", err)
	}

	// Configurar timeout por defecto
	bs.browser = bs.browser.Timeout(bs.config.BrowserTimeout)

	bs.isReady = true
	log.Printf("[INFO] Browser scraper inicializado (headless: %v)", bs.config.BrowserHeadless)

	return nil
}

// Scrape extrae contenido usando browser headless
func (bs *BrowserScraper) Scrape(ctx context.Context, targetURL string) (*ScrapedContent, error) {
	bs.mutex.Lock()
	if !bs.isReady {
		if err := bs.init(); err != nil {
			bs.mutex.Unlock()
			return &ScrapedContent{
				URL:       targetURL,
				Success:   false,
				Error:     fmt.Sprintf("browser no disponible: %v", err),
				ScrapedAt: time.Now(),
				Method:    "browser",
			}, err
		}
	}
	bs.mutex.Unlock()

	// Crear página con timeout del contexto
	page, err := bs.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return &ScrapedContent{
			URL:       targetURL,
			Success:   false,
			Error:     fmt.Sprintf("error creando página: %v", err),
			ScrapedAt: time.Now(),
			Method:    "browser",
		}, err
	}
	defer page.Close()

	// Configurar user agent
	page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: bs.config.UserAgent,
	})

	// Navegar a la URL
	err = page.Navigate(targetURL)
	if err != nil {
		return &ScrapedContent{
			URL:       targetURL,
			Success:   false,
			Error:     fmt.Sprintf("error navegando: %v", err),
			ScrapedAt: time.Now(),
			Method:    "browser",
		}, err
	}

	// Esperar a que cargue
	err = page.WaitLoad()
	if err != nil {
		// Continuar aunque falle WaitLoad
		log.Printf("[WARN] WaitLoad falló para %s: %v", targetURL, err)
	}

	// Esperar selector específico si está configurado
	if bs.config.WaitForSelector != "" {
		waitCtx, cancel := context.WithTimeout(ctx, bs.config.WaitTimeout)
		defer cancel()

		page = page.Context(waitCtx)
		_, err := page.Element(bs.config.WaitForSelector)
		if err != nil {
			log.Printf("[WARN] Selector '%s' no encontrado en %s: %v", bs.config.WaitForSelector, targetURL, err)
		}
	}

	// Esperar un poco más para JS dinámico
	time.Sleep(500 * time.Millisecond)

	// Extraer título
	title := ""
	titleEl, err := page.Element("title")
	if err == nil && titleEl != nil {
		title, _ = titleEl.Text()
	}

	// Extraer contenido de texto del body
	content := ""
	bodyEl, err := page.Element("body")
	if err == nil && bodyEl != nil {
		content, _ = bodyEl.Text()
	}

	// Limpiar contenido
	content = bs.cleanContent(content)

	// Calcular hash
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:8])

	return &ScrapedContent{
		URL:         targetURL,
		Title:       strings.TrimSpace(title),
		Content:     content,
		ContentHash: hashStr,
		ScrapedAt:   time.Now(),
		Method:      "browser",
		Success:     true,
	}, nil
}

// ScrapeWithScreenshot extrae contenido y toma captura de pantalla
func (bs *BrowserScraper) ScrapeWithScreenshot(ctx context.Context, targetURL string) (*ScrapedContent, []byte, error) {
	content, err := bs.Scrape(ctx, targetURL)
	if err != nil || !content.Success {
		return content, nil, err
	}

	// Tomar screenshot
	page, err := bs.browser.Page(proto.TargetCreateTarget{URL: targetURL})
	if err != nil {
		return content, nil, nil // Devolver contenido sin screenshot
	}
	defer page.Close()

	page.Navigate(targetURL)
	page.WaitLoad()

	screenshot, err := page.Screenshot(true, nil)
	if err != nil {
		return content, nil, nil
	}

	return content, screenshot, nil
}

// cleanContent limpia el contenido extraído
func (bs *BrowserScraper) cleanContent(text string) string {
	// Remover múltiples espacios en blanco
	lines := strings.Split(text, "\n")
	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	result := strings.Join(cleaned, "\n")

	// Limitar a tamaño máximo razonable
	maxSize := 500000 // 500KB de texto
	if len(result) > maxSize {
		result = result[:maxSize] + "\n\n[... contenido truncado ...]"
	}

	return result
}

// IsAvailable verifica si el browser está disponible
func (bs *BrowserScraper) IsAvailable() bool {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()
	return bs.isReady && bs.browser != nil
}

// Close cierra el browser y libera recursos
func (bs *BrowserScraper) Close() error {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()

	if bs.browser != nil {
		err := bs.browser.Close()
		bs.browser = nil
		bs.isReady = false
		if bs.launcher != nil {
			bs.launcher.Cleanup()
			bs.launcher = nil
		}
		return err
	}
	return nil
}

// Restart reinicia el browser
func (bs *BrowserScraper) Restart() error {
	bs.Close()
	return bs.init()
}
