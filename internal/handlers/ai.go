package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chat-empleados/db"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"
)

type AIHandler struct {
	queries   *db.Queries
	templates *template.Template
	ollama    *services.OllamaService
	security  *services.SecurityService
	scraper   *services.Scraper
}

func NewAIHandler(queries *db.Queries, templates *template.Template, ollama *services.OllamaService, security *services.SecurityService) *AIHandler {
	// Config de scraper sin browser (más rápido y confiable)
	scraperConfig := &services.ScraperConfig{
		HTTPTimeout:       15 * time.Second,
		MaxRetries:        2,
		MaxContentSize:    5 * 1024 * 1024, // 5MB
		UserAgent:         "AQUILA-Bot/1.0 (Enterprise Assistant)",
		AllowedDomains:    []string{},
		BlockedDomains:    []string{},
		RequestsPerSecond: 2.0,
		BurstSize:         5,
		CacheTTL:          15 * time.Minute,
		EnableCache:       true,
		EnableBrowser:     false, // Deshabilitado para mayor velocidad
	}

	return &AIHandler{
		queries:   queries,
		templates: templates,
		ollama:    ollama,
		security:  security,
		scraper:   services.NewScraper(scraperConfig),
	}
}

// urlRegex para detectar URLs en mensajes
var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

// extractURLs extrae todas las URLs de un texto
func extractURLs(text string) []string {
	matches := urlRegex.FindAllString(text, -1)
	// Limpiar URLs (quitar puntuación final)
	for i, url := range matches {
		url = strings.TrimRight(url, ".,;:!?)")
		matches[i] = url
	}
	return matches
}

// getKnowledgeContext obtiene el conocimiento empresarial como contexto para la IA
func (h *AIHandler) getKnowledgeContext(ctx context.Context) string {
	knowledge, err := h.queries.GetKnowledgeContext(ctx)
	if err != nil || len(knowledge) == 0 {
		return ""
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("\n\n--- CONOCIMIENTO EMPRESARIAL ---\n")
	contextBuilder.WriteString("Usa la siguiente informacion para responder preguntas sobre la empresa:\n\n")

	for _, k := range knowledge {
		contextBuilder.WriteString(fmt.Sprintf("## %s [%s]\n%s\n\n", k.Title, k.Category.String, k.Content))
	}

	contextBuilder.WriteString("--- FIN CONOCIMIENTO ---\n")
	return contextBuilder.String()
}

// enrichMessageWithURLContent extrae contenido de URLs y lo agrega al mensaje
func (h *AIHandler) enrichMessageWithURLContent(ctx context.Context, content string) string {
	urls := extractURLs(content)
	if len(urls) == 0 {
		return content
	}

	var enrichedContent strings.Builder
	enrichedContent.WriteString(content)
	enrichedContent.WriteString("\n\n--- CONTENIDO EXTRAIDO DE URLs ---\n")

	for _, url := range urls {
		log.Printf("[INFO] Extrayendo contenido de URL: %s", url)

		scraped, err := h.scraper.ScrapeForAI(ctx, url, 8000) // Max 8000 chars por URL
		if err != nil {
			log.Printf("[WARN] Error scrapeando %s: %v", url, err)
			enrichedContent.WriteString(fmt.Sprintf("\n[Error extrayendo %s: %v]\n", url, err))
			continue
		}

		enrichedContent.WriteString(scraped)
		enrichedContent.WriteString("\n")
	}

	enrichedContent.WriteString("--- FIN CONTENIDO URLs ---\n")
	return enrichedContent.String()
}

func (h *AIHandler) AIPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	conversations, err := h.queries.GetUserConversations(r.Context(), user.ID)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo conversaciones: %v", err)
	}

	var currentConv *db.AiConversation
	var messages []db.GetConversationMessagesRow
	var currentModel string

	convIDStr := r.URL.Query().Get("conv")
	if convIDStr != "" {
		convID, err := strconv.ParseInt(convIDStr, 10, 64)
		if err == nil {
			conv, err := h.queries.GetConversation(r.Context(), db.GetConversationParams{
				ID:     convID,
				UserID: user.ID,
			})
			if err == nil {
				currentConv = &conv
				messages, _ = h.queries.GetConversationMessages(r.Context(), convID)
				// Usar modelo de la conversacion o fallback al global
				if conv.Model.Valid && conv.Model.String != "" {
					currentModel = conv.Model.String
				} else {
					currentModel = h.ollama.GetModel()
				}
			}
		}
	}

	if currentModel == "" {
		currentModel = h.ollama.GetModel()
	}

	ollamaAvailable := h.ollama.IsAvailable(r.Context())

	// Obtener lista de modelos disponibles
	models, _ := h.ollama.ListModels(r.Context())

	data := TemplateData(r, map[string]interface{}{
		"Title":            Tr(r, "ai_chat"),
		"User":             user,
		"Conversations":    conversations,
		"CurrentConv":      currentConv,
		"Messages":         messages,
		"OllamaAvailable":  ollamaAvailable,
		"Model":            currentModel,
		"Models":           models,
		"GlobalModel":      h.ollama.GetModel(),
	})
	h.templates.ExecuteTemplate(w, "ai", data)
}

func (h *AIHandler) NewConversation(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	// Obtener modelo del query param o usar el global
	model := r.URL.Query().Get("model")
	if model == "" {
		model = h.ollama.GetModel()
	}

	conv, err := h.queries.CreateAIConversation(r.Context(), db.CreateAIConversationParams{
		UserID: user.ID,
		Title:  sql.NullString{String: "Nueva conversacion", Valid: true},
		Model:  sql.NullString{String: model, Valid: model != ""},
	})
	if err != nil {
		log.Printf("[ERROR] Error creando conversacion: %v", err)
		http.Error(w, "Error creando conversacion", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/ai?conv=%d", conv.ID), http.StatusSeeOther)
}

func (h *AIHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		sendAIError(w, "Error procesando formulario")
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	convIDStr := r.FormValue("conversation_id")

	if content == "" {
		sendAIError(w, "El mensaje no puede estar vacio")
		return
	}

	if len(content) > 4000 {
		sendAIError(w, "El mensaje es demasiado largo")
		return
	}

	content = h.security.SanitizeForDisplay(content)

	var convID int64
	var err error
	model := r.FormValue("model")

	if convIDStr == "" || convIDStr == "0" {
		// Usar modelo del form o el global
		if model == "" {
			model = h.ollama.GetModel()
		}
		conv, err := h.queries.CreateAIConversation(r.Context(), db.CreateAIConversationParams{
			UserID: user.ID,
			Title:  sql.NullString{String: truncateTitle(content), Valid: true},
			Model:  sql.NullString{String: model, Valid: model != ""},
		})
		if err != nil {
			log.Printf("[ERROR] Error creando conversacion: %v", err)
			sendAIError(w, "Error creando conversacion")
			return
		}
		convID = conv.ID
	} else {
		convID, err = strconv.ParseInt(convIDStr, 10, 64)
		if err != nil {
			sendAIError(w, "ID de conversacion invalido")
			return
		}

		hasAccess, err := h.security.ValidateConversationAccess(r.Context(), convID, user.ID)
		if err != nil || !hasAccess {
			log.Printf("[SECURITY] Usuario %s intento acceder a conversacion %d sin permiso", user.Nomina, convID)
			sendAIError(w, "No tienes acceso a esta conversacion")
			return
		}
	}

	_, err = h.queries.CreateAIMessage(r.Context(), db.CreateAIMessageParams{
		ConversationID: convID,
		Role:           "user",
		Content:        content,
		Filtered:       sql.NullInt64{Int64: 0, Valid: true},
		FilterReason:   sql.NullString{String: "", Valid: true},
	})
	if err != nil {
		log.Printf("[ERROR] Error guardando mensaje usuario: %v", err)
		sendAIError(w, "Error guardando mensaje")
		return
	}

	history, err := h.queries.GetConversationMessages(r.Context(), convID)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo historial: %v", err)
		sendAIError(w, "Error obteniendo historial")
		return
	}

	messages := make([]services.Message, 0, len(history)+1)

	// Agregar contexto de conocimiento empresarial
	knowledgeContext := h.getKnowledgeContext(r.Context())
	if knowledgeContext != "" {
		messages = append(messages, services.Message{
			Role:    "system",
			Content: knowledgeContext,
		})
	}

	for i, msg := range history {
		msgContent := msg.Content
		// Enriquecer el ultimo mensaje del usuario con contenido de URLs
		if i == len(history)-1 && msg.Role == "user" {
			msgContent = h.enrichMessageWithURLContent(r.Context(), msgContent)
		}
		messages = append(messages, services.Message{
			Role:    msg.Role,
			Content: msgContent,
		})
	}

	response, filterResult, err := h.ollama.Chat(r.Context(), messages, user.ID)
	if err != nil {
		log.Printf("[ERROR] Error llamando a Ollama: %v", err)
		sendAIError(w, "Error comunicando con la IA. Verifica que Ollama este ejecutandose.")
		return
	}

	if filterResult != nil && filterResult.Blocked {
		_, err = h.queries.CreateAIMessage(r.Context(), db.CreateAIMessageParams{
			ConversationID: convID,
			Role:           "assistant",
			Content:        "Lo siento, no puedo procesar esa solicitud por politicas de seguridad.",
			Filtered:       sql.NullInt64{Int64: 1, Valid: true},
			FilterReason:   sql.NullString{String: filterResult.FilterName, Valid: true},
		})
		if err != nil {
			log.Printf("[ERROR] Error guardando respuesta filtrada: %v", err)
		}

		h.security.LogViolation(
			r.Context(),
			user.ID,
			sql.NullInt64{Int64: filterResult.FilterID, Valid: true},
			content,
			filterResult.Action,
			r.RemoteAddr,
			r.UserAgent(),
		)

		sendAIResponse(w, convID, "Lo siento, no puedo procesar esa solicitud por politicas de seguridad.", true, filterResult.Reason)
		return
	}

	_, err = h.queries.CreateAIMessage(r.Context(), db.CreateAIMessageParams{
		ConversationID: convID,
		Role:           "assistant",
		Content:        response,
		Filtered:       sql.NullInt64{Int64: 0, Valid: true},
		FilterReason:   sql.NullString{String: "", Valid: true},
	})
	if err != nil {
		log.Printf("[ERROR] Error guardando respuesta IA: %v", err)
	}

	h.queries.TouchConversation(r.Context(), convID)

	sendAIResponse(w, convID, response, false, "")
}

func (h *AIHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	convIDStr := r.PathValue("id")
	convID, err := strconv.ParseInt(convIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	hasAccess, err := h.security.ValidateConversationAccess(r.Context(), convID, user.ID)
	if err != nil || !hasAccess {
		http.Error(w, "No autorizado", http.StatusForbidden)
		return
	}

	_, err = h.queries.DeleteConversation(r.Context(), db.DeleteConversationParams{
		ID:     convID,
		UserID: user.ID,
	})
	if err != nil {
		log.Printf("[ERROR] Error eliminando conversacion: %v", err)
		http.Error(w, "Error eliminando conversacion", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/ai")
	w.WriteHeader(http.StatusOK)
}

func (h *AIHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	convIDStr := r.PathValue("id")
	convID, err := strconv.ParseInt(convIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID invalido", http.StatusBadRequest)
		return
	}

	hasAccess, err := h.security.ValidateConversationAccess(r.Context(), convID, user.ID)
	if err != nil || !hasAccess {
		http.Error(w, "No autorizado", http.StatusForbidden)
		return
	}

	messages, err := h.queries.GetConversationMessages(r.Context(), convID)
	if err != nil {
		http.Error(w, "Error obteniendo mensajes", http.StatusInternalServerError)
		return
	}

	h.templates.ExecuteTemplate(w, "ai_messages", map[string]interface{}{
		"Messages": messages,
		"ConvID":   convID,
	})
}

type AIResponseData struct {
	ConversationID int64  `json:"conversation_id"`
	Response       string `json:"response"`
	Filtered       bool   `json:"filtered"`
	FilterReason   string `json:"filter_reason,omitempty"`
	Error          string `json:"error,omitempty"`
}

func sendAIResponse(w http.ResponseWriter, convID int64, response string, filtered bool, filterReason string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AIResponseData{
		ConversationID: convID,
		Response:       response,
		Filtered:       filtered,
		FilterReason:   filterReason,
	})
}

func sendAIError(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(AIResponseData{
		Error: errorMsg,
	})
}

func truncateTitle(content string) string {
	content = strings.TrimSpace(content)
	if len(content) > 50 {
		return content[:47] + "..."
	}
	return content
}

// SendMessageStream maneja el envío de mensajes con streaming SSE
func (h *AIHandler) SendMessageStream(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	// Soportar tanto application/x-www-form-urlencoded como multipart/form-data
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "Error procesando formulario", http.StatusBadRequest)
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error procesando formulario", http.StatusBadRequest)
			return
		}
	}

	content := strings.TrimSpace(r.FormValue("content"))
	convIDStr := r.FormValue("conversation_id")
	selectedModel := r.FormValue("model")

	if content == "" {
		http.Error(w, "El mensaje no puede estar vacio", http.StatusBadRequest)
		return
	}

	if len(content) > 4000 {
		http.Error(w, "El mensaje es demasiado largo", http.StatusBadRequest)
		return
	}

	content = h.security.SanitizeForDisplay(content)

	var convID int64
	var err error
	var convModel string

	if convIDStr == "" || convIDStr == "0" {
		// Nueva conversacion - usar modelo seleccionado o global
		if selectedModel == "" {
			selectedModel = h.ollama.GetModel()
		}
		convModel = selectedModel
		conv, err := h.queries.CreateAIConversation(r.Context(), db.CreateAIConversationParams{
			UserID: user.ID,
			Title:  sql.NullString{String: truncateTitle(content), Valid: true},
			Model:  sql.NullString{String: selectedModel, Valid: selectedModel != ""},
		})
		if err != nil {
			log.Printf("[ERROR] Error creando conversacion: %v", err)
			http.Error(w, "Error creando conversacion", http.StatusInternalServerError)
			return
		}
		convID = conv.ID
	} else {
		convID, err = strconv.ParseInt(convIDStr, 10, 64)
		if err != nil {
			http.Error(w, "ID de conversacion invalido", http.StatusBadRequest)
			return
		}

		hasAccess, err := h.security.ValidateConversationAccess(r.Context(), convID, user.ID)
		if err != nil || !hasAccess {
			log.Printf("[SECURITY] Usuario %s intento acceder a conversacion %d sin permiso", user.Nomina, convID)
			http.Error(w, "No tienes acceso a esta conversacion", http.StatusForbidden)
			return
		}

		// Obtener modelo de la conversacion existente
		conv, err := h.queries.GetConversationByID(r.Context(), convID)
		if err == nil && conv.Model.Valid && conv.Model.String != "" {
			convModel = conv.Model.String
		} else {
			convModel = h.ollama.GetModel() // Fallback al modelo global
		}
	}

	// Guardar mensaje del usuario
	_, err = h.queries.CreateAIMessage(r.Context(), db.CreateAIMessageParams{
		ConversationID: convID,
		Role:           "user",
		Content:        content,
		Filtered:       sql.NullInt64{Int64: 0, Valid: true},
		FilterReason:   sql.NullString{String: "", Valid: true},
	})
	if err != nil {
		log.Printf("[ERROR] Error guardando mensaje usuario: %v", err)
		http.Error(w, "Error guardando mensaje", http.StatusInternalServerError)
		return
	}

	// Obtener historial
	history, err := h.queries.GetConversationMessages(r.Context(), convID)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo historial: %v", err)
		http.Error(w, "Error obteniendo historial", http.StatusInternalServerError)
		return
	}

	messages := make([]services.Message, 0, len(history)+1)

	// Agregar contexto de conocimiento empresarial
	knowledgeContext := h.getKnowledgeContext(r.Context())
	if knowledgeContext != "" {
		messages = append(messages, services.Message{
			Role:    "system",
			Content: knowledgeContext,
		})
	}

	for _, msg := range history {
		messages = append(messages, services.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	log.Printf("[DEBUG] Iniciando streaming con %d mensajes para usuario %d, modelo: %s", len(messages), user.ID, convModel)

	// Configurar SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming no soportado", http.StatusInternalServerError)
		return
	}

	// Enviar evento inicial con ID de conversación y modelo
	log.Printf("[DEBUG] Enviando evento start con convID=%d, modelo=%s", convID, convModel)
	fmt.Fprintf(w, "data: {\"conversation_id\": %d, \"model\": \"%s\"}\n\n", convID, convModel)
	flusher.Flush()

	var fullResponse strings.Builder
	var streamErr error
	var filterResult *services.FilterResult
	streamDone := make(chan struct{})
	gotFirstChunk := false

	// Usar contexto con timeout largo (10 minutos) para modelos lentos como DeepSeek R1
	streamCtx, cancelStream := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelStream()

	log.Printf("[DEBUG] Llamando a ChatStreamWithModel con modelo: %s", convModel)

	// Iniciar streaming en goroutine
	go func() {
		defer close(streamDone)
		filterResult, streamErr = h.ollama.ChatStreamWithModel(streamCtx, messages, user.ID, convModel, func(chunk string) error {
			gotFirstChunk = true
			fullResponse.WriteString(chunk)

			// Enviar chunk como evento SSE
			data := map[string]string{"content": chunk}
			jsonData, _ := json.Marshal(data)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
			return nil
		})
	}()

	// Enviar heartbeats mientras esperamos respuesta del modelo
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-streamDone:
			// Streaming completado
			goto streamComplete
		case <-heartbeatTicker.C:
			// Enviar heartbeat solo si aun no hemos recibido chunks
			if !gotFirstChunk {
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
				log.Printf("[DEBUG] Heartbeat enviado, esperando respuesta de modelo...")
			}
		case <-r.Context().Done():
			// Cliente desconectado
			log.Printf("[DEBUG] Cliente desconectado durante streaming")
			cancelStream()
			return
		}
	}

streamComplete:
	log.Printf("[DEBUG] ChatStream terminado, err=%v", streamErr)

	if streamErr != nil {
		log.Printf("[ERROR] Error en streaming Ollama: %v", streamErr)
		fmt.Fprintf(w, "data: {\"error\": \"Error comunicando con la IA\"}\n\n")
		flusher.Flush()
		return
	}

	response := fullResponse.String()

	// Usar contexto de background para operaciones de BD (no depender del cliente)
	dbCtx := context.Background()

	// Verificar si fue filtrado
	if filterResult != nil && filterResult.Blocked {
		response = "Lo siento, no puedo procesar esa solicitud por politicas de seguridad."

		_, err = h.queries.CreateAIMessage(dbCtx, db.CreateAIMessageParams{
			ConversationID: convID,
			Role:           "assistant",
			Content:        response,
			Filtered:       sql.NullInt64{Int64: 1, Valid: true},
			FilterReason:   sql.NullString{String: filterResult.FilterName, Valid: true},
		})

		h.security.LogViolation(
			dbCtx,
			user.ID,
			sql.NullInt64{Int64: filterResult.FilterID, Valid: true},
			content,
			filterResult.Action,
			r.RemoteAddr,
			r.UserAgent(),
		)

		fmt.Fprintf(w, "event: filtered\ndata: {\"reason\": \"%s\"}\n\n", filterResult.Reason)
		flusher.Flush()
	} else {
		// Guardar respuesta normal
		_, err = h.queries.CreateAIMessage(dbCtx, db.CreateAIMessageParams{
			ConversationID: convID,
			Role:           "assistant",
			Content:        response,
			Filtered:       sql.NullInt64{Int64: 0, Valid: true},
			FilterReason:   sql.NullString{String: "", Valid: true},
		})
		if err != nil {
			log.Printf("[ERROR] Error guardando respuesta IA: %v", err)
		}
	}

	h.queries.TouchConversation(dbCtx, convID)

	// Enviar evento de finalización
	fmt.Fprintf(w, "event: done\ndata: {\"conversation_id\": %d}\n\n", convID)
	flusher.Flush()
}

// HealthCheck endpoint para verificar estado del servicio de IA
func (h *AIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := map[string]interface{}{
		"ollama_available": h.ollama.IsAvailable(ctx),
		"model":            h.ollama.GetModel(),
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ListModels devuelve la lista de modelos disponibles en Ollama
func (h *AIHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	models, err := h.ollama.ListModels(ctx)
	if err != nil {
		log.Printf("[ERROR] Error listando modelos: %v", err)
		http.Error(w, "Error obteniendo modelos", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"models":        models,
		"current_model": h.ollama.GetModel(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetModel cambia el modelo de IA actual
func (h *AIHandler) SetModel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error procesando formulario", http.StatusBadRequest)
		return
	}

	model := strings.TrimSpace(r.FormValue("model"))
	if model == "" {
		http.Error(w, "Modelo no especificado", http.StatusBadRequest)
		return
	}

	h.ollama.SetModel(model)

	user := middleware.GetUserFromContext(r.Context())
	log.Printf("[INFO] Usuario %s cambió el modelo a: %s", user.Nomina, model)

	// Si es una petición HTMX, redirigir
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin")
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"model":  model,
	})
}
