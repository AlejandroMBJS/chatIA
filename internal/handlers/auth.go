package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"chat-empleados/db"
	"chat-empleados/internal/config"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	queries       *db.Queries
	cfg           *config.Config
	templates     *template.Template
	notifications *services.NotificationService
}

func NewAuthHandler(queries *db.Queries, cfg *config.Config, templates *template.Template, notifications *services.NotificationService) *AuthHandler {
	return &AuthHandler{
		queries:       queries,
		cfg:           cfg,
		templates:     templates,
		notifications: notifications,
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Iniciar Sesion",
		"Error": r.URL.Query().Get("error"),
	}
	h.templates.ExecuteTemplate(w, "login.html", data)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Error procesando formulario", http.StatusSeeOther)
		return
	}

	nomina := strings.TrimSpace(r.FormValue("nomina"))
	password := r.FormValue("password")

	if nomina == "" || password == "" {
		http.Redirect(w, r, "/login?error=Nomina y contrasena son requeridos", http.StatusSeeOther)
		return
	}

	clientIP := getClientIP(r)

	user, err := h.queries.GetUserByNomina(r.Context(), nomina)
	if err != nil {
		log.Printf("[WARN] Intento de login fallido para nomina: %s desde IP: %s", nomina, clientIP)
		http.Redirect(w, r, "/login?error=Credenciales invalidas", http.StatusSeeOther)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Printf("[WARN] Password incorrecto para nomina: %s desde IP: %s", nomina, clientIP)
		http.Redirect(w, r, "/login?error=Credenciales invalidas", http.StatusSeeOther)
		return
	}

	if !user.Approved.Valid || user.Approved.Int64 == 0 {
		http.Redirect(w, r, "/pending", http.StatusSeeOther)
		return
	}

	token, err := generateToken()
	if err != nil {
		log.Printf("[ERROR] Error generando token: %v", err)
		http.Redirect(w, r, "/login?error=Error interno", http.StatusSeeOther)
		return
	}

	expiresAt := time.Now().Add(h.cfg.SessionDuration)

	_, err = h.queries.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		log.Printf("[ERROR] Error creando sesion: %v", err)
		http.Redirect(w, r, "/login?error=Error interno", http.StatusSeeOther)
		return
	}

	// Resetear rate limiter después de login exitoso
	middleware.ResetAuthRateLimit(clientIP)

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil || h.cfg.ForceSecureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	log.Printf("[INFO] Login exitoso: %s (%s) desde IP: %s", user.Nombre, user.Nomina, clientIP)
	http.Redirect(w, r, "/chat", http.StatusSeeOther)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Registro",
		"Error": r.URL.Query().Get("error"),
	}
	h.templates.ExecuteTemplate(w, "register.html", data)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/register?error=Error procesando formulario", http.StatusSeeOther)
		return
	}

	nomina := strings.TrimSpace(r.FormValue("nomina"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	nombre := strings.TrimSpace(r.FormValue("nombre"))
	departamento := strings.TrimSpace(r.FormValue("departamento"))

	if nomina == "" || password == "" || nombre == "" {
		http.Redirect(w, r, "/register?error=Todos los campos son requeridos", http.StatusSeeOther)
		return
	}

	if len(password) < 6 {
		http.Redirect(w, r, "/register?error=La contrasena debe tener al menos 6 caracteres", http.StatusSeeOther)
		return
	}

	if password != passwordConfirm {
		http.Redirect(w, r, "/register?error=Las contrasenas no coinciden", http.StatusSeeOther)
		return
	}

	_, err := h.queries.GetUserByNomina(r.Context(), nomina)
	if err == nil {
		http.Redirect(w, r, "/register?error=Esta nomina ya esta registrada", http.StatusSeeOther)
		return
	}
	if err != sql.ErrNoRows {
		log.Printf("[ERROR] Error verificando nomina: %v", err)
		http.Redirect(w, r, "/register?error=Error interno", http.StatusSeeOther)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[ERROR] Error hasheando password: %v", err)
		http.Redirect(w, r, "/register?error=Error interno", http.StatusSeeOther)
		return
	}

	_, err = h.queries.CreateUser(r.Context(), db.CreateUserParams{
		Nomina:       nomina,
		PasswordHash: string(hashedPassword),
		Nombre:       nombre,
		Departamento: sql.NullString{String: departamento, Valid: departamento != ""},
	})
	if err != nil {
		log.Printf("[ERROR] Error creando usuario: %v", err)
		http.Redirect(w, r, "/register?error=Error creando cuenta", http.StatusSeeOther)
		return
	}

	// Notificar a los administradores sobre el nuevo usuario pendiente
	go h.notifications.NotifyAdminsNewUser(context.Background(), nombre, nomina)

	log.Printf("[INFO] Nuevo registro: %s (%s)", nombre, nomina)
	http.Redirect(w, r, "/pending", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		h.queries.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) PendingPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Cuenta Pendiente",
	}
	h.templates.ExecuteTemplate(w, "pending.html", data)
}

func (h *AuthHandler) Profile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"Title": "Mi Perfil",
		"User":  user,
	}
	h.templates.ExecuteTemplate(w, "profile.html", data)
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
