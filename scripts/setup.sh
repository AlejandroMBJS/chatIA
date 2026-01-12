#!/bin/bash
# ========================================
# IRIS Chat - Setup inicial
# ========================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "========================================"
echo "  IRIS Chat - Setup Inicial"
echo "========================================"
echo ""

# Verificar Docker
if ! command -v docker &> /dev/null; then
    echo "[ERROR] Docker no esta instalado"
    echo "        Instala Docker: https://docs.docker.com/get-docker/"
    exit 1
fi

# Verificar Docker Compose
if ! docker compose version &> /dev/null; then
    echo "[ERROR] Docker Compose no esta disponible"
    exit 1
fi

echo "[OK] Docker y Docker Compose disponibles"

# Verificar que Ollama este corriendo en el host
echo ""
echo "Verificando Ollama..."
if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "[OK] Ollama esta corriendo en localhost:11434"
else
    echo "[WARN] Ollama no esta corriendo"
    echo "       Para iniciar Ollama: ollama serve"
    echo "       Para descargar el modelo: ollama pull deepseek-r1:14b"
fi

# Crear directorio de datos si no existe
echo ""
echo "Preparando directorios..."
mkdir -p "$PROJECT_DIR/data"
echo "[OK] Directorio de datos listo"

# Construir imagen
echo ""
echo "Construyendo imagen Docker..."
docker compose build

echo ""
echo "========================================"
echo "  Setup completado!"
echo "========================================"
echo ""
echo "Para iniciar el servicio:"
echo "  ./scripts/start.sh"
echo ""
echo "Para detener el servicio:"
echo "  ./scripts/stop.sh"
echo ""
echo "Credenciales por defecto:"
echo "  Usuario: admin"
echo "  Password: admin123"
echo ""
