# Contexto del Proyecto para IA

## Resumen Ejecutivo

Aplicación web en Go para empresa manufacturera. Dos funcionalidades principales:
1. Chat grupal entre empleados (WebSocket, tiempo real)
2. Chat privado con IA (DeepSeek R1 via Ollama)

## Stack

- Backend: Go 1.23+, net/http stdlib, gorilla/websocket
- DB: SQLite + SQLC
- Frontend: HTML + HTMX + CSS (sin frameworks JS)
- IA: Ollama localhost:11434, modelo deepseek-r1:14b

## Arquitectura

### Base de Datos (SQLite)

5 tablas:
- `users`: id, nomina, password_hash, nombre, approved, is_admin, created_at
- `sessions`: id, user_id, token, expires_at
- `group_messages`: id, user_id, content, created_at
- `ai_conversations`: id, user_id, title, created_at, updated_at
- `ai_messages`: id, conversation_id, role, content, created_at

### Flujo de Autenticación

1. Usuario se registra con nómina + password + nombre
2. Cuenta queda en estado `approved=0`
3. Admin aprueba/rechaza desde panel
4. Usuario aprobado puede hacer login
5. Login crea sesión con token en cookie
6. Middleware valida token en rutas protegidas

### Chat Grupal (WebSocket)

- Hub central mantiene conexiones activas
- Cliente conecta a /chat/ws
- Mensaje enviado -> broadcast a todos
- Mensajes se persisten en group_messages

### Chat IA

- Cada usuario tiene múltiples conversaciones
- Historial se guarda en ai_messages
- Streaming de respuestas via SSE
- Contexto completo se envía a Ollama en cada request

## Estructura de Archivos

```
main.go                     -> Entry point, rutas, DI
internal/config/config.go   -> Variables de entorno, configuración
internal/middleware/auth.go -> Validación de sesión
internal/handlers/auth.go   -> Login, register, logout
internal/handlers/chat.go   -> Chat grupal HTTP
internal/handlers/hub.go    -> WebSocket hub
internal/handlers/ai.go     -> Chat con IA
internal/handlers/admin.go  -> Panel administración
internal/services/ollama.go -> Cliente Ollama
db/                         -> Código generado por SQLC
templates/                  -> HTML templates
static/                     -> CSS, HTMX
```

## Convenciones de Código

### Manejo de Errores
```go
if err != nil {
    return fmt.Errorf("contexto: %w", err)
}
```

### Logging
```go
log.Printf("[INFO] mensaje %s", variable)
log.Printf("[ERROR] fallo: %v", err)
```

### Tests
- Table-driven tests
- Mocks para Ollama
- Tests de integración en main_test.go

## Dependencias Externas

```
github.com/gorilla/websocket  -> WebSocket
github.com/mattn/go-sqlite3   -> SQLite driver
golang.org/x/crypto/bcrypt    -> Hashing passwords
```

## Variables de Entorno (Opcionales)

```
PORT=8080
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=deepseek-r1:14b
DB_PATH=chat.db
SESSION_DURATION=24h
```

## Comandos de Desarrollo

```bash
make check    # Verificar que compila
make test     # Ejecutar tests
make dev      # Ejecutar servidor
make sqlc     # Regenerar código DB
make reset-db # Recrear base de datos
```

## Estado de Implementación

- [x] Setup proyecto (go.mod, Makefile)
- [x] Schema base de datos (schema.sql)
- [ ] Queries SQLC (queries.sql) ← SIGUIENTE PASO
- [ ] sqlc.yaml + generar código
- [ ] Configuración
- [ ] Middleware auth
- [ ] Handlers auth
- [ ] Templates base
- [ ] Chat grupal + WebSocket
- [ ] Servicio Ollama
- [ ] Chat IA
- [ ] Panel admin
- [ ] Tests integración

---

## CONTINUAR AQUÍ: Paso 2 - queries.sql

Crear archivo `queries.sql` en la raíz con este contenido:

```sql
-- ============ USERS ============

-- name: CreateUser :one
INSERT INTO users (nomina, password_hash, nombre)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetUserByNomina :one
SELECT * FROM users WHERE nomina = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetPendingUsers :many
SELECT id, nomina, nombre, created_at
FROM users
WHERE approved = 0 AND is_admin = 0
ORDER BY created_at DESC;

-- name: GetApprovedUsers :many
SELECT id, nomina, nombre, created_at
FROM users
WHERE approved = 1 AND is_admin = 0
ORDER BY nombre ASC;

-- name: ApproveUser :execresult
UPDATE users SET approved = 1 WHERE id = ? AND approved = 0;

-- name: RejectUser :execresult
DELETE FROM users WHERE id = ? AND approved = 0 AND is_admin = 0;

-- ============ SESSIONS ============

-- name: CreateSession :one
INSERT INTO sessions (user_id, token, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetSessionByToken :one
SELECT
    s.id, s.user_id, s.token, s.expires_at,
    u.nomina, u.nombre, u.is_admin, u.approved
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token = ? AND s.expires_at > datetime('now');

-- name: DeleteSession :execresult
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :execresult
DELETE FROM sessions WHERE expires_at <= datetime('now');

-- name: DeleteUserSessions :execresult
DELETE FROM sessions WHERE user_id = ?;

-- ============ GROUP CHAT ============

-- name: CreateGroupMessage :one
INSERT INTO group_messages (user_id, content)
VALUES (?, ?)
RETURNING *;

-- name: GetRecentGroupMessages :many
SELECT
    gm.id, gm.content, gm.created_at,
    u.id as user_id, u.nombre, u.nomina
FROM group_messages gm
JOIN users u ON gm.user_id = u.id
ORDER BY gm.created_at DESC
LIMIT ?;

-- name: GetGroupMessagesSince :many
SELECT
    gm.id, gm.content, gm.created_at,
    u.id as user_id, u.nombre, u.nomina
FROM group_messages gm
JOIN users u ON gm.user_id = u.id
WHERE gm.id > ?
ORDER BY gm.created_at ASC;

-- ============ AI CONVERSATIONS ============

-- name: CreateAIConversation :one
INSERT INTO ai_conversations (user_id, title)
VALUES (?, ?)
RETURNING *;

-- name: GetUserConversations :many
SELECT id, title, created_at, updated_at
FROM ai_conversations
WHERE user_id = ?
ORDER BY updated_at DESC
LIMIT 50;

-- name: GetConversation :one
SELECT * FROM ai_conversations
WHERE id = ? AND user_id = ?;

-- name: UpdateConversationTitle :execresult
UPDATE ai_conversations
SET title = ?, updated_at = datetime('now')
WHERE id = ? AND user_id = ?;

-- name: TouchConversation :execresult
UPDATE ai_conversations
SET updated_at = datetime('now')
WHERE id = ?;

-- name: DeleteConversation :execresult
DELETE FROM ai_conversations WHERE id = ? AND user_id = ?;

-- ============ AI MESSAGES ============

-- name: CreateAIMessage :one
INSERT INTO ai_messages (conversation_id, role, content)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetConversationMessages :many
SELECT id, role, content, created_at
FROM ai_messages
WHERE conversation_id = ?
ORDER BY created_at ASC;

-- name: GetRecentConversationMessages :many
SELECT id, role, content, created_at
FROM ai_messages
WHERE conversation_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: CountConversationMessages :one
SELECT COUNT(*) as count FROM ai_messages WHERE conversation_id = ?;
```

Después crear `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "queries.sql"
    schema: "schema.sql"
    gen:
      go:
        package: "db"
        out: "db"
```

Luego ejecutar:

```bash
make sqlc
```
