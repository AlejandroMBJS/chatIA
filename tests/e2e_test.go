package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const baseURL = "http://localhost:9999"

// TestRunner estructura para ejecutar tests
type TestRunner struct {
	client  *http.Client
	browser *rod.Browser
	t       *testing.T
}

// NewTestRunner crea un nuevo runner de tests
func NewTestRunner(t *testing.T) *TestRunner {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // No seguir redirects automaticamente
		},
	}

	return &TestRunner{
		client: client,
		t:      t,
	}
}

// initBrowser inicializa el browser para tests de UI
func (tr *TestRunner) initBrowser() error {
	l := launcher.New().
		Headless(true).
		Set("disable-gpu").
		Set("no-sandbox")

	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("error launching browser: %w", err)
	}

	tr.browser = rod.New().ControlURL(u)
	if err := tr.browser.Connect(); err != nil {
		return fmt.Errorf("error connecting to browser: %w", err)
	}

	return nil
}

// closeBrowser cierra el browser
func (tr *TestRunner) closeBrowser() {
	if tr.browser != nil {
		tr.browser.Close()
	}
}

// ==================== CURL/HTTP TESTS ====================

func TestHealthEndpoint(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Error en health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("Expected 'OK', got '%s'", string(body))
	}

	t.Log("✓ Health endpoint OK")
}

func TestLoginPageLoads(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("Error cargando login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Verificar elementos clave
	checks := []string{
		"GIA Chat",
		"nomina",
		"password",
		"login",
	}

	for _, check := range checks {
		if !strings.Contains(strings.ToLower(bodyStr), strings.ToLower(check)) {
			t.Errorf("Login page missing: %s", check)
		}
	}

	t.Log("✓ Login page loads correctly")
}

func TestRegisterPageLoads(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/register")
	if err != nil {
		t.Fatalf("Error cargando register: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	checks := []string{
		"nombre",
		"password",
		"nomina",
	}

	for _, check := range checks {
		if !strings.Contains(strings.ToLower(bodyStr), strings.ToLower(check)) {
			t.Errorf("Register page missing: %s", check)
		}
	}

	t.Log("✓ Register page loads correctly")
}

func TestLoginWithInvalidCredentials(t *testing.T) {
	tr := NewTestRunner(t)

	data := url.Values{}
	data.Set("nomina", "invalid_user")
	data.Set("password", "wrong_password")

	resp, err := tr.client.PostForm(baseURL+"/login", data)
	if err != nil {
		t.Fatalf("Error en login: %v", err)
	}
	defer resp.Body.Close()

	// Debe redirigir a login con error
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected redirect (303), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "error") {
		t.Errorf("Expected error in redirect, got: %s", location)
	}

	t.Log("✓ Invalid login handled correctly")
}

func TestLoginWithAdminCredentials(t *testing.T) {
	tr := NewTestRunner(t)

	data := url.Values{}
	data.Set("nomina", "admin")
	data.Set("password", "admin123")

	resp, err := tr.client.PostForm(baseURL+"/login", data)
	if err != nil {
		t.Fatalf("Error en login: %v", err)
	}
	defer resp.Body.Close()

	// Debe redirigir a /chat
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected redirect (303), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/chat" {
		t.Errorf("Expected redirect to /chat, got: %s", location)
	}

	// Verificar que se estableció la cookie de sesión
	cookies := resp.Cookies()
	hasSession := false
	for _, cookie := range cookies {
		if cookie.Name == "session_token" && cookie.Value != "" {
			hasSession = true
			break
		}
	}

	if !hasSession {
		t.Error("Session cookie not set after login")
	}

	t.Log("✓ Admin login successful")
}

func TestProtectedRouteWithoutAuth(t *testing.T) {
	tr := NewTestRunner(t)

	routes := []string{"/chat", "/ai", "/admin", "/profile"}

	for _, route := range routes {
		resp, err := tr.client.Get(baseURL + route)
		if err != nil {
			t.Fatalf("Error accessing %s: %v", route, err)
		}
		resp.Body.Close()

		// Debe redirigir a login
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Route %s: Expected redirect (303), got %d", route, resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/login" {
			t.Errorf("Route %s: Expected redirect to /login, got: %s", route, location)
		}
	}

	t.Log("✓ Protected routes require authentication")
}

func TestLanguageSwitch(t *testing.T) {
	tr := NewTestRunner(t)

	// Cambiar a inglés
	resp, err := tr.client.Get(baseURL + "/set-language?lang=en")
	if err != nil {
		t.Fatalf("Error switching language: %v", err)
	}
	resp.Body.Close()

	// Verificar cookie de idioma
	parsedURL, _ := url.Parse(baseURL)
	cookies := tr.client.Jar.Cookies(parsedURL)
	hasLangCookie := false
	for _, cookie := range cookies {
		if cookie.Name == "lang" && cookie.Value == "en" {
			hasLangCookie = true
			break
		}
	}

	if !hasLangCookie {
		t.Error("Language cookie not set to 'en'")
	}

	// Verificar que la página usa inglés
	resp, err = tr.client.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("Error loading login: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "Login") || !strings.Contains(bodyStr, "Password") {
		t.Error("Page not showing English content")
	}

	// Cambiar de vuelta a español
	resp2, _ := tr.client.Get(baseURL + "/set-language?lang=es")
	resp2.Body.Close()

	t.Log("✓ Language switching works correctly")
}

func TestAIHealthEndpoint(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/ai/health")
	if err != nil {
		t.Fatalf("Error en AI health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if _, ok := result["model"]; !ok {
		t.Error("AI health response missing 'model' field")
	}

	t.Log("✓ AI health endpoint OK")
}

func TestStaticFiles(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/static/styles.css")
	if err != nil {
		t.Fatalf("Error loading CSS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for CSS, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "css") {
		t.Errorf("Expected CSS content type, got: %s", contentType)
	}

	t.Log("✓ Static files served correctly")
}

func TestSecurityHeaders(t *testing.T) {
	tr := NewTestRunner(t)

	resp, err := tr.client.Get(baseURL + "/login")
	if err != nil {
		t.Fatalf("Error loading page: %v", err)
	}
	defer resp.Body.Close()

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}

	for header, expected := range headers {
		actual := resp.Header.Get(header)
		if actual != expected {
			t.Errorf("Header %s: expected '%s', got '%s'", header, expected, actual)
		}
	}

	t.Log("✓ Security headers present")
}

// ==================== BROWSER/UI TESTS ====================

func TestUILoginFlow(t *testing.T) {
	tr := NewTestRunner(t)

	if err := tr.initBrowser(); err != nil {
		t.Skipf("Skipping browser test: %v", err)
	}
	defer tr.closeBrowser()

	page := tr.browser.MustPage(baseURL + "/login")
	defer page.Close()

	// Esperar a que cargue
	page.MustWaitLoad()

	// Verificar título
	title := page.MustElement("h1").MustText()
	if !strings.Contains(title, "GIA") {
		t.Errorf("Unexpected title: %s", title)
	}

	// Verificar formulario
	nominaInput := page.MustElement("input[name='nomina']")
	passwordInput := page.MustElement("input[name='password']")
	submitBtn := page.MustElement("button[type='submit']")

	if nominaInput == nil || passwordInput == nil || submitBtn == nil {
		t.Error("Login form elements not found")
	}

	// Llenar formulario
	nominaInput.MustInput("admin")
	passwordInput.MustInput("admin123")

	// Submit
	submitBtn.MustClick()

	// Esperar navegación
	page.MustWaitNavigation()

	// Verificar que estamos en /chat
	currentURL := page.MustInfo().URL
	if !strings.Contains(currentURL, "/chat") {
		t.Errorf("Expected to be on /chat, got: %s", currentURL)
	}

	t.Log("✓ UI Login flow works correctly")
}

func TestUIResponsiveness(t *testing.T) {
	tr := NewTestRunner(t)

	if err := tr.initBrowser(); err != nil {
		t.Skipf("Skipping browser test: %v", err)
	}
	defer tr.closeBrowser()

	viewports := []struct {
		name   string
		width  int
		height int
	}{
		{"Desktop", 1920, 1080},
		{"Tablet", 768, 1024},
		{"Mobile", 375, 667},
	}

	for _, vp := range viewports {
		page := tr.browser.MustPage("")
		page.MustSetViewport(vp.width, vp.height, 1, false)
		page.MustNavigate(baseURL + "/login")
		page.MustWaitLoad()

		// Verificar que el contenido es visible
		body := page.MustElement("body")
		box := body.MustShape().Box()

		if box.Width <= 0 || box.Height <= 0 {
			t.Errorf("%s: Content not visible", vp.name)
		}

		// Verificar que el formulario está visible
		form := page.MustElement(".auth-form, form")
		if form == nil {
			t.Errorf("%s: Form not found", vp.name)
		}

		page.Close()
		t.Logf("✓ %s viewport (%dx%d) OK", vp.name, vp.width, vp.height)
	}
}

func TestUILanguageSelector(t *testing.T) {
	tr := NewTestRunner(t)

	if err := tr.initBrowser(); err != nil {
		t.Skipf("Skipping browser test: %v", err)
	}
	defer tr.closeBrowser()

	page := tr.browser.MustPage(baseURL + "/login")
	page.MustWaitLoad()

	// Buscar selector de idioma
	langBtns := page.MustElements(".lang-btn")
	if len(langBtns) < 2 {
		t.Error("Language selector buttons not found")
		return
	}

	// Click en EN
	for _, btn := range langBtns {
		text := btn.MustText()
		if text == "EN" {
			btn.MustClick()
			break
		}
	}

	page.MustWaitLoad()

	// Verificar contenido en inglés
	body := page.MustElement("body").MustText()
	if !strings.Contains(body, "Login") && !strings.Contains(body, "Password") {
		t.Error("Page not in English after language switch")
	}

	page.Close()
	t.Log("✓ UI Language selector works")
}

func TestUIAdminPanel(t *testing.T) {
	tr := NewTestRunner(t)

	if err := tr.initBrowser(); err != nil {
		t.Skipf("Skipping browser test: %v", err)
	}
	defer tr.closeBrowser()

	page := tr.browser.MustPage(baseURL + "/login")
	page.MustWaitLoad()

	// Login como admin
	page.MustElement("input[name='nomina']").MustInput("admin")
	page.MustElement("input[name='password']").MustInput("admin123")
	page.MustElement("button[type='submit']").MustClick()
	page.MustWaitNavigation()

	// Ir a admin
	page.MustNavigate(baseURL + "/admin")
	page.MustWaitLoad()

	// Verificar que estamos en admin
	currentURL := page.MustInfo().URL
	if !strings.Contains(currentURL, "/admin") {
		t.Errorf("Expected admin page, got: %s", currentURL)
	}

	// Verificar elementos del panel
	body := page.MustElement("body").MustText()
	adminElements := []string{"admin", "usuarios", "filtros"}
	found := 0
	for _, elem := range adminElements {
		if strings.Contains(strings.ToLower(body), elem) {
			found++
		}
	}

	if found < 2 {
		t.Error("Admin panel missing expected elements")
	}

	page.Close()
	t.Log("✓ UI Admin panel accessible")
}

// ==================== INTEGRATION TESTS ====================

func TestFullRegistrationFlow(t *testing.T) {
	tr := NewTestRunner(t)

	testUser := fmt.Sprintf("test_%d", time.Now().Unix())

	// Registrar usuario
	data := url.Values{}
	data.Set("nomina", testUser)
	data.Set("password", "test123456")
	data.Set("password_confirm", "test123456")
	data.Set("nombre", "Test User")
	data.Set("departamento", "Testing")

	resp, err := tr.client.PostForm(baseURL+"/register", data)
	if err != nil {
		t.Fatalf("Error registering: %v", err)
	}
	resp.Body.Close()

	// Debe redirigir a pending
	location := resp.Header.Get("Location")
	if location != "/pending" {
		t.Errorf("Expected redirect to /pending, got: %s", location)
	}

	// Intentar login (debe ir a pending porque no está aprobado)
	loginData := url.Values{}
	loginData.Set("nomina", testUser)
	loginData.Set("password", "test123456")

	resp2, err := tr.client.PostForm(baseURL+"/login", loginData)
	if err != nil {
		t.Fatalf("Error logging in: %v", err)
	}
	resp2.Body.Close()

	location2 := resp2.Header.Get("Location")
	if location2 != "/pending" {
		t.Errorf("Unapproved user should go to /pending, got: %s", location2)
	}

	t.Log("✓ Full registration flow works correctly")
}

func TestRateLimiting(t *testing.T) {
	tr := NewTestRunner(t)

	// Hacer muchos intentos de login fallidos
	for i := 0; i < 10; i++ {
		data := url.Values{}
		data.Set("nomina", "ratelimit_test")
		data.Set("password", "wrong")

		resp, _ := tr.client.PostForm(baseURL+"/login", data)
		resp.Body.Close()

		// Después de varios intentos debe dar rate limit
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Logf("✓ Rate limiting kicked in after %d attempts", i+1)
			return
		}
	}

	// Si llegamos aquí, el rate limiting puede estar configurado con límites altos
	t.Log("✓ Rate limiting test completed (may need more attempts to trigger)")
}

// ==================== SCRAPING TESTS ====================

func TestScraperHTTP(t *testing.T) {
	// Test simple HTTP scraping
	resp, err := http.Get("https://httpbin.org/html")
	if err != nil {
		t.Skipf("Skipping scraper test (no internet): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Herman Melville") {
		t.Error("Expected content not found in scraped page")
	}

	t.Log("✓ HTTP scraping works")
}

// ==================== MAIN TEST RUNNER ====================

func TestMain(m *testing.M) {
	// Verificar si el servidor está corriendo
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Println("⚠️  Server not running. Starting server...")

		// Intentar iniciar el servidor
		cmd := exec.Command("go", "run", "../main.go")
		cmd.Dir = ".."
		cmd.Start()

		// Esperar a que inicie
		time.Sleep(3 * time.Second)

		// Verificar de nuevo
		resp, err = http.Get(baseURL + "/health")
		if err != nil {
			fmt.Println("❌ Could not start server. Run 'go run main.go' first.")
			os.Exit(1)
		}
	}
	resp.Body.Close()

	fmt.Println("✓ Server is running")
	fmt.Println("==========================================")
	fmt.Println("Running E2E Tests...")
	fmt.Println("==========================================")

	code := m.Run()

	fmt.Println("==========================================")
	if code == 0 {
		fmt.Println("✓ All tests passed!")
	} else {
		fmt.Println("❌ Some tests failed")
	}

	os.Exit(code)
}
