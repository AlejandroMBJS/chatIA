package handlers

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"chat-empleados/db"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"
)

type KnowledgeHandler struct {
	queries       *db.Queries
	templates     *template.Template
	notifications *services.NotificationService
}

func NewKnowledgeHandler(queries *db.Queries, templates *template.Template, notifications *services.NotificationService) *KnowledgeHandler {
	return &KnowledgeHandler{
		queries:       queries,
		templates:     templates,
		notifications: notifications,
	}
}

// KnowledgePage muestra la pagina de base de conocimiento para empleados
func (h *KnowledgeHandler) KnowledgePage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	// Obtener conocimiento aprobado
	knowledge, err := h.queries.GetActiveKnowledge(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo conocimiento: %v", err)
	}

	// Obtener envios del usuario
	submissions, err := h.queries.GetSubmissionsByUser(r.Context(), user.ID)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo envios: %v", err)
	}

	// Para admins, mostrar contador de pendientes
	var pendingCount int64
	if user.IsAdmin {
		pendingCount, _ = h.queries.CountPendingSubmissions(r.Context())
	}

	data := TemplateData(r, map[string]interface{}{
		"Title":        "Base de Conocimiento",
		"User":         user,
		"Knowledge":    knowledge,
		"Submissions":  submissions,
		"PendingCount": pendingCount,
	})
	h.templates.ExecuteTemplate(w, "knowledge", data)
}

// SubmitKnowledge permite a empleados enviar conocimiento para revision (o auto-aprobar si es admin)
func (h *KnowledgeHandler) SubmitKnowledge(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	category := strings.TrimSpace(r.FormValue("category"))

	if title == "" || content == "" {
		http.Error(w, "Titulo y contenido son requeridos", http.StatusBadRequest)
		return
	}

	if category == "" {
		category = "general"
	}

	// Si es admin, agregar directamente al conocimiento aprobado
	if user.IsAdmin {
		_, err := h.queries.CreateKnowledge(r.Context(), db.CreateKnowledgeParams{
			Title:       title,
			Content:     content,
			Category:    sql.NullString{String: category, Valid: true},
			SubmittedBy: user.ID,
			ApprovedBy:  sql.NullInt64{Int64: user.ID, Valid: true},
		})
		if err != nil {
			log.Printf("[ERROR] Error creando conocimiento (admin): %v", err)
			http.Error(w, "Error agregando conocimiento", http.StatusInternalServerError)
			return
		}

		log.Printf("[INFO] Conocimiento agregado por admin %s: %s", user.Nomina, title)

		w.Header().Set("HX-Redirect", "/knowledge")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Para empleados normales, crear envio para revision
	_, err := h.queries.CreateKnowledgeSubmission(r.Context(), db.CreateKnowledgeSubmissionParams{
		Title:       title,
		Content:     content,
		Category:    sql.NullString{String: category, Valid: true},
		SubmittedBy: user.ID,
	})
	if err != nil {
		log.Printf("[ERROR] Error creando envio de conocimiento: %v", err)
		http.Error(w, "Error enviando conocimiento", http.StatusInternalServerError)
		return
	}

	// Notificar a admins
	go h.notifications.NotifyAdminsKnowledgeSubmission(context.Background(), user.Nombre, title)

	log.Printf("[INFO] Nuevo envio de conocimiento de %s: %s", user.Nomina, title)

	w.Header().Set("HX-Redirect", "/knowledge")
	w.WriteHeader(http.StatusOK)
}

// AdminKnowledgePage muestra la pagina de gestion de conocimiento para admins
func (h *KnowledgeHandler) AdminKnowledgePage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	// Obtener envios pendientes
	pendingSubmissions, err := h.queries.GetPendingSubmissions(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo envios pendientes: %v", err)
	}

	// Obtener todo el conocimiento
	allKnowledge, err := h.queries.GetAllKnowledge(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo conocimiento: %v", err)
	}

	// Obtener preguntas sin respuesta
	pendingQuestions, err := h.queries.GetPendingQuestions(r.Context())
	if err != nil {
		log.Printf("[ERROR] Error obteniendo preguntas: %v", err)
	}

	// Contar pendientes
	pendingCount, _ := h.queries.CountPendingSubmissions(r.Context())
	questionsCount, _ := h.queries.CountPendingQuestions(r.Context())

	data := TemplateData(r, map[string]interface{}{
		"Title":              "Gestion de Conocimiento",
		"User":               user,
		"PendingSubmissions": pendingSubmissions,
		"AllKnowledge":       allKnowledge,
		"PendingQuestions":   pendingQuestions,
		"PendingCount":       pendingCount,
		"QuestionsCount":     questionsCount,
	})
	h.templates.ExecuteTemplate(w, "admin_knowledge", data)
}

// ApproveSubmission aprueba un envio y lo agrega al conocimiento
func (h *KnowledgeHandler) ApproveSubmission(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	submissionIDStr := r.PathValue("id")
	submissionID, err := strconv.ParseInt(submissionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	// Obtener el envio
	submission, err := h.queries.GetSubmissionByID(r.Context(), submissionID)
	if err != nil {
		http.Error(w, "Envio no encontrado", http.StatusNotFound)
		return
	}

	adminNotes := r.FormValue("admin_notes")

	// Aprobar el envio
	_, err = h.queries.ApproveSubmission(r.Context(), db.ApproveSubmissionParams{
		ReviewedBy: sql.NullInt64{Int64: adminUser.ID, Valid: true},
		AdminNotes: sql.NullString{String: adminNotes, Valid: adminNotes != ""},
		ID:         submissionID,
	})
	if err != nil {
		log.Printf("[ERROR] Error aprobando envio: %v", err)
		http.Error(w, "Error aprobando envio", http.StatusInternalServerError)
		return
	}

	// Agregar al conocimiento aprobado
	_, err = h.queries.CreateKnowledge(r.Context(), db.CreateKnowledgeParams{
		Title:       submission.Title,
		Content:     submission.Content,
		Category:    submission.Category,
		SubmittedBy: submission.SubmittedBy,
		ApprovedBy:  sql.NullInt64{Int64: adminUser.ID, Valid: true},
	})
	if err != nil {
		log.Printf("[ERROR] Error creando conocimiento: %v", err)
	}

	log.Printf("[INFO] Conocimiento aprobado por %s: %s", adminUser.Nomina, submission.Title)

	w.Header().Set("HX-Trigger", "knowledgeUpdated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Conocimiento aprobado"))
}

// RejectSubmission rechaza un envio de conocimiento
func (h *KnowledgeHandler) RejectSubmission(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	submissionIDStr := r.PathValue("id")
	submissionID, err := strconv.ParseInt(submissionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	adminNotes := r.FormValue("admin_notes")

	_, err = h.queries.RejectSubmission(r.Context(), db.RejectSubmissionParams{
		ReviewedBy: sql.NullInt64{Int64: adminUser.ID, Valid: true},
		AdminNotes: sql.NullString{String: adminNotes, Valid: adminNotes != ""},
		ID:         submissionID,
	})
	if err != nil {
		log.Printf("[ERROR] Error rechazando envio: %v", err)
		http.Error(w, "Error rechazando envio", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Conocimiento rechazado por %s: ID %d", adminUser.Nomina, submissionID)

	w.Header().Set("HX-Trigger", "knowledgeUpdated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Conocimiento rechazado"))
}

// AnswerQuestion responde una pregunta sin respuesta
func (h *KnowledgeHandler) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	questionIDStr := r.PathValue("id")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	answer := strings.TrimSpace(r.FormValue("answer"))
	addToKnowledge := r.FormValue("add_to_knowledge") == "on" || r.FormValue("add_to_knowledge") == "true"

	if answer == "" {
		http.Error(w, "La respuesta es requerida", http.StatusBadRequest)
		return
	}

	// Obtener la pregunta
	question, err := h.queries.GetQuestionByID(r.Context(), questionID)
	if err != nil {
		http.Error(w, "Pregunta no encontrada", http.StatusNotFound)
		return
	}

	addKnowledgeInt := int64(0)
	if addToKnowledge {
		addKnowledgeInt = 1
	}

	_, err = h.queries.AnswerQuestion(r.Context(), db.AnswerQuestionParams{
		Answer:         sql.NullString{String: answer, Valid: true},
		AnsweredBy:     sql.NullInt64{Int64: adminUser.ID, Valid: true},
		AddToKnowledge: sql.NullInt64{Int64: addKnowledgeInt, Valid: true},
		ID:             questionID,
	})
	if err != nil {
		log.Printf("[ERROR] Error respondiendo pregunta: %v", err)
		http.Error(w, "Error respondiendo pregunta", http.StatusInternalServerError)
		return
	}

	// Si se debe agregar al conocimiento, crear entrada
	if addToKnowledge {
		_, err = h.queries.CreateKnowledge(r.Context(), db.CreateKnowledgeParams{
			Title:       "Pregunta: " + truncateString(question.Question, 50),
			Content:     "Pregunta: " + question.Question + "\n\nRespuesta: " + answer,
			Category:    sql.NullString{String: "preguntas_frecuentes", Valid: true},
			SubmittedBy: question.AskedBy,
			ApprovedBy:  sql.NullInt64{Int64: adminUser.ID, Valid: true},
		})
		if err != nil {
			log.Printf("[ERROR] Error agregando pregunta al conocimiento: %v", err)
		}
	}

	log.Printf("[INFO] Pregunta respondida por %s: ID %d", adminUser.Nomina, questionID)

	w.Header().Set("HX-Trigger", "questionAnswered")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Pregunta respondida"))
}

// IgnoreQuestion ignora una pregunta sin respuesta
func (h *KnowledgeHandler) IgnoreQuestion(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	questionIDStr := r.PathValue("id")
	questionID, err := strconv.ParseInt(questionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	_, err = h.queries.IgnoreQuestion(r.Context(), db.IgnoreQuestionParams{
		AnsweredBy: sql.NullInt64{Int64: adminUser.ID, Valid: true},
		ID:         questionID,
	})
	if err != nil {
		log.Printf("[ERROR] Error ignorando pregunta: %v", err)
		http.Error(w, "Error ignorando pregunta", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Pregunta ignorada por %s: ID %d", adminUser.Nomina, questionID)

	w.Header().Set("HX-Trigger", "questionIgnored")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Pregunta ignorada"))
}

// DeleteKnowledge elimina una entrada del conocimiento
func (h *KnowledgeHandler) DeleteKnowledge(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r.Context())

	knowledgeIDStr := r.PathValue("id")
	knowledgeID, err := strconv.ParseInt(knowledgeIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	_, err = h.queries.DeleteKnowledge(r.Context(), knowledgeID)
	if err != nil {
		log.Printf("[ERROR] Error eliminando conocimiento: %v", err)
		http.Error(w, "Error eliminando conocimiento", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Conocimiento eliminado por %s: ID %d", adminUser.Nomina, knowledgeID)

	w.Header().Set("HX-Trigger", "knowledgeDeleted")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Conocimiento eliminado"))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
