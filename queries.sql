-- ============ USERS ============

-- name: CreateUser :one
INSERT INTO users (nomina, password_hash, nombre, departamento)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetUserByNomina :one
SELECT * FROM users WHERE nomina = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetPendingUsers :many
SELECT id, nomina, nombre, departamento, created_at
FROM users
WHERE approved = 0 AND is_admin = 0
ORDER BY created_at DESC;

-- name: GetApprovedUsers :many
SELECT id, nomina, nombre, departamento, created_at
FROM users
WHERE approved = 1
ORDER BY nombre ASC;

-- name: GetAllUsers :many
SELECT id, nomina, nombre, departamento, approved, is_admin, created_at
FROM users
ORDER BY created_at DESC;

-- name: ApproveUser :execresult
UPDATE users SET approved = 1, updated_at = datetime('now')
WHERE id = ? AND approved = 0;

-- name: RejectUser :execresult
DELETE FROM users WHERE id = ? AND approved = 0 AND is_admin = 0;

-- name: UpdateUserDepartamento :execresult
UPDATE users SET departamento = ?, updated_at = datetime('now') WHERE id = ?;

-- name: DeleteUser :execresult
DELETE FROM users WHERE id = ? AND is_admin = 0;

-- ============ SESSIONS ============

-- name: CreateSession :one
INSERT INTO sessions (user_id, token, expires_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetSessionByToken :one
SELECT
    s.id, s.user_id, s.token, s.expires_at,
    u.nomina, u.nombre, u.is_admin, u.approved, u.departamento
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token = ? AND s.expires_at > datetime('now');

-- name: DeleteSession :execresult
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :execresult
DELETE FROM sessions WHERE expires_at <= datetime('now');

-- name: DeleteUserSessions :execresult
DELETE FROM sessions WHERE user_id = ?;

-- name: CountActiveSessions :one
SELECT COUNT(*) as count FROM sessions WHERE expires_at > datetime('now');

-- ============ GROUP CHAT ============

-- name: CreateGroupMessage :one
INSERT INTO group_messages (user_id, content)
VALUES (?, ?)
RETURNING *;

-- name: GetRecentGroupMessages :many
SELECT
    gm.id, gm.content, gm.created_at,
    u.id as user_id, u.nombre, u.nomina
FROM group_messages gm
JOIN users u ON gm.user_id = u.id
ORDER BY gm.created_at DESC
LIMIT ?;

-- name: GetGroupMessagesSince :many
SELECT
    gm.id, gm.content, gm.created_at,
    u.id as user_id, u.nombre, u.nomina
FROM group_messages gm
JOIN users u ON gm.user_id = u.id
WHERE gm.id > ?
ORDER BY gm.created_at ASC;

-- name: CountGroupMessages :one
SELECT COUNT(*) as count FROM group_messages;

-- ============ AI CONVERSATIONS ============

-- name: CreateAIConversation :one
INSERT INTO ai_conversations (user_id, title)
VALUES (?, ?)
RETURNING *;

-- name: GetUserConversations :many
SELECT id, title, created_at, updated_at
FROM ai_conversations
WHERE user_id = ?
ORDER BY updated_at DESC
LIMIT 50;

-- name: GetConversation :one
SELECT * FROM ai_conversations
WHERE id = ? AND user_id = ?;

-- name: GetConversationByID :one
SELECT * FROM ai_conversations WHERE id = ?;

-- name: UpdateConversationTitle :execresult
UPDATE ai_conversations
SET title = ?, updated_at = datetime('now')
WHERE id = ? AND user_id = ?;

-- name: TouchConversation :execresult
UPDATE ai_conversations
SET updated_at = datetime('now')
WHERE id = ?;

-- name: DeleteConversation :execresult
DELETE FROM ai_conversations WHERE id = ? AND user_id = ?;

-- name: CountUserConversations :one
SELECT COUNT(*) as count FROM ai_conversations WHERE user_id = ?;

-- ============ AI MESSAGES ============

-- name: CreateAIMessage :one
INSERT INTO ai_messages (conversation_id, role, content, filtered, filter_reason)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetConversationMessages :many
SELECT id, role, content, filtered, filter_reason, created_at
FROM ai_messages
WHERE conversation_id = ?
ORDER BY created_at ASC;

-- name: GetRecentConversationMessages :many
SELECT id, role, content, filtered, filter_reason, created_at
FROM ai_messages
WHERE conversation_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: CountConversationMessages :one
SELECT COUNT(*) as count FROM ai_messages WHERE conversation_id = ?;

-- name: GetFilteredMessages :many
SELECT
    m.id, m.role, m.content, m.filter_reason, m.created_at,
    c.user_id, u.nombre, u.nomina
FROM ai_messages m
JOIN ai_conversations c ON m.conversation_id = c.id
JOIN users u ON c.user_id = u.id
WHERE m.filtered = 1
ORDER BY m.created_at DESC
LIMIT ?;

-- ============ SECURITY FILTERS ============

-- name: CreateSecurityFilter :one
INSERT INTO security_filters (name, description, filter_type, pattern, action, applies_to, severity, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSecurityFilterByID :one
SELECT * FROM security_filters WHERE id = ?;

-- name: GetActiveSecurityFilters :many
SELECT * FROM security_filters
WHERE is_active = 1
ORDER BY severity DESC, name ASC;

-- name: GetAllSecurityFilters :many
SELECT
    sf.*,
    u.nombre as created_by_name
FROM security_filters sf
LEFT JOIN users u ON sf.created_by = u.id
ORDER BY sf.created_at DESC;

-- name: GetSecurityFiltersByType :many
SELECT * FROM security_filters
WHERE filter_type = ? AND is_active = 1
ORDER BY severity DESC;

-- name: GetSecurityFiltersByAppliesTo :many
SELECT * FROM security_filters
WHERE (applies_to = ? OR applies_to = 'both') AND is_active = 1
ORDER BY severity DESC;

-- name: UpdateSecurityFilter :execresult
UPDATE security_filters
SET name = ?, description = ?, pattern = ?, action = ?, applies_to = ?, severity = ?, is_active = ?, updated_at = datetime('now')
WHERE id = ?;

-- name: ToggleSecurityFilter :execresult
UPDATE security_filters
SET is_active = ?, updated_at = datetime('now')
WHERE id = ?;

-- name: DeleteSecurityFilter :execresult
DELETE FROM security_filters WHERE id = ?;

-- name: CountActiveFilters :one
SELECT COUNT(*) as count FROM security_filters WHERE is_active = 1;

-- ============ FILTER CATEGORIES ============

-- name: GetFilterCategories :many
SELECT * FROM filter_categories ORDER BY name ASC;

-- name: GetActiveFilterCategories :many
SELECT * FROM filter_categories WHERE is_active = 1 ORDER BY name ASC;

-- name: CreateFilterCategory :one
INSERT INTO filter_categories (name, description)
VALUES (?, ?)
RETURNING *;

-- name: UpdateFilterCategory :execresult
UPDATE filter_categories SET description = ?, is_active = ? WHERE id = ?;

-- name: DeleteFilterCategory :execresult
DELETE FROM filter_categories WHERE id = ?;

-- ============ SECURITY LOGS ============

-- name: CreateSecurityLog :one
INSERT INTO security_logs (user_id, filter_id, original_content, action_taken, ip_address, user_agent)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetRecentSecurityLogs :many
SELECT
    sl.*,
    u.nombre, u.nomina,
    sf.name as filter_name, sf.severity
FROM security_logs sl
JOIN users u ON sl.user_id = u.id
LEFT JOIN security_filters sf ON sl.filter_id = sf.id
ORDER BY sl.created_at DESC
LIMIT ?;

-- name: GetSecurityLogsByUser :many
SELECT
    sl.*,
    sf.name as filter_name, sf.severity
FROM security_logs sl
LEFT JOIN security_filters sf ON sl.filter_id = sf.id
WHERE sl.user_id = ?
ORDER BY sl.created_at DESC
LIMIT ?;

-- name: GetSecurityLogsByDateRange :many
SELECT
    sl.*,
    u.nombre, u.nomina,
    sf.name as filter_name, sf.severity
FROM security_logs sl
JOIN users u ON sl.user_id = u.id
LEFT JOIN security_filters sf ON sl.filter_id = sf.id
WHERE sl.created_at BETWEEN ? AND ?
ORDER BY sl.created_at DESC;

-- name: CountSecurityLogsByUser :one
SELECT COUNT(*) as count FROM security_logs WHERE user_id = ?;

-- name: CountSecurityLogsToday :one
SELECT COUNT(*) as count FROM security_logs
WHERE date(created_at) = date('now');

-- name: GetSecurityStats :one
SELECT
    COUNT(*) as total_violations,
    COUNT(DISTINCT user_id) as unique_users,
    (SELECT COUNT(*) FROM security_logs WHERE date(created_at) = date('now')) as today_violations
FROM security_logs;

-- ============ SYSTEM CONFIG ============

-- name: GetConfig :one
SELECT value FROM system_config WHERE key = ?;

-- name: GetAllConfig :many
SELECT * FROM system_config ORDER BY key ASC;

-- name: SetConfig :execresult
INSERT INTO system_config (key, value, description)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now');

-- name: DeleteConfig :execresult
DELETE FROM system_config WHERE key = ?;

-- ============ NOTIFICATIONS ============

-- name: CreateNotification :one
INSERT INTO notifications (user_id, type, title, message)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetUserNotifications :many
SELECT * FROM notifications
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: GetUnreadNotifications :many
SELECT * FROM notifications
WHERE user_id = ? AND read = 0
ORDER BY created_at DESC;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) as count FROM notifications
WHERE user_id = ? AND read = 0;

-- name: MarkNotificationRead :execresult
UPDATE notifications SET read = 1 WHERE id = ? AND user_id = ?;

-- name: MarkAllNotificationsRead :execresult
UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0;

-- name: DeleteOldNotifications :execresult
DELETE FROM notifications WHERE created_at < datetime('now', '-30 days');

-- name: GetAdminUserIDs :many
SELECT id FROM users WHERE is_admin = 1;

-- ============ STATISTICS ============

-- name: GetDashboardStats :one
SELECT
    (SELECT COUNT(*) FROM users WHERE approved = 1) as total_users,
    (SELECT COUNT(*) FROM users WHERE approved = 0 AND is_admin = 0) as pending_users,
    (SELECT COUNT(*) FROM ai_conversations) as total_conversations,
    (SELECT COUNT(*) FROM ai_messages) as total_ai_messages,
    (SELECT COUNT(*) FROM group_messages) as total_group_messages,
    (SELECT COUNT(*) FROM security_logs WHERE date(created_at) = date('now')) as security_incidents_today;
