package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"chat-empleados/db"
	"chat-empleados/internal/middleware"
	"chat-empleados/internal/services"

	"github.com/gorilla/websocket"
)

type ChatHandler struct {
	queries   *db.Queries
	templates *template.Template
	hub       *Hub
	security  *services.SecurityService
}

func NewChatHandler(queries *db.Queries, templates *template.Template, security *services.SecurityService) *ChatHandler {
	hub := NewHub()
	go hub.Run()

	return &ChatHandler{
		queries:   queries,
		templates: templates,
		hub:       hub,
		security:  security,
	}
}

func (h *ChatHandler) ChatPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())

	messages, err := h.queries.GetRecentGroupMessages(r.Context(), 50)
	if err != nil {
		log.Printf("[ERROR] Error obteniendo mensajes: %v", err)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	data := TemplateData(r, map[string]interface{}{
		"Title":    Tr(r, "group_chat"),
		"User":     user,
		"Messages": messages,
	})
	h.templates.ExecuteTemplate(w, "chat", data)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Permitir conexiones sin Origin (ej: curl, móviles)
		}
		// Validar que el origen coincida con el host del request
		host := r.Host
		// Aceptar localhost en desarrollo y el host actual
		validOrigins := []string{
			"http://" + host,
			"https://" + host,
			"http://localhost:9999",
			"http://127.0.0.1:9999",
		}
		for _, valid := range validOrigins {
			if origin == valid {
				return true
			}
		}
		log.Printf("[SECURITY] WebSocket origin rechazado: %s (host: %s)", origin, host)
		return false
	},
}

func (h *ChatHandler) WebSocket(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "No autorizado", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] Error upgrading websocket: %v", err)
		return
	}

	client := &Client{
		hub:    h.hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: user.ID,
		nombre: user.Nombre,
		nomina: user.Nomina,
	}
	h.hub.register <- client

	go client.writePump()
	go client.readPump(h.queries, h.security)
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("[INFO] Cliente conectado: %s", client.nombre)

			msg := ChatMessage{
				Type:      "system",
				Content:   client.nombre + " se ha conectado",
				Timestamp: time.Now().Format("15:04"),
			}
			data, _ := json.Marshal(msg)
			h.broadcastToAll(data)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			log.Printf("[INFO] Cliente desconectado: %s", client.nombre)

			msg := ChatMessage{
				Type:      "system",
				Content:   client.nombre + " se ha desconectado",
				Timestamp: time.Now().Format("15:04"),
			}
			data, _ := json.Marshal(msg)
			h.broadcastToAll(data)

		case message := <-h.broadcast:
			h.broadcastToAll(message)
		}
	}
}

func (h *Hub) broadcastToAll(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, client)
		}
	}
}

func (h *Hub) GetOnlineCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID int64
	nombre string
	nomina string
}

type ChatMessage struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	UserID    int64  `json:"user_id,omitempty"`
	Nombre    string `json:"nombre,omitempty"`
	Nomina    string `json:"nomina,omitempty"`
	Timestamp string `json:"timestamp"`
	Blocked   bool   `json:"blocked,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type IncomingMessage struct {
	Content string `json:"content"`
}

func (c *Client) readPump(queries *db.Queries, security *services.SecurityService) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ERROR] WebSocket error: %v", err)
			}
			break
		}

		var incoming IncomingMessage
		if err := json.Unmarshal(message, &incoming); err != nil {
			continue
		}

		content := incoming.Content
		if len(content) == 0 || len(content) > 2000 {
			continue
		}

		content = security.SanitizeForDisplay(content)

		if filterResult := security.CheckInput(nil, content); filterResult != nil {
			if filterResult.Blocked {
				errorMsg := ChatMessage{
					Type:      "error",
					Content:   "Tu mensaje fue bloqueado: " + filterResult.Reason,
					Timestamp: time.Now().Format("15:04"),
					Blocked:   true,
					Reason:    filterResult.FilterName,
				}
				data, _ := json.Marshal(errorMsg)
				c.send <- data

				log.Printf("[SECURITY] Mensaje grupal bloqueado de %s: %s", c.nomina, filterResult.FilterName)
				continue
			}
		}

		_, err = queries.CreateGroupMessage(nil, db.CreateGroupMessageParams{
			UserID:  c.userID,
			Content: content,
		})
		if err != nil {
			log.Printf("[ERROR] Error guardando mensaje: %v", err)
			continue
		}

		chatMsg := ChatMessage{
			Type:      "message",
			Content:   content,
			UserID:    c.userID,
			Nombre:    c.nombre,
			Nomina:    c.nomina,
			Timestamp: time.Now().Format("15:04"),
		}
		data, _ := json.Marshal(chatMsg)
		c.hub.broadcast <- data
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
