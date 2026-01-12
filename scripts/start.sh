#!/bin/bash
# ========================================
# IRIS Chat - Iniciar servicio
# ========================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "========================================"
echo "  IRIS Chat - Iniciando servicio"
echo "========================================"
echo ""

# Hacer ejecutables los scripts por si acaso
chmod +x "$SCRIPT_DIR"/*.sh 2>/dev/null || true

# Verificar si ya esta corriendo
if docker compose ps --status running 2>/dev/null | grep -q "iris-chat"; then
    echo "[INFO] El servicio ya esta corriendo"
    echo ""
    docker compose ps
    echo ""
    echo "URL: http://localhost:9999"
    exit 0
fi

# Verificar si la imagen existe, si no, construirla
if ! docker images | grep -q "iris-chat\|giachat"; then
    echo "[INFO] Imagen no encontrada, construyendo..."
    docker compose build
fi

# Verificar Ollama
echo ""
if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "[OK] Ollama disponible"
else
    echo "[WARN] Ollama no esta corriendo en localhost:11434"
    echo "       La IA no estara disponible hasta que inicies Ollama"
    echo ""
fi

# Crear directorio de datos
mkdir -p "$PROJECT_DIR/data"

# Iniciar servicio
echo "Iniciando contenedor..."
docker compose up -d

# Esperar a que este listo
echo "Esperando que el servicio este listo..."
for i in {1..30}; do
    if curl -s http://localhost:9999/health > /dev/null 2>&1; then
        echo ""
        echo "========================================"
        echo "  Servicio iniciado correctamente!"
        echo "========================================"
        echo ""
        echo "URL: http://localhost:9999"
        echo ""
        echo "Credenciales por defecto:"
        echo "  Usuario: admin"
        echo "  Password: admin123"
        echo ""
        echo "Para ver logs: docker compose logs -f"
        echo "Para detener:  ./scripts/stop.sh"
        echo ""
        exit 0
    fi
    sleep 1
    echo -n "."
done

echo ""
echo "[ERROR] El servicio no respondio en 30 segundos"
echo "        Revisa los logs: docker compose logs"
exit 1
