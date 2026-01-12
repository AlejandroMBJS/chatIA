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

	"chat-empleados/db"
	"chat-empleados/internal/config"
	"chat-empleados/internal/handlers"
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
	chatHandler := handlers.NewChatHandler(queries, templates, securityService)
	aiHandler := handlers.NewAIHandler(queries, templates, ollamaService, securityService)
	adminHandler := handlers.NewAdminHandler(queries, templates, securityService, notificationService)

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

	mux.Handle("GET /", authMiddleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
	})))

	mux.Handle("GET /chat", authMiddleware.RequireAuth(http.HandlerFunc(chatHandler.ChatPage)))
	mux.Handle("GET /chat/ws", authMiddleware.RequireAuth(http.HandlerFunc(chatHandler.WebSocket)))

	mux.Handle("GET /ai", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.AIPage)))
	mux.Handle("GET /ai/new", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.NewConversation)))
	mux.Handle("POST /ai/send", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.SendMessage)))
	mux.Handle("POST /ai/stream", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.SendMessageStream)))
	mux.Handle("DELETE /ai/conversation/{id}", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.DeleteConversation)))
	mux.Handle("GET /ai/conversation/{id}/messages", authMiddleware.RequireAuth(http.HandlerFunc(aiHandler.GetConversationMessages)))
	mux.Handle("GET /ai/health", http.HandlerFunc(aiHandler.HealthCheck))

	mux.Handle("GET /profile", authMiddleware.RequireAuth(http.HandlerFunc(authHandler.Profile)))

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

	handler := middleware.Logging(middleware.RateLimit(middleware.SecurityHeaders(mux)))

	addr := ":" + cfg.Port
	log.Printf("[INFO] ========================================")
	log.Printf("[INFO] GIA Chat - Sistema de Comunicacion")
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
	needsInit := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		needsInit = true
	}

	database, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		return nil, err
	}

	if needsInit {
		log.Printf("[INFO] Inicializando base de datos...")
		schemaPath := filepath.Join(".", "schema.sql")
		schema, err := os.ReadFile(schemaPath)
		if err != nil {
			return nil, err
		}

		if _, err := database.Exec(string(schema)); err != nil {
			return nil, err
		}
		log.Printf("[INFO] Base de datos inicializada correctamente")
	}

	return database, nil
}

func loadTemplates() (*template.Template, error) {
	return template.ParseFS(templatesFS, "templates/*.html")
}
