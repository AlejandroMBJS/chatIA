#!/bin/bash
# ========================================
# IRIS Chat - Setup inicial
# ========================================
# Ejecutar despues de git clone/pull
# ========================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "========================================"
echo "  IRIS Chat - Setup Inicial"
echo "========================================"
echo ""

# Hacer ejecutables todos los scripts
echo "Configurando permisos de scripts..."
chmod +x "$SCRIPT_DIR"/*.sh 2>/dev/null || true
echo "[OK] Scripts configurados"

# Detectar si usar docker-compose o podman-compose
if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
    echo "[INFO] Usando podman-compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
    echo "[INFO] Usando docker-compose"
elif docker compose version &> /dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
    echo "[INFO] Usando docker compose"
else
    echo "[ERROR] No se encontro docker-compose ni podman-compose"
    exit 1
fi

# Verificar que Ollama este corriendo en el host
echo ""
echo "Verificando Ollama..."
if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "[OK] Ollama esta corriendo en localhost:11434"

    # Verificar si el modelo esta disponible
    if curl -s http://localhost:11434/api/tags | grep -q "deepseek-r1"; then
        echo "[OK] Modelo deepseek-r1 disponible"
    else
        echo "[WARN] Modelo deepseek-r1 no encontrado"
        echo "       Ejecuta: ollama pull deepseek-r1:14b"
    fi
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

# Construir imagen Docker
echo ""
echo "Construyendo imagen Docker..."
$COMPOSE_CMD build

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
