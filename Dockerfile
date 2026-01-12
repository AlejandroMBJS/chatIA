# ========================================
# IRIS Chat - Dockerfile Multi-stage
# ========================================

# Stage 1: Build
FROM docker.io/library/golang:1.22-alpine AS builder

# Instalar dependencias de compilacion para SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copiar archivos de dependencias primero (para cache de capas)
COPY go.mod go.sum ./
RUN go mod download

# Copiar codigo fuente
COPY . .

# Compilar con CGO habilitado (necesario para SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags="-w -s" -o chat-empleados .

# Stage 2: Runtime
FROM docker.io/library/alpine:3.19

# Instalar dependencias runtime
RUN apk add --no-cache \
    sqlite \
    ca-certificates \
    tzdata \
    curl

# Crear usuario no-root para seguridad
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -h /app -D appuser

WORKDIR /app

# Copiar binario desde el builder
COPY --from=builder /app/chat-empleados .

# Copiar schema para inicializacion de DB
COPY --from=builder /app/schema.sql .

# Crear directorio para datos y asegurar permisos
RUN mkdir -p /data && chown -R appuser:appgroup /data /app

# Cambiar a usuario no-root
USER appuser

# Variables de entorno por defecto
ENV PORT=9999 \
    DB_PATH=/data/chat.db \
    OLLAMA_URL=http://host.docker.internal:11434 \
    OLLAMA_MODEL=deepseek-r1:14b \
    SESSION_DURATION=24h \
    ENABLE_SECURITY_FILTERS=true \
    FORCE_SECURE_COOKIE=false \
    OLLAMA_TIMEOUT=5m \
    OLLAMA_RETRIES=3

# Puerto de la aplicacion
EXPOSE 9999

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:9999/health || exit 1

# Comando de inicio
CMD ["./chat-empleados"]
