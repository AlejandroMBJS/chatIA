#!/bin/bash
# ========================================
# IRIS Chat - Resetear base de datos
# ========================================
# ADVERTENCIA: Este script borra TODOS los datos
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

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
docker compose down

echo "Eliminando volumen de datos..."
docker volume rm giachat_iris-data 2>/dev/null || true

echo ""
echo "[OK] Base de datos eliminada"
echo ""
echo "Para iniciar con una base de datos limpia:"
echo "  ./scripts/start.sh"
echo ""
