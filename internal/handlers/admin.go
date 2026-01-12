package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"chat-empleados/db"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"

	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	queries       *db.Queries
	templates     *template.Template
	security      *services.SecurityService
	notifications *services.NotificationService
}

func NewAdminHandler(queries *db.Queries, templates *template.Template, security *services.SecurityService, notifications *services.NotificationService) *AdminHandler {
	return &AdminHandler{
		queries:       queries,
		templates:     templates,
		security:      security,
		notifications: notifications,
	}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	stats, err := h.queries.GetDashboardStats(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo estadisticas: %v", err)
	}

	pendingUsers, err := h.queries.GetPendingUsers(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo usuarios pendientes: %v", err)
	}

	recentLogs, err := h.queries.GetRecentSecurityLogs(r.Context(), 10)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo logs de seguridad: %v", err)
	}

	filterCount, _ := h.queries.CountActiveFilters(r.Context())

	data := TemplateData(r, map[string]interface{}{
		"Title":        Tr(r, "admin_panel"),
		"User":         user,
		"Stats":        stats,
		"PendingUsers": pendingUsers,
		"RecentLogs":   recentLogs,
		"FilterCount":  filterCount,
	})
	h.templates.ExecuteTemplate(w, "admin", data)
}

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	allUsers, err := h.queries.GetAllUsers(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo usuarios: %v", err)
	}

	pendingUsers, err := h.queries.GetPendingUsers(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo usuarios pendientes: %v", err)
	}

	data := map[string]interface{}{
		"Title":        "Gestion de Usuarios",
		"User":         user,
		"AllUsers":     allUsers,
		"PendingUsers": pendingUsers,
	}
	h.templates.ExecuteTemplate(w, "admin_users", data)
}

func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	result, err := h.queries.ApproveUser(r.Context(), userID)
	if err != nil {
		log.Printf("[ERROR] Error aprobando usuario: %v", err)
		http.Error(w, "Error aprobando usuario", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Usuario no encontrado o ya aprobado", http.StatusNotFound)
		return
	}

	user, _ := h.queries.GetUserByID(r.Context(), userID)
	log.Printf("[INFO] Usuario aprobado: %s (%s)", user.Nombre, user.Nomina)

	// Notificar al usuario que fue aprobado
	go h.notifications.NotifyUserApproved(context.Background(), userID)

	w.Header().Set("HX-Trigger", "userUpdated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Usuario aprobado"))
}

func (h *AdminHandler) RejectUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	result, err := h.queries.RejectUser(r.Context(), userID)
	if err != nil {
		log.Printf("[ERROR] Error rechazando usuario: %v", err)
		http.Error(w, "Error rechazando usuario", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Usuario no encontrado o no puede ser rechazado", http.StatusNotFound)
		return
	}

	log.Printf("[INFO] Usuario rechazado: %s (%s)", user.Nombre, user.Nomina)

	w.Header().Set("HX-Trigger", "userUpdated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Usuario rechazado"))
}

func (h *AdminHandler) SecurityFilters(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	filters, err := h.queries.GetAllSecurityFilters(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo filtros: %v", err)
	}

	categories, err := h.queries.GetFilterCategories(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo categorias: %v", err)
	}

	stats, _ := h.security.GetFilterStats(r.Context())

	data := map[string]interface{}{
		"Title":      "Filtros de Seguridad",
		"User":       user,
		"Filters":    filters,
		"Categories": categories,
		"Stats":      stats,
	}
	h.templates.ExecuteTemplate(w, "admin_filters", data)
}

func (h *AdminHandler) CreateFilter(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	filterType := r.FormValue("filter_type")
	pattern := strings.TrimSpace(r.FormValue("pattern"))
	action := r.FormValue("action")
	appliesTo := r.FormValue("applies_to")
	severity := r.FormValue("severity")

	if name == "" || pattern == "" {
		http.Error(w, "Nombre y patron son requeridos", http.StatusBadRequest)
		return
	}

	_, err := h.queries.CreateSecurityFilter(r.Context(), db.CreateSecurityFilterParams{
		Name:        name,
		Description: sql.NullString{String: description, Valid: description != ""},
		FilterType:  filterType,
		Pattern:     pattern,
		Action:      action,
		AppliesTo:   sql.NullString{String: appliesTo, Valid: appliesTo != ""},
		Severity:    sql.NullString{String: severity, Valid: severity != ""},
		CreatedBy:   sql.NullInt64{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("[ERROR] Error creando filtro: %v", err)
		http.Error(w, "Error creando filtro", http.StatusInternalServerError)
		return
	}

	h.security.ReloadFilters(r.Context())
	log.Printf("[INFO] Filtro creado por %s: %s", user.Nomina, name)

	w.Header().Set("HX-Redirect", "/admin/filters")
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) ToggleFilter(w http.ResponseWriter, r *http.Request) {
	filterIDStr := r.PathValue("id")
	filterID, err := strconv.ParseInt(filterIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	filter, err := h.queries.GetSecurityFilterByID(r.Context(), filterID)
	if err != nil {
		http.Error(w, "Filtro no encontrado", http.StatusNotFound)
		return
	}

	newStatus := sql.NullInt64{Int64: 1, Valid: true}
	if filter.IsActive.Valid && filter.IsActive.Int64 == 1 {
		newStatus = sql.NullInt64{Int64: 0, Valid: true}
	}

	_, err = h.queries.ToggleSecurityFilter(r.Context(), db.ToggleSecurityFilterParams{
		IsActive: newStatus,
		ID:       filterID,
	})
	if err != nil {
		log.Printf("[ERROR] Error actualizando filtro: %v", err)
		http.Error(w, "Error actualizando filtro", http.StatusInternalServerError)
		return
	}

	h.security.ReloadFilters(r.Context())

	user := middleware.GetUserFromContext(r.Context())
	status := "activado"
	if newStatus.Int64 == 0 {
		status = "desactivado"
	}
	log.Printf("[INFO] Filtro %s %s por %s", filter.Name, status, user.Nomina)

	w.Header().Set("HX-Trigger", "filterUpdated")
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) DeleteFilter(w http.ResponseWriter, r *http.Request) {
	filterIDStr := r.PathValue("id")
	filterID, err := strconv.ParseInt(filterIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	filter, err := h.queries.GetSecurityFilterByID(r.Context(), filterID)
	if err != nil {
		http.Error(w, "Filtro no encontrado", http.StatusNotFound)
		return
	}

	_, err = h.queries.DeleteSecurityFilter(r.Context(), filterID)
	if err != nil {
		log.Printf("[ERROR] Error eliminando filtro: %v", err)
		http.Error(w, "Error eliminando filtro", http.StatusInternalServerError)
		return
	}

	h.security.ReloadFilters(r.Context())

	user := middleware.GetUserFromContext(r.Context())
	log.Printf("[INFO] Filtro eliminado por %s: %s", user.Nomina, filter.Name)

	w.Header().Set("HX-Trigger", "filterUpdated")
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) SecurityLogs(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	logs, err := h.queries.GetRecentSecurityLogs(r.Context(), 100)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo logs: %v", err)
	}

	stats, _ := h.security.GetFilterStats(r.Context())

	data := map[string]interface{}{
		"Title": "Logs de Seguridad",
		"User":  user,
		"Logs":  logs,
		"Stats": stats,
	}
	h.templates.ExecuteTemplate(w, "admin_logs", data)
}

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.queries.GetDashboardStats(r.Context())
	if err != nil {
		http.Error(w, "Error obteniendo estadisticas", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *AdminHandler) AdminChangePassword(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	newPassword := r.FormValue("new_password")
	if newPassword == "" || len(newPassword) < 6 {
		http.Error(w, "La contraseña debe tener al menos 6 caracteres", http.StatusBadRequest)
		return
	}

	targetUser, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error procesando contraseña", http.StatusInternalServerError)
		return
	}

	_, err = h.queries.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		PasswordHash: string(newHash),
		ID:           userID,
	})
	if err != nil {
		http.Error(w, "Error actualizando contraseña", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Admin %s cambió la contraseña del usuario %s (%s)", adminUser.Nomina, targetUser.Nombre, targetUser.Nomina)

	w.Header().Set("HX-Trigger", "passwordChanged")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Contraseña actualizada"))
}

func (h *AdminHandler) ToggleUserAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	targetUser, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	// No permitir quitar admin al usuario admin principal
	if targetUser.Nomina == "admin" {
		http.Error(w, "No se puede modificar el usuario admin principal", http.StatusForbidden)
		return
	}

	// Toggle admin status
	newStatus := int64(1)
	if targetUser.IsAdmin.Valid && targetUser.IsAdmin.Int64 == 1 {
		newStatus = 0
	}

	_, err = h.queries.SetUserAdmin(r.Context(), db.SetUserAdminParams{
		IsAdmin: newStatus,
		ID:      userID,
	})
	if err != nil {
		log.Printf("[ERROR] Error actualizando admin status: %v", err)
		http.Error(w, "Error actualizando usuario", http.StatusInternalServerError)
		return
	}

	status := "otorgado"
	if newStatus == 0 {
		status = "revocado"
	}
	log.Printf("[INFO] Admin %s %s rol admin a %s (%s)", adminUser.Nomina, status, targetUser.Nombre, targetUser.Nomina)

	w.Header().Set("HX-Redirect", "/admin/users")
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	userIDStr := r.PathValue("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	targetUser, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	// No permitir eliminar al usuario admin principal
	if targetUser.Nomina == "admin" {
		http.Error(w, "No se puede eliminar el usuario admin principal", http.StatusForbidden)
		return
	}

	// Primero eliminar sesiones del usuario
	h.queries.DeleteUserSessions(r.Context(), userID)

	// Luego eliminar el usuario
	result, err := h.queries.DeleteUser(r.Context(), userID)
	if err != nil {
		log.Printf("[ERROR] Error eliminando usuario: %v", err)
		http.Error(w, "Error eliminando usuario", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "No se pudo eliminar el usuario", http.StatusBadRequest)
		return
	}

	log.Printf("[INFO] Admin %s eliminó usuario %s (%s)", adminUser.Nomina, targetUser.Nombre, targetUser.Nomina)

	w.Header().Set("HX-Redirect", "/admin/users")
	w.WriteHeader(http.StatusOK)
}
