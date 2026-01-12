# IRIS Chat - AQUILA

Sistema de chat empresarial con IA integrada.

## Instalacion Rapida (Docker)

```bash
# 1. Clonar el repositorio
git clone <repo-url>
cd GIAChat

# 2. Dar permisos a los scripts
chmod +x scripts/*.sh

# 3. Setup inicial (construye la imagen Docker)
./scripts/setup.sh

# 4. Iniciar el servicio
./scripts/start.sh
```

**URL:** http://localhost:9999

**Credenciales:** admin / admin123

## Requisitos

- Docker y Docker Compose
- Ollama corriendo en el host

```bash
# Iniciar Ollama
ollama serve

# Descargar el modelo
ollama pull deepseek-r1:14b
```

## Comandos

| Comando | Descripcion |
|---------|-------------|
| `./scripts/setup.sh` | Setup inicial (primera vez) |
| `./scripts/start.sh` | Iniciar servicio |
| `./scripts/stop.sh` | Detener servicio |
| `./scripts/restart.sh` | Reiniciar (sin perder datos) |
| `./scripts/logs.sh` | Ver logs |
| `./scripts/reset-db.sh` | Resetear BD (BORRA TODO) |

O usando Make:

```bash
make docker-setup     # Setup inicial
make docker-start     # Iniciar
make docker-stop      # Detener
make docker-restart   # Reiniciar
make docker-logs      # Ver logs
make docker-reset     # Resetear BD
```

## Desarrollo Local (sin Docker)

```bash
# Instalar dependencias
go mod tidy

# Ejecutar
make dev
```

## Caracteristicas

- Chat con IA (DeepSeek R1)
- Base de conocimiento empresarial
- Filtros de seguridad
- Panel de administracion
- Autenticacion con nomina/password

## Stack Tecnologico

| Componente | Tecnologia |
|------------|------------|
| Backend | Go 1.23+ |
| Base de datos | SQLite |
| Frontend | HTML + HTMX + CSS |
| IA | Ollama (deepseek-r1:14b) |

## Estructura

```
GIAChat/
├── scripts/           # Scripts de administracion
├── internal/          # Codigo Go
│   ├── handlers/      # HTTP handlers
│   ├── services/      # Servicios (Ollama, etc)
│   └── middleware/    # Auth, rate limiting
├── templates/         # HTML templates
├── static/            # CSS, imagenes
├── db/                # Codigo generado SQLC
├── Dockerfile
├── docker-compose.yml
└── schema.sql         # Esquema de BD
```
