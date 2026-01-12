package main

import (
	"context"
	"database/sql"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"chat-empleados/db"
	"chat-empleados/internal/config"
	"chat-empleados/internal/handlers"
	"chat-empleados/internal/i18n"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

func main() {
	cfg := config.Load()

	database, err := initDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Error inicializando base de datos: %v", err)
	}
	defer database.Close()

	queries := db.New(database)

	templates, err := loadTemplates()
	if err != nil {
		log.Fatalf("[FATAL] Error cargando templates: %v", err)
	}

	securityService := services.NewSecurityService(queries)
	ollamaService := services.NewOllamaService(cfg, securityService)
	notificationService := services.NewNotificationService(queries)

	authMiddleware := middleware.NewAuthMiddleware(queries)
	authHandler := handlers.NewAuthHandler(queries, cfg, templates, notificationService)
	// chatHandler deshabilitado temporalmente
	// chatHandler := handlers.NewChatHandler(queries, templates, securityService)
	aiHandler := handlers.NewAIHandler(queries, templates, ollamaService, securityService)
	adminHandler := handlers.NewAdminHandler(queries, templates, securityService, notificationService)
	knowledgeHandler := handlers.NewKnowledgeHandler(queries, templates, notificationService)

	mux := http.NewServeMux()

	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.Handle("GET /login", authMiddleware.RedirectIfAuth(http.HandlerFunc(authHandler.LoginPage)))
	mux.Handle("POST /login", middleware.AuthRateLimit(http.HandlerFunc(authHandler.Login)))
	mux.Handle("GET /register", authMiddleware.RedirectIfAuth(http.HandlerFunc(authHandler.RegisterPage)))
	mux.Handle("POST /register", middleware.AuthRateLimit(http.HandlerFunc(authHandler.Register)))
	mux.HandleFunc("POST /logout", authHandler.Logout)
	mux.HandleFunc("GET /pending", authHandler.PendingPage)
	mux.HandleFunc("POST /request-approval", authHandler.RequestApproval)

	// Cambio de idioma
	mux.HandleFunc("POST /set-language", func(w http.ResponseWriter, r *http.Request) {
		lang := r.FormValue("lang")
		if lang == "en" {
			i18n.SetLanguageCookie(w, i18n.English)
		} else {
			i18n.SetLanguageCookie(w, i18n.Spanish)
		}
		// Redirigir a la pagina anterior o al inicio
		referer := r.Header.Get("Referer")
		if referer == "" {
			referer = "/"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /set-language", func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "en" {
			i18n.SetLanguageCookie(w, i18n.English)
		} else {
			i18n.SetLanguageCookie(w, i18n.Spanish)
		}
		referer := r.Header.Get("Referer")
		if referer == "" {
			referer = "/"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	})

	mux.Handle("GET /", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ai", http.StatusSeeOther)
	})))

	// Chat grupal deshabilitado temporalmente - redirigir a IA
	mux.Handle("GET /chat", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ai", http.StatusSeeOther)
	})))

	mux.Handle("GET /ai", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.AIPage)))
	mux.Handle("GET /ai/new", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.NewConversation)))
	mux.Handle("POST /ai/send", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.SendMessage)))
	mux.Handle("POST /ai/stream", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.SendMessageStream)))
	mux.Handle("DELETE /ai/conversation/{id}", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.DeleteConversation)))
	mux.Handle("GET /ai/conversation/{id}/messages", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.GetConversationMessages)))
	mux.Handle("GET /ai/health", http.HandlerFunc(aiHandler.HealthCheck))

	mux.Handle("GET /profile", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Profile)))
	mux.Handle("POST /profile/password", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.ChangePassword)))

	mux.Handle("GET /admin", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.Dashboard)))
	mux.Handle("GET /admin/users", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.Users)))
	mux.Handle("POST /admin/approve/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.ApproveUser)))
	mux.Handle("POST /admin/reject/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.RejectUser)))
	mux.Handle("GET /admin/filters", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.SecurityFilters)))
	mux.Handle("POST /admin/filters/create", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.CreateFilter)))
	mux.Handle("POST /admin/filters/toggle/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.ToggleFilter)))
	mux.Handle("DELETE /admin/filters/delete/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.DeleteFilter)))
	mux.Handle("GET /admin/logs", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.SecurityLogs)))
	mux.Handle("GET /admin/stats", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.GetStats)))
	mux.Handle("POST /admin/user/{id}/password", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.AdminChangePassword)))
	mux.Handle("POST /admin/user/{id}/toggle-admin", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.ToggleUserAdmin)))
	mux.Handle("DELETE /admin/user/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(adminHandler.DeleteUser)))

	// Knowledge Base routes
	mux.Handle("GET /knowledge", authMiddleware.RequireAuth(http.HandlerFunc(knowledgeHandler.KnowledgePage)))
	mux.Handle("POST /knowledge/submit", authMiddleware.RequireAuth(http.HandlerFunc(knowledgeHandler.SubmitKnowledge)))
	mux.Handle("GET /admin/knowledge", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.AdminKnowledgePage)))
	mux.Handle("POST /admin/knowledge/approve/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.ApproveSubmission)))
	mux.Handle("POST /admin/knowledge/reject/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.RejectSubmission)))
	mux.Handle("POST /admin/knowledge/question/{id}/answer", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.AnswerQuestion)))
	mux.Handle("POST /admin/knowledge/question/{id}/ignore", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.IgnoreQuestion)))
	mux.Handle("DELETE /admin/knowledge/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(knowledgeHandler.DeleteKnowledge)))

	handler := middleware.Logging(middleware.LanguageMiddleware(middleware.RateLimit(middleware.SecurityHeaders(mux))))

	addr := ":" + cfg.Port
	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] AQUILA - IRIS IA Chat")
	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] Servidor iniciando en puerto %s", cfg.Port)
	log.Printf("[INFO] Base de datos: %s", cfg.DBPath)
	log.Printf("[INFO] Ollama URL: %s", cfg.OllamaURL)
	log.Printf("[INFO] Modelo IA: %s", cfg.OllamaModel)
	log.Printf("[INFO] Filtros de seguridad: %v", cfg.EnableFilters)
	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] Usuario admin por defecto:")
	log.Printf("[INFO]   Nomina: admin")
	log.Printf("[INFO]   Password: admin123")
	log.Printf("[INFO] ========================================")

	if ollamaService.IsAvailable(context.Background()) {
		log.Printf("[INFO] Ollama disponible y funcionando")
	} else {
		log.Printf("[WARN] Ollama no disponible. Ejecuta: ollama run %s", cfg.OllamaModel)
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("[FATAL] Error iniciando servidor: %v", err)
	}
}

func initDB(dbPath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		return nil, err
	}

	// Siempre ejecutar schema.sql para crear tablas faltantes
	// Todas las sentencias usan IF NOT EXISTS y INSERT OR IGNORE
	log.Printf("[INFO] Verificando esquema de base de datos...")
	schemaPath := filepath.Join(".", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}

	if _, err := database.Exec(string(schema)); err != nil {
		return nil, err
	}
	log.Printf("[INFO] Esquema de base de datos verificado")

	// Siempre asegurar que exista el usuario admin
	if err := ensureAdminUser(database); err != nil {
		log.Printf("[WARN] Error asegurando usuario admin: %v", err)
	}

	return database, nil
}

// ensureAdminUser se asegura de que exista un usuario admin con las credenciales predeterminadas
func ensureAdminUser(database *sql.DB) error {
	// Verificar si existe el usuario admin
	var count int
	err := database.QueryRow("SELECT COUNT(*) FROM users WHERE nomina = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Crear usuario admin (password: admin123)
		// Hash generado con bcrypt cost 10
		_, err = database.Exec(`
			INSERT INTO users (nomina, password_hash, nombre, approved, is_admin)
			VALUES ('admin', '$2a$10$Iw6JSavGrirhkkoDsvY9leKrgEHjH913k1e8/NixaGaffrq4sWgNK', 'Administrador', 1, 1)
		`)
		if err != nil {
			return err
		}
		log.Printf("[INFO] Usuario admin creado automaticamente")
	} else {
		// Asegurar que el admin existente tenga los permisos correctos
		_, err = database.Exec("UPDATE users SET is_admin = 1, approved = 1 WHERE nomina = 'admin'")
		if err != nil {
			return err
		}
	}

	return nil
}

func loadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDate": func(t interface{}) string {
			switch v := t.(type) {
			case sql.NullTime:
				if v.Valid {
					return v.Time.Format("02/01/2006 15:04")
				}
				return ""
			case time.Time:
				return v.Format("02/01/2006 15:04")
			default:
				return ""
			}
		},
		"formatDateShort": func(t interface{}) string {
			switch v := t.(type) {
			case sql.NullTime:
				if v.Valid {
					return v.Time.Format("02/01/06")
				}
				return ""
			case time.Time:
				return v.Format("02/01/06")
			default:
				return ""
			}
		},
	}
	return template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html")
}
