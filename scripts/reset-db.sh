#!/bin/bash
# ========================================
# IRIS Chat - Resetear base de datos
# ========================================
# ADVERTENCIA: Este script borra TODOS los datos
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Obtener el nombre del proyecto de docker compose
PROJECT_NAME=$(basename "$PROJECT_DIR" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]//g')
VOLUME_NAME="${PROJECT_NAME}_iris-data"

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
echo "Volumen a eliminar: $VOLUME_NAME"
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
docker volume rm "$VOLUME_NAME" 2>/dev/null || true

# Tambien intentar con nombre alternativo por si acaso
docker volume rm "iris_iris-data" 2>/dev/null || true
docker volume rm "giachat_iris-data" 2>/dev/null || true

echo ""
echo "[OK] Base de datos eliminada"
echo ""
echo "Para iniciar con una base de datos limpia:"
echo "  ./scripts/start.sh"
echo ""
