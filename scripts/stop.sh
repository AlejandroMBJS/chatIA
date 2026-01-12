#!/bin/bash
# ========================================
# IRIS Chat - Detener servicio
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "========================================"
echo "  IRIS Chat - Deteniendo servicio"
echo "========================================"
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

# Detener servicio (sin borrar volumenes)
echo "Deteniendo contenedor..."
$COMPOSE_CMD down

echo ""
echo "[OK] Servicio detenido"
echo ""
echo "Nota: Los datos de la base de datos se mantienen en el volumen"
echo "      Para iniciar de nuevo: ./scripts/start.sh"
echo "      Para resetear la BD:   ./scripts/reset-db.sh"
echo ""
