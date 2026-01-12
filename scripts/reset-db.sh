#!/bin/bash
# ========================================
# IRIS Chat - Resetear base de datos
# ========================================
# ADVERTENCIA: Este script borra TODOS los datos
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Detectar compose command
if command -v podman-compose &> /dev/null; then
    COMPOSE_CMD="podman-compose"
    CONTAINER_CMD="podman"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
    CONTAINER_CMD="docker"
elif docker compose version &> /dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
    CONTAINER_CMD="docker"
else
    echo "[ERROR] No se encontro docker-compose ni podman-compose"
    exit 1
fi

# Obtener el nombre del proyecto de docker compose
PROJECT_NAME=$(basename "$PROJECT_DIR" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]//g')

echo "========================================"
echo "  IRIS Chat - Resetear Base de Datos"
echo "========================================"
echo ""
echo "[ADVERTENCIA] Esto borrara TODOS los datos:"
echo "  - Usuarios"
echo "  - Conversaciones"
echo "  - Mensajes"
echo "  - Configuracion"
echo ""
read -p "Estas seguro? (escribe 'SI' para confirmar): " confirm

if [ "$confirm" != "SI" ]; then
    echo ""
    echo "Operacion cancelada"
    exit 0
fi

echo ""
echo "Deteniendo servicio..."
$COMPOSE_CMD down 2>/dev/null || true

echo "Eliminando volumen de datos..."
# Intentar varios nombres posibles de volumenes
$CONTAINER_CMD volume rm "${PROJECT_NAME}_iris-data" 2>/dev/null || true
$CONTAINER_CMD volume rm "iris_iris-data" 2>/dev/null || true
$CONTAINER_CMD volume rm "giachat_iris-data" 2>/dev/null || true

echo ""
echo "[OK] Base de datos eliminada"
echo ""
echo "Para iniciar con una base de datos limpia:"
echo "  ./scripts/start.sh"
echo ""
