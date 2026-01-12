#!/bin/bash
# ========================================
# IRIS Chat - Reiniciar servicio
# ========================================
# NOTA: El reinicio NO borra la base de datos
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "========================================"
echo "  IRIS Chat - Reiniciando servicio"
echo "========================================"
echo ""
echo "[INFO] La base de datos se mantiene intacta"
echo ""

# Detectar compose command
if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
else
    echo "[ERROR] No se encontro docker-compose ni podman-compose"
    exit 1
fi

# Detener si esta corriendo
echo "Deteniendo servicio actual..."
$COMPOSE_CMD down 2>/dev/null || true
sleep 2

# Reconstruir si se solicita
if [ "$1" == "--rebuild" ]; then
    echo "Reconstruyendo imagen..."
    $COMPOSE_CMD build
fi

# Iniciar servicio
echo "Iniciando servicio..."
$COMPOSE_CMD up -d

# Esperar a que este listo
echo "Esperando que el servicio este listo..."
for i in {1..30}; do
    if curl -s http://localhost:9999/health > /dev/null 2>&1; then
        echo ""
        echo "========================================"
        echo "  Servicio reiniciado correctamente!"
        echo "========================================"
        echo ""
        echo "URL: http://localhost:9999"
        echo ""
        echo "La base de datos NO fue modificada."
        echo ""
        exit 0
    fi
    sleep 1
    echo -n "."
done

echo ""
echo "[ERROR] El servicio no respondio en 30 segundos"
echo "        Revisa los logs: $COMPOSE_CMD logs"
exit 1
