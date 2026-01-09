# Chat Empleados con DeepSeek IA

Aplicación web interna para empresa manufacturera que permite comunicación entre empleados y asistencia de inteligencia artificial.

## Características

- **Chat grupal**: Comunicación en tiempo real entre empleados via WebSocket
- **Chat IA privado**: Cada empleado tiene conversaciones privadas con DeepSeek R1
- **Autenticación**: Login con número de nómina + contraseña
- **Panel admin**: Aprobar/rechazar cuentas de nuevos empleados

## Stack Tecnológico

| Componente | Tecnología |
|------------|------------|
| Backend | Go 1.23+ (stdlib net/http) |
| WebSocket | gorilla/websocket |
| Base de datos | SQLite |
| Queries | SQLC (type-safe) |
| Frontend | HTML + HTMX + CSS |
| IA | Ollama API (localhost:11434) |
| Modelo | deepseek-r1:14b |
| Templates | embed.FS |

## Estructura del Proyecto

```
chat-empleados/
├── main.go                 # Entry point, rutas, DI
├── go.mod
├── sqlc.yaml
├── schema.sql              # Definición de tablas
├── queries.sql             # Queries SQLC
├── Makefile
│
├── db/                     # Generado por SQLC
│   ├── db.go
│   ├── models.go
│   └── queries.sql.go
│
├── internal/
│   ├── config/             # Configuración
│   ├── middleware/         # Auth middleware
│   ├── handlers/           # HTTP handlers
│   └── services/           # Ollama client
│
├── templates/              # HTML templates
└── static/                 # CSS, JS (HTMX)
```

## Requisitos

- Go 1.23+
- SQLite3
- SQLC (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- Ollama con modelo deepseek-r1:14b

## Instalación

```bash
# Clonar e instalar dependencias
git clone <repo>
cd chat-empleados
go mod tidy

# Generar código SQLC
make sqlc

# Inicializar base de datos
make reset-db

# Ejecutar
make dev
```

## Comandos Make

| Comando | Descripción |
|---------|-------------|
| `make build` | Compilar binario |
| `make run` | Compilar y ejecutar |
| `make dev` | Ejecutar con go run |
| `make test` | Ejecutar tests |
| `make check` | Verificar build + vet |
| `make sqlc` | Generar código SQLC |
| `make reset-db` | Recrear base de datos |
| `make clean` | Limpiar binarios |

## Usuario Admin por Defecto

- **Nómina**: admin
- **Contraseña**: admin123

## Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | /health | Health check |
| GET | /login | Página de login |
| POST | /login | Procesar login |
| GET | /register | Página de registro |
| POST | /register | Procesar registro |
| POST | /logout | Cerrar sesión |
| GET | /chat | Chat grupal |
| WS | /chat/ws | WebSocket chat grupal |
| GET | /ai | Chat con IA |
| POST | /ai/send | Enviar mensaje a IA |
| GET | /admin | Panel administración |
| POST | /admin/approve/{id} | Aprobar usuario |
| POST | /admin/reject/{id} | Rechazar usuario |
