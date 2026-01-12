#!/bin/bash
# ========================================
# IRIS Chat - Ver logs
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

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

echo "Mostrando logs (Ctrl+C para salir)..."
echo ""

$COMPOSE_CMD logs -f --tail=100
