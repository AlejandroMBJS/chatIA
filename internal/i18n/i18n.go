package i18n

import (
	"net/http"
	"strings"
	"sync"
)

// Language representa un idioma soportado
type Language string

const (
	Spanish Language = "es"
	English Language = "en"
)

// DefaultLanguage es el idioma por defecto
const DefaultLanguage = Spanish

// Translator maneja las traducciones
type Translator struct {
	translations map[Language]map[string]string
	mu           sync.RWMutex
}

// Global translator instance
var T *Translator

func init() {
	T = NewTranslator()
	T.LoadTranslations()
}

// NewTranslator crea un nuevo traductor
func NewTranslator() *Translator {
	return &Translator{
		translations: make(map[Language]map[string]string),
	}
}

// LoadTranslations carga todas las traducciones
func (t *Translator) LoadTranslations() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Traducciones en Espanol
	t.translations[Spanish] = map[string]string{
		// General
		"app_name":           "AQUILA",
		"loading":            "Cargando...",
		"error":              "Error",
		"success":            "Exito",
		"save":               "Guardar",
		"cancel":             "Cancelar",
		"delete":             "Eliminar",
		"edit":               "Editar",
		"close":              "Cerrar",
		"back":               "Volver",
		"next":               "Siguiente",
		"previous":           "Anterior",
		"search":             "Buscar",
		"filter":             "Filtrar",
		"actions":            "Acciones",
		"confirm":            "Confirmar",
		"yes":                "Si",
		"no":                 "No",

		// Auth
		"login":                    "Iniciar Sesion",
		"logout":                   "Cerrar Sesion",
		"register":                 "Registrarse",
		"nomina":                   "Nomina",
		"password":                 "Contrasena",
		"confirm_password":         "Confirmar Contrasena",
		"name":                     "Nombre",
		"department":               "Departamento",
		"login_error":              "Credenciales invalidas",
		"register_success":         "Registro exitoso",
		"passwords_not_match":      "Las contrasenas no coinciden",
		"password_min_length":      "La contrasena debe tener al menos 6 caracteres",
		"nomina_exists":            "Esta nomina ya esta registrada",
		"all_fields_required":      "Todos los campos son requeridos",
		"try_login":                "Intentar Iniciar Sesion",
		"no_account":               "No tienes cuenta?",
		"have_account":             "Ya tienes cuenta?",

		// Pending approval
		"account_pending":          "Cuenta No Aprobada",
		"account_not_approved":     "Tu cuenta aun no ha sido aprobada",
		"pending_approval":         "Pendiente de aprobacion",
		"request_urgent_approval":  "Solicitar Aprobacion Urgente",
		"approval_requested":       "Solicitud enviada! Los administradores han sido notificados.",
		"already_approved":         "Tu cuenta ya esta aprobada!",
		"while_waiting":            "Mientras esperas:",
		"contact_supervisor":       "Contacta a tu supervisor si es urgente",
		"approval_time":            "El proceso normal toma entre 5 minutos y 24 horas",
		"access_when_approved":     "Recibiras acceso una vez aprobado",
		"wrong_credentials":        "Credenciales incorrectas?",
		"register_again":           "Registrate de nuevo",
		"check_if_approved":        "Verificar si ya fui aprobado",

		// Chat
		"group_chat":           "Chat Grupal",
		"ai_chat":              "Chat con IA",
		"send":                 "Enviar",
		"type_message":         "Escribe un mensaje...",
		"no_messages":          "No hay mensajes aun",
		"new_conversation":     "Nueva conversacion",
		"conversations":        "Conversaciones",
		"delete_conversation":  "Eliminar conversacion",
		"ai_unavailable":       "IA no disponible",
		"ai_thinking":          "Pensando...",

		// Admin
		"admin_panel":          "Panel de Administracion",
		"users":                "Usuarios",
		"pending_users":        "Usuarios Pendientes",
		"approved_users":       "Usuarios Aprobados",
		"approve":              "Aprobar",
		"reject":               "Rechazar",
		"security_filters":     "Filtros de Seguridad",
		"security_logs":        "Logs de Seguridad",
		"statistics":           "Estadisticas",
		"total_users":          "Total Usuarios",
		"total_messages":       "Total Mensajes",
		"total_conversations":  "Total Conversaciones",
		"incidents_today":      "Incidentes Hoy",
		"create_filter":        "Crear Filtro",
		"filter_name":          "Nombre del Filtro",
		"filter_type":          "Tipo de Filtro",
		"filter_pattern":       "Patron",
		"filter_action":        "Accion",
		"filter_severity":      "Severidad",
		"active":               "Activo",
		"inactive":             "Inactivo",

		// Profile
		"my_profile":           "Mi Perfil",
		"member_since":         "Miembro desde",
		"account_status":       "Estado de Cuenta",

		// Navigation
		"home":                 "Inicio",
		"settings":             "Configuracion",
		"help":                 "Ayuda",
		"language":             "Idioma",
		"spanish":              "Espanol",
		"english":              "Ingles",

		// Errors
		"error_processing":     "Error procesando solicitud",
		"error_internal":       "Error interno del servidor",
		"error_not_found":      "No encontrado",
		"error_unauthorized":   "No autorizado",
		"error_forbidden":      "Acceso denegado",
		"session_expired":      "Sesion expirada",

		// Scraping
		"extracting_content":   "Extrayendo contenido de URL...",
		"content_extracted":    "Contenido extraido",
		"extraction_error":     "Error extrayendo contenido",
	}

	// Traducciones en Ingles
	t.translations[English] = map[string]string{
		// General
		"app_name":           "AQUILA",
		"loading":            "Loading...",
		"error":              "Error",
		"success":            "Success",
		"save":               "Save",
		"cancel":             "Cancel",
		"delete":             "Delete",
		"edit":               "Edit",
		"close":              "Close",
		"back":               "Back",
		"next":               "Next",
		"previous":           "Previous",
		"search":             "Search",
		"filter":             "Filter",
		"actions":            "Actions",
		"confirm":            "Confirm",
		"yes":                "Yes",
		"no":                 "No",

		// Auth
		"login":                    "Login",
		"logout":                   "Logout",
		"register":                 "Register",
		"nomina":                   "Employee ID",
		"password":                 "Password",
		"confirm_password":         "Confirm Password",
		"name":                     "Name",
		"department":               "Department",
		"login_error":              "Invalid credentials",
		"register_success":         "Registration successful",
		"passwords_not_match":      "Passwords do not match",
		"password_min_length":      "Password must be at least 6 characters",
		"nomina_exists":            "This employee ID is already registered",
		"all_fields_required":      "All fields are required",
		"try_login":                "Try to Login",
		"no_account":               "Don't have an account?",
		"have_account":             "Already have an account?",

		// Pending approval
		"account_pending":          "Account Not Approved",
		"account_not_approved":     "Your account has not been approved yet",
		"pending_approval":         "Pending approval",
		"request_urgent_approval":  "Request Urgent Approval",
		"approval_requested":       "Request sent! Administrators have been notified.",
		"already_approved":         "Your account is already approved!",
		"while_waiting":            "While you wait:",
		"contact_supervisor":       "Contact your supervisor if urgent",
		"approval_time":            "Normal process takes 5 minutes to 24 hours",
		"access_when_approved":     "You will get access once approved",
		"wrong_credentials":        "Wrong credentials?",
		"register_again":           "Register again",
		"check_if_approved":        "Check if already approved",

		// Chat
		"group_chat":           "Group Chat",
		"ai_chat":              "AI Chat",
		"send":                 "Send",
		"type_message":         "Type a message...",
		"no_messages":          "No messages yet",
		"new_conversation":     "New conversation",
		"conversations":        "Conversations",
		"delete_conversation":  "Delete conversation",
		"ai_unavailable":       "AI unavailable",
		"ai_thinking":          "Thinking...",

		// Admin
		"admin_panel":          "Admin Panel",
		"users":                "Users",
		"pending_users":        "Pending Users",
		"approved_users":       "Approved Users",
		"approve":              "Approve",
		"reject":               "Reject",
		"security_filters":     "Security Filters",
		"security_logs":        "Security Logs",
		"statistics":           "Statistics",
		"total_users":          "Total Users",
		"total_messages":       "Total Messages",
		"total_conversations":  "Total Conversations",
		"incidents_today":      "Incidents Today",
		"create_filter":        "Create Filter",
		"filter_name":          "Filter Name",
		"filter_type":          "Filter Type",
		"filter_pattern":       "Pattern",
		"filter_action":        "Action",
		"filter_severity":      "Severity",
		"active":               "Active",
		"inactive":             "Inactive",

		// Profile
		"my_profile":           "My Profile",
		"member_since":         "Member since",
		"account_status":       "Account Status",

		// Navigation
		"home":                 "Home",
		"settings":             "Settings",
		"help":                 "Help",
		"language":             "Language",
		"spanish":              "Spanish",
		"english":              "English",

		// Errors
		"error_processing":     "Error processing request",
		"error_internal":       "Internal server error",
		"error_not_found":      "Not found",
		"error_unauthorized":   "Unauthorized",
		"error_forbidden":      "Access denied",
		"session_expired":      "Session expired",

		// Scraping
		"extracting_content":   "Extracting content from URL...",
		"content_extracted":    "Content extracted",
		"extraction_error":     "Error extracting content",
	}
}

// Get obtiene una traduccion
func (t *Translator) Get(lang Language, key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if translations, ok := t.translations[lang]; ok {
		if text, ok := translations[key]; ok {
			return text
		}
	}

	// Fallback a espanol
	if translations, ok := t.translations[Spanish]; ok {
		if text, ok := translations[key]; ok {
			return text
		}
	}

	// Devolver la key si no se encuentra
	return key
}

// GetAll obtiene todas las traducciones para un idioma
func (t *Translator) GetAll(lang Language) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]string)
	if translations, ok := t.translations[lang]; ok {
		for k, v := range translations {
			result[k] = v
		}
	}
	return result
}

// DetectLanguage detecta el idioma del request
func DetectLanguage(r *http.Request) Language {
	// 1. Primero verificar cookie
	if cookie, err := r.Cookie("lang"); err == nil {
		lang := Language(cookie.Value)
		if lang == Spanish || lang == English {
			return lang
		}
	}

	// 2. Verificar header Accept-Language
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		// Parsear Accept-Language (ej: "es-MX,es;q=0.9,en;q=0.8")
		parts := strings.Split(acceptLang, ",")
		for _, part := range parts {
			lang := strings.TrimSpace(strings.Split(part, ";")[0])
			lang = strings.Split(lang, "-")[0] // es-MX -> es

			switch lang {
			case "es":
				return Spanish
			case "en":
				return English
			}
		}
	}

	return DefaultLanguage
}

// SetLanguageCookie establece la cookie de idioma
func SetLanguageCookie(w http.ResponseWriter, lang Language) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    string(lang),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 ano
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// Tr es un helper para obtener traducciones rapidamente
func Tr(lang Language, key string) string {
	return T.Get(lang, key)
}

// TrMap devuelve un mapa de traducciones para usar en templates
func TrMap(lang Language) map[string]string {
	return T.GetAll(lang)
}
