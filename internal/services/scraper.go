package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// ScrapedContent representa el contenido extraído de una URL
type ScrapedContent struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentHash string    `json:"content_hash"`
	ScrapedAt   time.Time `json:"scraped_at"`
	Method      string    `json:"method"` // "http" o "browser"
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// ScraperConfig configuración del scraper
type ScraperConfig struct {
	// HTTP settings
	HTTPTimeout     time.Duration
	MaxRetries      int
	MaxContentSize  int64 // bytes
	UserAgent       string
	AllowedDomains  []string // vacío = todos permitidos
	BlockedDomains  []string

	// Rate limiting
	RequestsPerSecond float64
	BurstSize         int

	// Cache
	CacheTTL    time.Duration
	EnableCache bool

	// Browser settings (para rod)
	BrowserTimeout    time.Duration
	EnableBrowser     bool
	BrowserHeadless   bool
	WaitForSelector   string // selector CSS para esperar
	WaitTimeout       time.Duration
}

// DefaultScraperConfig configuración por defecto para producción
func DefaultScraperConfig() *ScraperConfig {
	return &ScraperConfig{
		HTTPTimeout:       30 * time.Second,
		MaxRetries:        3,
		MaxContentSize:    10 * 1024 * 1024, // 10MB
		UserAgent:         "GIAChat-Bot/1.0 (Internal Enterprise Assistant)",
		AllowedDomains:    []string{},
		BlockedDomains:    []string{},
		RequestsPerSecond: 2.0,
		BurstSize:         5,
		CacheTTL:          15 * time.Minute,
		EnableCache:       true,
		BrowserTimeout:    60 * time.Second,
		EnableBrowser:     true,
		BrowserHeadless:   true,
		WaitForSelector:   "body",
		WaitTimeout:       10 * time.Second,
	}
}

// Scraper servicio principal de scraping
type Scraper struct {
	config     *ScraperConfig
	httpClient *http.Client

	// Rate limiting por dominio
	rateLimiters map[string]*rateLimiter
	rlMutex      sync.RWMutex

	// Cache
	cache      map[string]*cacheEntry
	cacheMutex sync.RWMutex

	// Browser scraper (lazy init)
	browserScraper *BrowserScraper
	browserMutex   sync.Mutex
}

type rateLimiter struct {
	tokens     float64
	lastUpdate time.Time
	mutex      sync.Mutex
}

type cacheEntry struct {
	content   *ScrapedContent
	expiresAt time.Time
}

// NewScraper crea un nuevo servicio de scraping
func NewScraper(config *ScraperConfig) *Scraper {
	if config == nil {
		config = DefaultScraperConfig()
	}

	return &Scraper{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("demasiados redirects (max 10)")
				}
				return nil
			},
		},
		rateLimiters: make(map[string]*rateLimiter),
		cache:        make(map[string]*cacheEntry),
	}
}

// Scrape extrae contenido de una URL usando el método más apropiado
func (s *Scraper) Scrape(ctx context.Context, targetURL string) (*ScrapedContent, error) {
	// Validar URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return &ScrapedContent{
			URL:       targetURL,
			Success:   false,
			Error:     fmt.Sprintf("URL inválida: %v", err),
			ScrapedAt: time.Now(),
		}, err
	}

	// Verificar dominio permitido
	if err := s.checkDomain(parsedURL.Host); err != nil {
		return &ScrapedContent{
			URL:       targetURL,
			Success:   false,
			Error:     err.Error(),
			ScrapedAt: time.Now(),
		}, err
	}

	// Verificar cache
	if s.config.EnableCache {
		if cached := s.getFromCache(targetURL); cached != nil {
			return cached, nil
		}
	}

	// Rate limiting
	if err := s.waitForRateLimit(ctx, parsedURL.Host); err != nil {
		return &ScrapedContent{
			URL:       targetURL,
			Success:   false,
			Error:     fmt.Sprintf("rate limit: %v", err),
			ScrapedAt: time.Now(),
		}, err
	}

	// Intentar primero con HTTP simple
	content, err := s.scrapeHTTP(ctx, targetURL)
	if err == nil && s.contentLooksComplete(content) {
		s.saveToCache(targetURL, content)
		return content, nil
	}

	// Si HTTP falló o el contenido parece incompleto, usar browser
	if s.config.EnableBrowser {
		browserContent, browserErr := s.scrapeBrowser(ctx, targetURL)
		if browserErr == nil {
			s.saveToCache(targetURL, browserContent)
			return browserContent, nil
		}
		// Si browser también falla, devolver el error de HTTP si teníamos contenido parcial
		if content != nil && content.Content != "" {
			s.saveToCache(targetURL, content)
			return content, nil
		}
		return browserContent, browserErr
	}

	// Solo HTTP disponible
	if content != nil {
		s.saveToCache(targetURL, content)
		return content, nil
	}

	return &ScrapedContent{
		URL:       targetURL,
		Success:   false,
		Error:     fmt.Sprintf("scraping falló: %v", err),
		ScrapedAt: time.Now(),
		Method:    "http",
	}, err
}

// ScrapeForAI extrae y formatea contenido optimizado para contexto de IA
func (s *Scraper) ScrapeForAI(ctx context.Context, targetURL string, maxChars int) (string, error) {
	content, err := s.Scrape(ctx, targetURL)
	if err != nil {
		return "", err
	}

	if !content.Success {
		return "", fmt.Errorf("scraping falló: %s", content.Error)
	}

	// Formatear para IA
	result := fmt.Sprintf("=== Contenido de: %s ===\n", content.URL)
	if content.Title != "" {
		result += fmt.Sprintf("Título: %s\n", content.Title)
	}
	result += fmt.Sprintf("Extraído: %s (método: %s)\n\n", content.ScrapedAt.Format("2006-01-02 15:04"), content.Method)

	text := content.Content
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars] + "\n\n[... contenido truncado ...]"
	}
	result += text

	return result, nil
}

// scrapeHTTP extrae contenido usando HTTP simple
func (s *Scraper) scrapeHTTP(ctx context.Context, targetURL string) (*ScrapedContent, error) {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Backoff exponencial
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("User-Agent", s.config.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "es-MX,es;q=0.9,en;q=0.8")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		defer resp.Body.Close()

		// Verificar status code
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				// Error de cliente, no reintentar
				break
			}
			continue
		}

		// Limitar tamaño de lectura
		limitReader := io.LimitReader(resp.Body, s.config.MaxContentSize)
		body, err := io.ReadAll(limitReader)
		if err != nil {
			lastErr = err
			continue
		}

		// Parsear HTML y extraer contenido
		title, content := s.parseHTML(string(body))

		// Calcular hash del contenido
		hash := sha256.Sum256([]byte(content))
		hashStr := hex.EncodeToString(hash[:8])

		return &ScrapedContent{
			URL:         targetURL,
			Title:       title,
			Content:     content,
			ContentHash: hashStr,
			ScrapedAt:   time.Now(),
			Method:      "http",
			Success:     true,
		}, nil
	}

	return nil, fmt.Errorf("después de %d intentos: %v", s.config.MaxRetries+1, lastErr)
}

// parseHTML extrae título y contenido de texto del HTML
func (s *Scraper) parseHTML(htmlContent string) (title string, content string) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback: extraer texto básico
		return "", s.stripHTMLTags(htmlContent)
	}

	var titleBuilder strings.Builder
	var contentBuilder strings.Builder
	var inScript, inStyle, inTitle bool

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "noscript":
				inScript = true
				defer func() { inScript = false }()
			case "style":
				inStyle = true
				defer func() { inStyle = false }()
			case "title":
				inTitle = true
				defer func() { inTitle = false }()
			case "br", "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
				contentBuilder.WriteString("\n")
			case "td", "th":
				contentBuilder.WriteString(" | ")
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if inTitle && titleBuilder.Len() == 0 {
					titleBuilder.WriteString(text)
				}
				if !inScript && !inStyle {
					contentBuilder.WriteString(text)
					contentBuilder.WriteString(" ")
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	// Limpiar contenido
	content = contentBuilder.String()
	content = s.cleanText(content)

	return titleBuilder.String(), content
}

// stripHTMLTags elimina tags HTML de forma básica
func (s *Scraper) stripHTMLTags(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return s.cleanText(re.ReplaceAllString(input, " "))
}

// cleanText limpia y normaliza texto
func (s *Scraper) cleanText(text string) string {
	// Remover múltiples espacios
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Remover múltiples saltos de línea
	newlineRe := regexp.MustCompile(`\n{3,}`)
	text = newlineRe.ReplaceAllString(text, "\n\n")

	// Remover caracteres de control (excepto newline y tab)
	controlRe := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	text = controlRe.ReplaceAllString(text, "")

	// Trim
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

// contentLooksComplete verifica si el contenido parece completo (no necesita JS)
func (s *Scraper) contentLooksComplete(content *ScrapedContent) bool {
	if content == nil || !content.Success {
		return false
	}

	text := content.Content

	// Si tiene muy poco contenido, probablemente necesita JS
	if len(text) < 200 {
		return false
	}

	// Patrones que indican que la página requiere JavaScript
	jsPatterns := []string{
		"enable javascript",
		"javascript required",
		"please enable javascript",
		"loading...",
		"cargando...",
		"react-root",
		"ng-app",
		"data-reactroot",
		"__nuxt",
		"__next",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range jsPatterns {
		if strings.Contains(lowerText, pattern) {
			return false
		}
	}

	return true
}

// checkDomain verifica si el dominio está permitido
func (s *Scraper) checkDomain(domain string) error {
	domain = strings.ToLower(domain)

	// Verificar blocklist
	for _, blocked := range s.config.BlockedDomains {
		if strings.Contains(domain, strings.ToLower(blocked)) {
			return fmt.Errorf("dominio bloqueado: %s", domain)
		}
	}

	// Verificar allowlist (si está configurada)
	if len(s.config.AllowedDomains) > 0 {
		for _, allowed := range s.config.AllowedDomains {
			if strings.Contains(domain, strings.ToLower(allowed)) {
				return nil
			}
		}
		return fmt.Errorf("dominio no permitido: %s", domain)
	}

	return nil
}

// waitForRateLimit espera si es necesario por rate limiting
func (s *Scraper) waitForRateLimit(ctx context.Context, domain string) error {
	s.rlMutex.Lock()
	rl, exists := s.rateLimiters[domain]
	if !exists {
		rl = &rateLimiter{
			tokens:     float64(s.config.BurstSize),
			lastUpdate: time.Now(),
		}
		s.rateLimiters[domain] = rl
	}
	s.rlMutex.Unlock()

	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// Actualizar tokens basado en tiempo transcurrido
	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.tokens += elapsed * s.config.RequestsPerSecond
	if rl.tokens > float64(s.config.BurstSize) {
		rl.tokens = float64(s.config.BurstSize)
	}
	rl.lastUpdate = now

	// Si no hay tokens, esperar
	if rl.tokens < 1 {
		waitTime := time.Duration((1-rl.tokens)/s.config.RequestsPerSecond*1000) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
		rl.tokens = 0
	} else {
		rl.tokens--
	}

	return nil
}

// getFromCache obtiene contenido del cache
func (s *Scraper) getFromCache(url string) *ScrapedContent {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	entry, exists := s.cache[url]
	if !exists {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.content
}

// saveToCache guarda contenido en cache
func (s *Scraper) saveToCache(url string, content *ScrapedContent) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.cache[url] = &cacheEntry{
		content:   content,
		expiresAt: time.Now().Add(s.config.CacheTTL),
	}
}

// ClearCache limpia el cache
func (s *Scraper) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache = make(map[string]*cacheEntry)
}

// scrapeBrowser extrae contenido usando browser headless
func (s *Scraper) scrapeBrowser(ctx context.Context, targetURL string) (*ScrapedContent, error) {
	s.browserMutex.Lock()
	if s.browserScraper == nil {
		var err error
		s.browserScraper, err = NewBrowserScraper(s.config)
		if err != nil {
			s.browserMutex.Unlock()
			return nil, fmt.Errorf("error inicializando browser: %w", err)
		}
	}
	s.browserMutex.Unlock()

	return s.browserScraper.Scrape(ctx, targetURL)
}

// IsBrowserAvailable verifica si el browser está disponible
func (s *Scraper) IsBrowserAvailable() bool {
	s.browserMutex.Lock()
	defer s.browserMutex.Unlock()

	if s.browserScraper == nil {
		return false
	}
	return s.browserScraper.IsAvailable()
}

// Close cierra recursos del scraper
func (s *Scraper) Close() error {
	s.browserMutex.Lock()
	defer s.browserMutex.Unlock()

	if s.browserScraper != nil {
		return s.browserScraper.Close()
	}
	return nil
}
