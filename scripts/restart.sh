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

# Detener si esta corriendo
if docker compose ps --status running 2>/dev/null | grep -q "iris-chat"; then
    echo "Deteniendo servicio actual..."
    docker compose down
    sleep 2
fi

# Reconstruir si se solicita
if [ "$1" == "--rebuild" ]; then
    echo "Reconstruyendo imagen..."
    docker compose build
fi

# Iniciar servicio
echo "Iniciando servicio..."
docker compose up -d

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
echo "        Revisa los logs: docker compose logs"
exit 1
