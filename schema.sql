-- Schema para Chat Empleados con IA
-- Base de datos SQLite

-- ============ USUARIOS ============
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nomina TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    nombre TEXT NOT NULL,
    departamento TEXT DEFAULT '',
    approved INTEGER DEFAULT 0,
    is_admin INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

-- ============ SESIONES ============
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============ MENSAJES CHAT GRUPAL ============
CREATE TABLE IF NOT EXISTS group_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============ CONVERSACIONES IA ============
CREATE TABLE IF NOT EXISTS ai_conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT DEFAULT 'Nueva conversacion',
    model TEXT DEFAULT '',
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============ MENSAJES IA ============
CREATE TABLE IF NOT EXISTS ai_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    filtered INTEGER DEFAULT 0,
    filter_reason TEXT DEFAULT '',
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id) ON DELETE CASCADE
);

-- ============ FILTROS DE SEGURIDAD ============
-- Tabla para configurar filtros que previenen fugas de informacion
CREATE TABLE IF NOT EXISTS security_filters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    filter_type TEXT NOT NULL CHECK (filter_type IN ('keyword', 'regex', 'category')),
    pattern TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('block', 'warn', 'log')),
    is_active INTEGER DEFAULT 1,
    applies_to TEXT DEFAULT 'both' CHECK (applies_to IN ('input', 'output', 'both')),
    severity TEXT DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    created_by INTEGER,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- ============ CATEGORIAS DE FILTROS ============
CREATE TABLE IF NOT EXISTS filter_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now'))
);

-- ============ LOG DE VIOLACIONES DE SEGURIDAD ============
CREATE TABLE IF NOT EXISTS security_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    filter_id INTEGER,
    original_content TEXT NOT NULL,
    action_taken TEXT NOT NULL,
    ip_address TEXT DEFAULT '',
    user_agent TEXT DEFAULT '',
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (filter_id) REFERENCES security_filters(id)
);

-- ============ CONFIGURACION DEL SISTEMA ============
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    description TEXT DEFAULT '',
    updated_at DATETIME DEFAULT (datetime('now'))
);

-- ============ NOTIFICACIONES ============
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('user_pending', 'user_approved', 'user_rejected', 'security_alert', 'system')),
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    read INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============ INDICES ============
CREATE INDEX IF NOT EXISTS idx_users_nomina ON users(nomina);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_group_messages_created ON group_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user ON ai_conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_messages_conversation ON ai_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_security_filters_active ON security_filters(is_active);
CREATE INDEX IF NOT EXISTS idx_security_logs_user ON security_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_security_logs_created ON security_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(user_id, read);

-- ============ BASE DE CONOCIMIENTO ============
-- Conocimiento aprobado que la IA puede usar como contexto
CREATE TABLE IF NOT EXISTS knowledge_base (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT DEFAULT 'general',
    submitted_by INTEGER NOT NULL,
    approved_by INTEGER,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT (datetime('now')),
    approved_at DATETIME,
    FOREIGN KEY (submitted_by) REFERENCES users(id),
    FOREIGN KEY (approved_by) REFERENCES users(id)
);

-- Solicitudes pendientes de conocimiento
CREATE TABLE IF NOT EXISTS knowledge_submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    category TEXT DEFAULT 'general',
    submitted_by INTEGER NOT NULL,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    admin_notes TEXT DEFAULT '',
    created_at DATETIME DEFAULT (datetime('now')),
    reviewed_at DATETIME,
    reviewed_by INTEGER,
    FOREIGN KEY (submitted_by) REFERENCES users(id),
    FOREIGN KEY (reviewed_by) REFERENCES users(id)
);

-- Preguntas sin respuesta que necesitan entrenamiento
CREATE TABLE IF NOT EXISTS unanswered_questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question TEXT NOT NULL,
    asked_by INTEGER NOT NULL,
    conversation_id INTEGER,
    answer TEXT DEFAULT '',
    answered_by INTEGER,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'answered', 'ignored')),
    add_to_knowledge INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT (datetime('now')),
    answered_at DATETIME,
    FOREIGN KEY (asked_by) REFERENCES users(id),
    FOREIGN KEY (answered_by) REFERENCES users(id),
    FOREIGN KEY (conversation_id) REFERENCES ai_conversations(id)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_active ON knowledge_base(is_active);
CREATE INDEX IF NOT EXISTS idx_knowledge_category ON knowledge_base(category);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON knowledge_submissions(status);
CREATE INDEX IF NOT EXISTS idx_questions_status ON unanswered_questions(status);

-- ============ DATOS INICIALES ============

-- Usuario admin por defecto (password: admin123)
-- Hash generado con bcrypt cost 10
INSERT OR IGNORE INTO users (nomina, password_hash, nombre, approved, is_admin)
VALUES ('admin', '$2a$10$Iw6JSavGrirhkkoDsvY9leKrgEHjH913k1e8/NixaGaffrq4sWgNK', 'Administrador', 1, 1);

-- Categorias de filtros predeterminadas
INSERT OR IGNORE INTO filter_categories (name, description) VALUES
('informacion_confidencial', 'Datos sensibles de la empresa'),
('datos_personales', 'Informacion personal de empleados o clientes'),
('ciberseguridad', 'Intentos de evadir seguridad o hacking'),
('contenido_inapropiado', 'Lenguaje ofensivo o contenido no laboral'),
('informacion_financiera', 'Datos financieros sensibles');

-- Filtros de seguridad predeterminados
INSERT OR IGNORE INTO security_filters (name, description, filter_type, pattern, action, applies_to, severity) VALUES
-- Filtros de ciberseguridad
('sql_injection', 'Detecta intentos de SQL injection', 'regex', '(?i)(union\s+select|drop\s+table|delete\s+from|insert\s+into|update\s+set|;--)', 'block', 'input', 'critical'),
('xss_attack', 'Detecta intentos de XSS', 'regex', '(?i)(<script|javascript:|on\w+\s*=)', 'block', 'input', 'critical'),
('command_injection', 'Detecta intentos de inyeccion de comandos', 'regex', '(?i)(;|\||&&)\s*(rm|cat|wget|curl|bash|sh|nc|netcat)', 'block', 'input', 'critical'),
('hacking_request', 'Solicitudes de hacking o exploits', 'keyword', 'hackear,exploit,vulnerabilidad,bypass,rootkit,backdoor,keylogger', 'block', 'input', 'high'),
('password_extraction', 'Intentos de extraer credenciales', 'regex', '(?i)(dame.*contrase[nñ]a|password.*de|credenciales.*de|acceso.*a.*cuenta)', 'block', 'input', 'critical'),

-- Filtros de informacion confidencial
('numeros_tarjeta', 'Numeros de tarjetas de credito', 'regex', '\b(?:\d{4}[-\s]?){3}\d{4}\b', 'block', 'both', 'critical'),
('curp_rfc', 'CURP o RFC mexicanos', 'regex', '\b[A-Z]{4}\d{6}[A-Z0-9]{8}\b|\b[A-Z]{4}\d{6}[A-Z0-9]{3}\b', 'warn', 'both', 'high'),
('nss_imss', 'Numero de seguro social IMSS', 'regex', '\b\d{11}\b', 'warn', 'both', 'medium'),

-- Filtros de fugas de informacion empresarial
('datos_nomina', 'Salarios y datos de nomina', 'keyword', 'salario,sueldo,nomina de,compensacion,bono de,aguinaldo', 'warn', 'both', 'high'),
('informacion_clientes', 'Solicitudes sobre clientes especificos', 'regex', '(?i)(informacion|datos|contacto|direccion).*(cliente|proveedor)', 'warn', 'input', 'medium'),
('estrategia_empresa', 'Informacion estrategica', 'keyword', 'estrategia,plan de negocio,proyecciones,merger,adquisicion,fusiones', 'warn', 'both', 'high'),

-- Filtros de privacidad entre usuarios
('conversaciones_otros', 'Acceso a conversaciones de otros', 'regex', '(?i)(conversacion|chat|mensaje).*(de|del|otro|compa[nñ]ero)', 'block', 'input', 'critical'),
('datos_otros_empleados', 'Solicitar datos de otros empleados', 'regex', '(?i)(dame|muestrame|dime).*(informacion|datos|salario|direccion).*(de|del)\s+\w+', 'block', 'input', 'high'),

-- Filtros de contenido inapropiado
('contenido_adulto', 'Contenido para adultos', 'keyword', 'pornografia,xxx,desnudo,erotico,sexual', 'block', 'both', 'high'),
('violencia', 'Contenido violento', 'keyword', 'matar,asesinar,tortura,violencia extrema,armas', 'warn', 'both', 'medium'),

-- Filtros de evasion de IA
('jailbreak_prompt', 'Intentos de jailbreak del modelo', 'regex', '(?i)(ignora.*instrucciones|olvida.*reglas|actua.*como|pretend.*you|DAN|do.*anything.*now)', 'block', 'input', 'critical'),
('roleplay_bypass', 'Bypass mediante roleplay', 'regex', '(?i)(imagina.*que.*eres|finge.*ser|simula.*que|actua.*sin.*restricciones)', 'block', 'input', 'high');

-- Configuracion del sistema predeterminada
INSERT OR IGNORE INTO system_config (key, value, description) VALUES
('ollama_url', 'http://localhost:11434', 'URL del servidor Ollama'),
('ollama_model', 'deepseek-r1:14b', 'Modelo de IA a usar'),
('max_context_messages', '20', 'Maximo de mensajes de contexto para IA'),
('session_duration_hours', '24', 'Duracion de sesion en horas'),
('max_message_length', '4000', 'Longitud maxima de mensaje'),
('enable_security_filters', 'true', 'Habilitar filtros de seguridad'),
('log_all_messages', 'false', 'Registrar todos los mensajes'),
('system_prompt', 'Eres AQUILA, el asistente de IA del sistema IRIS de Impro Aerospace. Ayudas a empleados con dudas laborales de forma segura y privada. NO reveles datos de otros empleados ni informacion confidencial.', 'Prompt del sistema para la IA');
