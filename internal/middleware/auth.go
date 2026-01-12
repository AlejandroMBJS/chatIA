package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"chat-empleados/db"
	"chat-empleados/internal/i18n"
)

type contextKey string

const (
	UserContextKey    contextKey = "user"
	SessionContextKey contextKey = "session"
	CSRFContextKey    contextKey = "csrf_token"
	LangContextKey    contextKey = "lang"
)

// Rate limiter para prevenir ataques de fuerza bruta
type RateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
}

var globalRateLimiter = &RateLimiter{
	requests: make(map[string][]time.Time),
	limit:    200,            // 200 requests
	window:   time.Minute,    // por minuto
}

var authRateLimiter = &RateLimiter{
	requests: make(map[string][]time.Time),
	limit:    5,                  // 5 intentos
	window:   15 * time.Minute,   // en 15 minutos
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Limpiar requests antiguos
	if times, exists := rl.requests[key]; exists {
		valid := make([]time.Time, 0)
		for _, t := range times {
			if t.After(windowStart) {
				valid = append(valid, t)
			}
		}
		rl.requests[key] = valid
	}

	// Verificar límite
	if len(rl.requests[key]) >= rl.limit {
		return false
	}

	// Registrar request
	rl.requests[key] = append(rl.requests[key], now)
	return true
}

func (rl *RateLimiter) Reset(key string) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	delete(rl.requests, key)
}

type AuthUser struct {
	ID           int64
	Nomina       string
	Nombre       string
	IsAdmin      bool
	Approved     bool
	Departamento string
	SessionToken string
}

type AuthMiddleware struct {
	queries *db.Queries
}

func NewAuthMiddleware(queries *db.Queries) *AuthMiddleware {
	return &AuthMiddleware{queries: queries}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.getUserFromRequest(r)
		if user == nil {
			if isHTMXRequest(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if !user.Approved {
			if isHTMXRequest(r) {
				w.Header().Set("HX-Redirect", "/pending")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Redirect(w, r, "/pending", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.getUserFromRequest(r)
		if user == nil {
			if isHTMXRequest(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if !user.IsAdmin {
			if isHTMXRequest(r) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("Acceso denegado"))
				return
			}
			http.Error(w, "Acceso denegado", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.getUserFromRequest(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) RedirectIfAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := m.getUserFromRequest(r)
		if user != nil && user.Approved {
			http.Redirect(w, r, "/chat", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) getUserFromRequest(r *http.Request) *AuthUser {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return nil
	}

	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return nil
	}

	session, err := m.queries.GetSessionByToken(r.Context(), token)
	if err != nil {
		return nil
	}

	return &AuthUser{
		ID:           session.UserID,
		Nomina:       session.Nomina,
		Nombre:       session.Nombre,
		IsAdmin:      session.IsAdmin.Valid && session.IsAdmin.Int64 != 0,
		Approved:     session.Approved.Valid && session.Approved.Int64 != 0,
		Departamento: stringFromNullable(session.Departamento),
		SessionToken: token,
	}
}

func GetUserFromContext(ctx context.Context) *AuthUser {
	user, ok := ctx.Value(UserContextKey).(*AuthUser)
	if !ok {
		return nil
	}
	return user
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func stringFromNullable(s interface{}) string {
	switch v := s.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, getClientIP(r))
		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// RateLimit middleware para limitar requests por IP
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !globalRateLimiter.Allow(ip) {
			log.Printf("[SECURITY] Rate limit exceeded for IP: %s", ip)
			http.Error(w, "Demasiadas solicitudes. Intenta de nuevo en un minuto.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthRateLimit middleware específico para endpoints de autenticación
func AuthRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !authRateLimiter.Allow(ip) {
			log.Printf("[SECURITY] Auth rate limit exceeded for IP: %s", ip)
			http.Error(w, "Demasiados intentos de autenticacion. Intenta de nuevo en 15 minutos.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ResetAuthRateLimit resetea el rate limiter después de login exitoso
func ResetAuthRateLimit(ip string) {
	authRateLimiter.Reset(ip)
}

// CSRF token generation and validation
func GenerateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func ValidateCSRFToken(r *http.Request, sessionToken string) bool {
	// Para requests HTMX o fetch, el token viene en header
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		// También verificar en form data
		token = r.FormValue("csrf_token")
	}
	if token == "" {
		return false
	}

	// El CSRF token debe coincidir con el almacenado en la sesión
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) == 1
}

// CSRFProtection middleware
func CSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Solo aplicar a métodos que modifican estado
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
			// Excluir endpoints de API que usan otros mecanismos
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				cookie, err := r.Cookie("csrf_token")
				if err != nil {
					// Generar nuevo token si no existe
					token, err := GenerateCSRFToken()
					if err != nil {
						http.Error(w, "Error interno", http.StatusInternalServerError)
						return
					}
					http.SetCookie(w, &http.Cookie{
						Name:     "csrf_token",
						Value:    token,
						Path:     "/",
						HttpOnly: false, // JavaScript necesita leerlo
						SameSite: http.SameSiteStrictMode,
						Secure:   r.TLS != nil,
					})
					http.Error(w, "Sesion expirada. Recarga la pagina.", http.StatusForbidden)
					return
				}

				// Validar token
				formToken := r.FormValue("csrf_token")
				headerToken := r.Header.Get("X-CSRF-Token")
				token := formToken
				if token == "" {
					token = headerToken
				}

				if subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
					log.Printf("[SECURITY] CSRF token mismatch from IP: %s", getClientIP(r))
					http.Error(w, "Token de seguridad invalido", http.StatusForbidden)
					return
				}
			}
		}

		// Generar token para la respuesta si no existe
		if _, err := r.Cookie("csrf_token"); err != nil {
			token, _ := GenerateCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     "csrf_token",
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteStrictMode,
				Secure:   r.TLS != nil,
			})
		}

		next.ServeHTTP(w, r)
	})
}

// GetCSRFToken obtiene el token CSRF de la cookie
func GetCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func getClientIP(r *http.Request) string {
	// Verificar headers de proxy (solo confiar si está detrás de un proxy conocido)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Extraer IP sin puerto
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// LanguageMiddleware detecta y establece el idioma del usuario
func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.DetectLanguage(r)
		ctx := context.WithValue(r.Context(), LangContextKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetLanguageFromContext obtiene el idioma del contexto
func GetLanguageFromContext(ctx context.Context) i18n.Language {
	lang, ok := ctx.Value(LangContextKey).(i18n.Language)
	if !ok {
		return i18n.DefaultLanguage
	}
	return lang
}

// GetTranslations obtiene todas las traducciones para el idioma actual
func GetTranslations(ctx context.Context) map[string]string {
	lang := GetLanguageFromContext(ctx)
	return i18n.TrMap(lang)
}
