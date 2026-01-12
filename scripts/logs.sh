#!/bin/bash
# ========================================
# IRIS Chat - Ver logs
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "Mostrando logs (Ctrl+C para salir)..."
echo ""

docker compose logs -f --tail=100
