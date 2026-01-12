package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"chat-empleados/internal/config"
)

var (
	ErrOllamaUnavailable = errors.New("ollama no disponible")
	ErrMaxRetriesReached = errors.New("maximo de reintentos alcanzado")
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  *Options  `json:"options,omitempty"`
}

type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type StreamChunk struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

type OllamaService struct {
	cfg          *config.Config
	client       *http.Client
	security     *SecurityService
	available    bool
	availMutex   sync.RWMutex
	lastCheck    time.Time
	checkPeriod  time.Duration
	currentModel string
	modelMutex   sync.RWMutex
}

type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

func NewOllamaService(cfg *config.Config, security *SecurityService) *OllamaService {
	svc := &OllamaService{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.OllamaTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 10,
			},
		},
		security:     security,
		checkPeriod:  30 * time.Second,
		currentModel: cfg.OllamaModel,
	}

	// Verificar disponibilidad inicial
	go svc.checkAvailability()

	return svc
}

func (o *OllamaService) checkAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	available := o.pingOllama(ctx)

	o.availMutex.Lock()
	o.available = available
	o.lastCheck = time.Now()
	o.availMutex.Unlock()

	if available {
		log.Printf("[INFO] Ollama disponible en %s", o.cfg.OllamaURL)
	} else {
		log.Printf("[WARN] Ollama no disponible en %s", o.cfg.OllamaURL)
	}
}

func (o *OllamaService) pingOllama(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", o.cfg.OllamaURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (o *OllamaService) Chat(ctx context.Context, messages []Message, userID int64) (string, *FilterResult, error) {
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == "user" {
			if filterResult := o.security.CheckInput(ctx, lastMsg.Content); filterResult != nil {
				if filterResult.Blocked {
					log.Printf("[SECURITY] Usuario %d bloqueado por filtro '%s': %s",
						userID, filterResult.FilterName, filterResult.MatchedText)
					return "", filterResult, nil
				}
			}
		}
	}

	messagesWithSystem := make([]Message, 0, len(messages)+1)
	messagesWithSystem = append(messagesWithSystem, Message{
		Role:    "system",
		Content: o.cfg.SystemPrompt,
	})
	messagesWithSystem = append(messagesWithSystem, messages...)

	reqBody := ChatRequest{
		Model:    o.GetModel(),
		Messages: messagesWithSystem,
		Stream:   false,
		Options: &Options{
			Temperature: 0.7,
			TopP:        0.9,
			NumCtx:      4096,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("error serializando request: %w", err)
	}

	// Implementar retry con backoff exponencial
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < o.cfg.OllamaRetries; attempt++ {
		if attempt > 0 {
			// Backoff exponencial: 1s, 2s, 4s...
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[INFO] Reintentando conexion a Ollama (intento %d/%d) en %v...",
				attempt+1, o.cfg.OllamaRetries, backoff)
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", o.cfg.OllamaURL+"/api/chat", bytes.NewReader(jsonBody))
		if err != nil {
			lastErr = fmt.Errorf("error creando request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = o.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error conectando con Ollama: %w", err)
			o.markUnavailable()
			continue
		}

		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))

		// Si es un error de servidor (5xx), reintentar
		if resp.StatusCode >= 500 {
			continue
		}
		// Si es error de cliente (4xx), no reintentar
		break
	}

	if lastErr != nil {
		return "", nil, lastErr
	}
	defer resp.Body.Close()

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", nil, fmt.Errorf("error decodificando respuesta: %w", err)
	}

	response := chatResp.Message.Content
	o.markAvailable()

	if filterResult := o.security.CheckOutput(ctx, response); filterResult != nil {
		if filterResult.Blocked {
			log.Printf("[SECURITY] Respuesta IA bloqueada por filtro '%s'", filterResult.FilterName)
			return "Lo siento, no puedo proporcionar esa informacion.", filterResult, nil
		}
	}

	return response, nil, nil
}

func (o *OllamaService) markAvailable() {
	o.availMutex.Lock()
	o.available = true
	o.lastCheck = time.Now()
	o.availMutex.Unlock()
}

func (o *OllamaService) markUnavailable() {
	o.availMutex.Lock()
	o.available = false
	o.lastCheck = time.Now()
	o.availMutex.Unlock()
}

func (o *OllamaService) ChatStream(ctx context.Context, messages []Message, userID int64, onChunk func(string) error) (*FilterResult, error) {
	return o.ChatStreamWithModel(ctx, messages, userID, "", onChunk)
}

func (o *OllamaService) ChatStreamWithModel(ctx context.Context, messages []Message, userID int64, model string, onChunk func(string) error) (*FilterResult, error) {
	log.Printf("[DEBUG] ChatStream: iniciando para usuario %d", userID)

	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == "user" {
			if filterResult := o.security.CheckInput(ctx, lastMsg.Content); filterResult != nil {
				if filterResult.Blocked {
					log.Printf("[SECURITY] Usuario %d bloqueado por filtro '%s': %s",
						userID, filterResult.FilterName, filterResult.MatchedText)
					return filterResult, nil
				}
			}
		}
	}

	// Usar modelo específico o el global
	useModel := model
	if useModel == "" {
		useModel = o.GetModel()
	}

	messagesWithSystem := make([]Message, 0, len(messages)+1)
	messagesWithSystem = append(messagesWithSystem, Message{
		Role:    "system",
		Content: o.cfg.SystemPrompt,
	})
	messagesWithSystem = append(messagesWithSystem, messages...)

	reqBody := ChatRequest{
		Model:    useModel,
		Messages: messagesWithSystem,
		Stream:   true,
		Options: &Options{
			Temperature: 0.7,
			TopP:        0.9,
			NumCtx:      4096,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error serializando request: %w", err)
	}

	log.Printf("[DEBUG] ChatStream: enviando request a Ollama...")

	// Cliente especial para streaming sin timeout global
	streamClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
			MaxIdleConnsPerHost: 10,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.cfg.OllamaURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creando request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := streamClient.Do(req)
	if err != nil {
		log.Printf("[DEBUG] ChatStream: error conectando: %v", err)
		return nil, fmt.Errorf("error conectando con Ollama: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[DEBUG] ChatStream: respuesta recibida, status=%d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		content := chunk.Message.Content
		fullResponse.WriteString(content)

		if err := onChunk(content); err != nil {
			return nil, err
		}

		if chunk.Done {
			break
		}
	}

	finalResponse := fullResponse.String()
	if filterResult := o.security.CheckOutput(ctx, finalResponse); filterResult != nil {
		if filterResult.Blocked {
			log.Printf("[SECURITY] Respuesta IA streaming bloqueada por filtro '%s'", filterResult.FilterName)
			return filterResult, nil
		}
	}

	return nil, scanner.Err()
}

func (o *OllamaService) IsAvailable(ctx context.Context) bool {
	o.availMutex.RLock()
	lastCheck := o.lastCheck
	available := o.available
	o.availMutex.RUnlock()

	// Si la última verificación fue hace menos del período de check, usar el valor cacheado
	if time.Since(lastCheck) < o.checkPeriod {
		return available
	}

	// Verificar de nuevo
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	available = o.pingOllama(ctx)

	o.availMutex.Lock()
	o.available = available
	o.lastCheck = time.Now()
	o.availMutex.Unlock()

	return available
}

func (o *OllamaService) GetModel() string {
	o.modelMutex.RLock()
	defer o.modelMutex.RUnlock()
	return o.currentModel
}

func (o *OllamaService) SetModel(model string) {
	o.modelMutex.Lock()
	defer o.modelMutex.Unlock()
	o.currentModel = model
	log.Printf("[INFO] Modelo de IA cambiado a: %s", model)
}

func (o *OllamaService) ListModels(ctx context.Context) ([]OllamaModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.cfg.OllamaURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error listing models: %d", resp.StatusCode)
	}

	var result OllamaModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Models, nil
}
