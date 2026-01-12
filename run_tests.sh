#!/bin/bash

# ============================================
# GIA Chat - Suite de Pruebas Exhaustivas
# ============================================

BASE_URL="http://localhost:9999"
COOKIE_FILE="/tmp/test_cookies.txt"
PASSED=0
FAILED=0
TOTAL=0

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Funciones de utilidad
log_test() {
    echo -e "\n${YELLOW}[TEST]${NC} $1"
    ((TOTAL++))
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED++))
}

cleanup() {
    rm -f $COOKIE_FILE /tmp/response.html /tmp/test_*.txt
}

# Verificar que el servidor está corriendo
check_server() {
    log_test "Verificando que el servidor está corriendo"
    HEALTH=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/health)
    if [ "$HEALTH" = "200" ]; then
        log_pass "Servidor respondiendo en $BASE_URL"
    else
        log_fail "Servidor no responde (status: $HEALTH)"
        echo "Iniciando servidor..."
        export DB_PATH="./chat.db"
        ./giachat &
        sleep 3
        HEALTH=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/health)
        if [ "$HEALTH" = "200" ]; then
            log_pass "Servidor iniciado correctamente"
        else
            log_fail "No se pudo iniciar el servidor"
            exit 1
        fi
    fi
}

# ============================================
# TESTS DE ENDPOINTS PÚBLICOS
# ============================================

test_public_endpoints() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Endpoints Públicos${NC}"
    echo -e "${YELLOW}========================================${NC}"

    # Health check
    log_test "GET /health"
    RESPONSE=$(curl -s $BASE_URL/health)
    if [ "$RESPONSE" = "OK" ]; then
        log_pass "Health endpoint retorna OK"
    else
        log_fail "Health endpoint no retorna OK: $RESPONSE"
    fi

    # Login page
    log_test "GET /login"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" $BASE_URL/login)
    if [ "$STATUS" = "200" ] && grep -q "nomina" /tmp/response.html; then
        log_pass "Login page carga correctamente"
    else
        log_fail "Login page falla (status: $STATUS)"
    fi

    # Register page
    log_test "GET /register"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" $BASE_URL/register)
    if [ "$STATUS" = "200" ] && grep -q "nombre" /tmp/response.html; then
        log_pass "Register page carga correctamente"
    else
        log_fail "Register page falla (status: $STATUS)"
    fi

    # Pending page
    log_test "GET /pending"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" $BASE_URL/pending)
    if [ "$STATUS" = "200" ]; then
        log_pass "Pending page carga correctamente"
    else
        log_fail "Pending page falla (status: $STATUS)"
    fi

    # Static CSS
    log_test "GET /static/styles.css"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/static/styles.css)
    if [ "$STATUS" = "200" ]; then
        log_pass "CSS carga correctamente"
    else
        log_fail "CSS falla (status: $STATUS)"
    fi

    # AI Health
    log_test "GET /ai/health"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" $BASE_URL/ai/health)
    if [ "$STATUS" = "200" ] && grep -q "model" /tmp/response.html; then
        log_pass "AI health endpoint funciona"
    else
        log_fail "AI health endpoint falla (status: $STATUS)"
    fi
}

# ============================================
# TESTS DE AUTENTICACIÓN
# ============================================

test_authentication() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Autenticación${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup

    # Login inválido
    log_test "POST /login con credenciales inválidas"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -X POST -d "nomina=fake&password=fake" $BASE_URL/login)
    if [[ "$REDIRECT" == *"error"* ]]; then
        log_pass "Login inválido redirige con error"
    else
        log_fail "Login inválido no muestra error: $REDIRECT"
    fi

    # Login válido (admin)
    log_test "POST /login con admin/admin123"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -c $COOKIE_FILE -X POST -d "nomina=admin&password=admin123" $BASE_URL/login)
    if [[ "$REDIRECT" == *"/chat"* ]]; then
        log_pass "Login admin redirige a /chat"
    else
        log_fail "Login admin no redirige a /chat: $REDIRECT"
    fi

    # Verificar cookie de sesión
    log_test "Verificar cookie de sesión"
    if grep -q "session_token" $COOKIE_FILE 2>/dev/null; then
        log_pass "Cookie de sesión establecida"
    else
        log_fail "Cookie de sesión no establecida"
    fi

    # Acceso a ruta protegida con sesión
    log_test "GET /chat con sesión válida"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/chat)
    if [ "$STATUS" = "200" ]; then
        log_pass "Chat accesible con sesión"
    else
        log_fail "Chat no accesible con sesión (status: $STATUS)"
    fi

    # Logout
    log_test "POST /logout"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE -X POST $BASE_URL/logout)
    if [ "$STATUS" = "303" ]; then
        log_pass "Logout exitoso"
    else
        log_fail "Logout falla (status: $STATUS)"
    fi
}

# ============================================
# TESTS DE RUTAS PROTEGIDAS
# ============================================

test_protected_routes() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Rutas Protegidas (sin auth)${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup

    ROUTES=("/chat" "/ai" "/admin" "/profile")

    for route in "${ROUTES[@]}"; do
        log_test "GET $route sin autenticación"
        REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" $BASE_URL$route)
        if [[ "$REDIRECT" == *"/login"* ]]; then
            log_pass "$route redirige a login"
        else
            log_fail "$route no redirige a login: $REDIRECT"
        fi
    done
}

# ============================================
# TESTS DE REGISTRO DE USUARIOS
# ============================================

test_registration() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Registro de Usuarios${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup
    TEST_USER="testuser_$(date +%s)"

    # Registro con datos válidos
    log_test "POST /register con datos válidos"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -X POST \
        -d "nomina=$TEST_USER&password=test123456&password_confirm=test123456&nombre=Test User&departamento=QA" \
        $BASE_URL/register)
    if [[ "$REDIRECT" == *"/pending"* ]]; then
        log_pass "Registro exitoso redirige a /pending"
    else
        log_fail "Registro no redirige a /pending: $REDIRECT"
    fi

    # Verificar usuario en BD
    log_test "Verificar usuario en base de datos"
    USER_EXISTS=$(sqlite3 ./chat.db "SELECT COUNT(*) FROM users WHERE nomina='$TEST_USER';")
    if [ "$USER_EXISTS" = "1" ]; then
        log_pass "Usuario creado en BD"
    else
        log_fail "Usuario no encontrado en BD"
    fi

    # Verificar que usuario no está aprobado
    log_test "Verificar usuario no aprobado"
    APPROVED=$(sqlite3 ./chat.db "SELECT approved FROM users WHERE nomina='$TEST_USER';")
    if [ "$APPROVED" = "0" ]; then
        log_pass "Usuario pendiente de aprobación"
    else
        log_fail "Usuario no debería estar aprobado"
    fi

    # Login con usuario no aprobado
    log_test "POST /login con usuario no aprobado"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -c $COOKIE_FILE -X POST \
        -d "nomina=$TEST_USER&password=test123456" $BASE_URL/login)
    if [[ "$REDIRECT" == *"/pending"* ]]; then
        log_pass "Usuario no aprobado redirige a /pending"
    else
        log_fail "Usuario no aprobado no redirige correctamente: $REDIRECT"
    fi

    # Registro con passwords que no coinciden
    log_test "POST /register con passwords diferentes"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -X POST \
        -d "nomina=baduser&password=pass1&password_confirm=pass2&nombre=Bad User" \
        $BASE_URL/register)
    if [[ "$REDIRECT" == *"error"* ]]; then
        log_pass "Passwords diferentes muestra error"
    else
        log_fail "No detecta passwords diferentes: $REDIRECT"
    fi

    # Registro con usuario duplicado
    log_test "POST /register con usuario duplicado"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -X POST \
        -d "nomina=$TEST_USER&password=test123456&password_confirm=test123456&nombre=Duplicate" \
        $BASE_URL/register)
    if [[ "$REDIRECT" == *"error"* ]]; then
        log_pass "Usuario duplicado muestra error"
    else
        log_fail "No detecta usuario duplicado: $REDIRECT"
    fi
}

# ============================================
# TESTS DE CAMBIO DE IDIOMA
# ============================================

test_language() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Cambio de Idioma${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup

    # Cambiar a inglés
    log_test "GET /set-language?lang=en"
    curl -s -c $COOKIE_FILE $BASE_URL/set-language?lang=en > /dev/null
    if grep -q "lang.*en" $COOKIE_FILE 2>/dev/null; then
        log_pass "Cookie de idioma inglés establecida"
    else
        log_fail "Cookie de idioma no establecida"
    fi

    # Verificar contenido en inglés
    log_test "Verificar página en inglés"
    curl -s -b $COOKIE_FILE $BASE_URL/login > /tmp/response.html
    if grep -qi "Login\|Password\|Employee" /tmp/response.html; then
        log_pass "Página muestra contenido en inglés"
    else
        log_fail "Página no muestra contenido en inglés"
    fi

    # Cambiar a español
    log_test "GET /set-language?lang=es"
    curl -s -c $COOKIE_FILE -b $COOKIE_FILE $BASE_URL/set-language?lang=es > /dev/null
    if grep -q "lang.*es" $COOKIE_FILE 2>/dev/null; then
        log_pass "Cookie de idioma español establecida"
    else
        log_fail "Cookie de idioma español no establecida"
    fi
}

# ============================================
# TESTS DE ADMIN PANEL
# ============================================

test_admin() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Panel de Administración${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup

    # Login como admin
    curl -s -c $COOKIE_FILE -X POST -d "nomina=admin&password=admin123" $BASE_URL/login > /dev/null

    # Acceso a /admin
    log_test "GET /admin con sesión admin"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/admin)
    if [ "$STATUS" = "200" ]; then
        log_pass "Admin panel accesible"
    else
        log_fail "Admin panel no accesible (status: $STATUS)"
    fi

    # Verificar elementos del admin panel
    log_test "Verificar contenido del admin panel"
    if grep -qi "usuarios\|admin\|filtros" /tmp/response.html; then
        log_pass "Admin panel muestra contenido esperado"
    else
        log_fail "Admin panel no muestra contenido esperado"
    fi

    # Acceso a /admin/users
    log_test "GET /admin/users"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/admin/users)
    if [ "$STATUS" = "200" ]; then
        log_pass "Admin users accesible"
    else
        log_fail "Admin users no accesible (status: $STATUS)"
    fi

    # Acceso a /admin/filters
    log_test "GET /admin/filters"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/admin/filters)
    if [ "$STATUS" = "200" ]; then
        log_pass "Admin filters accesible"
    else
        log_fail "Admin filters no accesible (status: $STATUS)"
    fi

    # Acceso a /admin/logs
    log_test "GET /admin/logs"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/admin/logs)
    if [ "$STATUS" = "200" ]; then
        log_pass "Admin logs accesible"
    else
        log_fail "Admin logs no accesible (status: $STATUS)"
    fi

    # Acceso a /admin/stats
    log_test "GET /admin/stats"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/admin/stats)
    if [ "$STATUS" = "200" ] && grep -q "{" /tmp/response.html; then
        log_pass "Admin stats retorna JSON"
    else
        log_fail "Admin stats no retorna JSON (status: $STATUS)"
    fi
}

# ============================================
# TESTS DE APROBACIÓN DE USUARIOS
# ============================================

test_user_approval() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Aprobación de Usuarios${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup
    TEST_USER="approve_test_$(date +%s)"

    # Crear usuario de prueba
    curl -s -X POST -d "nomina=$TEST_USER&password=test123456&password_confirm=test123456&nombre=Approval Test" \
        $BASE_URL/register > /dev/null

    # Obtener ID del usuario
    USER_ID=$(sqlite3 ./chat.db "SELECT id FROM users WHERE nomina='$TEST_USER';")

    # Login como admin
    curl -s -c $COOKIE_FILE -X POST -d "nomina=admin&password=admin123" $BASE_URL/login > /dev/null

    # Aprobar usuario
    log_test "POST /admin/approve/$USER_ID"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE -X POST $BASE_URL/admin/approve/$USER_ID)
    if [ "$STATUS" = "200" ]; then
        log_pass "Aprobación de usuario exitosa"
    else
        log_fail "Aprobación de usuario falla (status: $STATUS)"
    fi

    # Verificar que usuario está aprobado
    log_test "Verificar usuario aprobado en BD"
    APPROVED=$(sqlite3 ./chat.db "SELECT approved FROM users WHERE nomina='$TEST_USER';")
    if [ "$APPROVED" = "1" ]; then
        log_pass "Usuario marcado como aprobado en BD"
    else
        log_fail "Usuario no marcado como aprobado"
    fi

    # Login con usuario aprobado
    cleanup
    log_test "POST /login con usuario aprobado"
    REDIRECT=$(curl -s -o /dev/null -w "%{redirect_url}" -c $COOKIE_FILE -X POST \
        -d "nomina=$TEST_USER&password=test123456" $BASE_URL/login)
    if [[ "$REDIRECT" == *"/chat"* ]]; then
        log_pass "Usuario aprobado puede acceder a /chat"
    else
        log_fail "Usuario aprobado no puede acceder: $REDIRECT"
    fi
}

# ============================================
# TESTS DE SEGURIDAD
# ============================================

test_security() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Seguridad${NC}"
    echo -e "${YELLOW}========================================${NC}"

    # Headers de seguridad
    log_test "Verificar headers de seguridad"
    HEADERS=$(curl -s -I $BASE_URL/login)
    MISSING=""

    echo "$HEADERS" | grep -qi "X-Content-Type-Options" || MISSING="$MISSING X-Content-Type-Options"
    echo "$HEADERS" | grep -qi "X-Frame-Options" || MISSING="$MISSING X-Frame-Options"
    echo "$HEADERS" | grep -qi "X-XSS-Protection" || MISSING="$MISSING X-XSS-Protection"

    if [ -z "$MISSING" ]; then
        log_pass "Todos los headers de seguridad presentes"
    else
        log_fail "Headers faltantes:$MISSING"
    fi

    # Protección XSS en registro
    log_test "Verificar sanitización XSS en registro"
    curl -s -X POST -d "nomina=xss_test&password=test123456&password_confirm=test123456&nombre=<script>alert('xss')</script>" \
        $BASE_URL/register > /dev/null
    NAME=$(sqlite3 ./chat.db "SELECT nombre FROM users WHERE nomina='xss_test';")
    if [[ "$NAME" != *"<script>"* ]]; then
        log_pass "XSS sanitizado en nombre"
    else
        log_fail "XSS no sanitizado: $NAME"
    fi
}

# ============================================
# TESTS DE CHAT Y AI
# ============================================

test_chat_ai() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Chat y AI${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup

    # Login como admin
    curl -s -c $COOKIE_FILE -X POST -d "nomina=admin&password=admin123" $BASE_URL/login > /dev/null

    # Página de chat
    log_test "GET /chat"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/chat)
    if [ "$STATUS" = "200" ]; then
        log_pass "Página de chat carga"
    else
        log_fail "Página de chat no carga (status: $STATUS)"
    fi

    # Verificar WebSocket endpoint existe
    log_test "Verificar que /chat/ws está configurado"
    # No podemos probar WebSocket con curl, pero verificamos que la ruta existe
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/chat/ws)
    # WebSocket devuelve 400 si no es una request de upgrade, lo cual es esperado
    if [ "$STATUS" = "400" ] || [ "$STATUS" = "200" ]; then
        log_pass "Endpoint WebSocket configurado"
    else
        log_fail "Endpoint WebSocket no configurado (status: $STATUS)"
    fi

    # Página de AI
    log_test "GET /ai"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/ai)
    if [ "$STATUS" = "200" ]; then
        log_pass "Página de AI carga"
    else
        log_fail "Página de AI no carga (status: $STATUS)"
    fi

    # Nueva conversación
    log_test "GET /ai/new"
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -b $COOKIE_FILE $BASE_URL/ai/new)
    if [ "$STATUS" = "303" ]; then
        log_pass "Nueva conversación crea redirect"
    else
        log_fail "Nueva conversación falla (status: $STATUS)"
    fi
}

# ============================================
# TESTS DE REQUEST APPROVAL
# ============================================

test_request_approval() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Solicitud de Aprobación${NC}"
    echo -e "${YELLOW}========================================${NC}"

    cleanup
    TEST_USER="request_test_$(date +%s)"

    # Crear usuario
    curl -s -X POST -d "nomina=$TEST_USER&password=test123456&password_confirm=test123456&nombre=Request Test" \
        $BASE_URL/register > /dev/null

    # Login (irá a pending y establecerá cookie last_nomina)
    curl -s -c $COOKIE_FILE -X POST -d "nomina=$TEST_USER&password=test123456" $BASE_URL/login > /dev/null

    # Solicitar aprobación
    log_test "POST /request-approval"
    STATUS=$(curl -s -o /tmp/response.html -w "%{http_code}" -b $COOKIE_FILE -X POST $BASE_URL/request-approval)
    if [ "$STATUS" = "200" ]; then
        log_pass "Solicitud de aprobación enviada"
    else
        log_fail "Solicitud de aprobación falla (status: $STATUS)"
    fi

    # Verificar mensaje de respuesta
    log_test "Verificar respuesta de solicitud"
    if grep -qi "enviada\|sent\|notif" /tmp/response.html; then
        log_pass "Respuesta indica éxito"
    else
        log_fail "Respuesta no indica éxito"
    fi
}

# ============================================
# TESTS DE BASE DE DATOS
# ============================================

test_database() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}TESTS: Base de Datos${NC}"
    echo -e "${YELLOW}========================================${NC}"

    # Verificar tablas
    log_test "Verificar tablas existen"
    TABLES=$(sqlite3 ./chat.db ".tables")
    REQUIRED="users sessions group_messages ai_conversations ai_messages security_filters notifications"
    MISSING=""

    for table in $REQUIRED; do
        if ! echo "$TABLES" | grep -q "$table"; then
            MISSING="$MISSING $table"
        fi
    done

    if [ -z "$MISSING" ]; then
        log_pass "Todas las tablas existen"
    else
        log_fail "Tablas faltantes:$MISSING"
    fi

    # Verificar filtros de seguridad cargados
    log_test "Verificar filtros de seguridad"
    FILTER_COUNT=$(sqlite3 ./chat.db "SELECT COUNT(*) FROM security_filters WHERE is_active=1;")
    if [ "$FILTER_COUNT" -gt "10" ]; then
        log_pass "$FILTER_COUNT filtros de seguridad activos"
    else
        log_fail "Pocos filtros activos: $FILTER_COUNT"
    fi

    # Verificar admin existe
    log_test "Verificar usuario admin"
    ADMIN=$(sqlite3 ./chat.db "SELECT COUNT(*) FROM users WHERE nomina='admin' AND is_admin=1 AND approved=1;")
    if [ "$ADMIN" = "1" ]; then
        log_pass "Usuario admin configurado correctamente"
    else
        log_fail "Usuario admin no configurado"
    fi
}

# ============================================
# RESUMEN FINAL
# ============================================

print_summary() {
    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}RESUMEN DE PRUEBAS${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo -e "Total de tests: $TOTAL"
    echo -e "${GREEN}Pasados: $PASSED${NC}"
    echo -e "${RED}Fallidos: $FAILED${NC}"

    if [ $FAILED -eq 0 ]; then
        echo -e "\n${GREEN}========================================${NC}"
        echo -e "${GREEN}✓ TODAS LAS PRUEBAS PASARON${NC}"
        echo -e "${GREEN}El sistema está listo para producción${NC}"
        echo -e "${GREEN}========================================${NC}"
    else
        echo -e "\n${RED}========================================${NC}"
        echo -e "${RED}✗ HAY $FAILED PRUEBAS FALLIDAS${NC}"
        echo -e "${RED}Revisar los errores antes de producción${NC}"
        echo -e "${RED}========================================${NC}"
    fi
}

# ============================================
# EJECUTAR TODOS LOS TESTS
# ============================================

main() {
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}GIA Chat - Suite de Pruebas Completa${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo "Fecha: $(date)"
    echo "URL: $BASE_URL"

    check_server
    test_public_endpoints
    test_authentication
    test_protected_routes
    test_registration
    test_language
    test_admin
    test_user_approval
    test_security
    test_chat_ai
    test_request_approval
    test_database

    cleanup
    print_summary

    exit $FAILED
}

main
