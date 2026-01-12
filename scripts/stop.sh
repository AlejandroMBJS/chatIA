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

# Verificar si esta corriendo
if ! docker compose ps --status running 2>/dev/null | grep -q "iris-chat"; then
    echo "[INFO] El servicio no esta corriendo"
    exit 0
fi

# Detener servicio (sin borrar volumenes)
echo "Deteniendo contenedor..."
docker compose down

echo ""
echo "[OK] Servicio detenido"
echo ""
echo "Nota: Los datos de la base de datos se mantienen en el volumen de Docker"
echo "      Para iniciar de nuevo: ./scripts/start.sh"
echo "      Para resetear la BD:   ./scripts/reset-db.sh"
echo ""
