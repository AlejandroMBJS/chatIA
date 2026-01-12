package services

import (
	"context"
	"fmt"
	"log"

	"chat-empleados/db"
)

type NotificationService struct {
	queries *db.Queries
}

func NewNotificationService(queries *db.Queries) *NotificationService {
	return &NotificationService{
		queries: queries,
	}
}

// NotifyAdminsNewUser notifica a todos los admins cuando hay un nuevo usuario pendiente
func (n *NotificationService) NotifyAdminsNewUser(ctx context.Context, userName, userNomina string) error {
	adminIDs, err := n.queries.GetAdminUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("error obteniendo admins: %w", err)
	}

	for _, adminID := range adminIDs {
		_, err := n.queries.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:  adminID,
			Type:    "user_pending",
			Title:   "Nuevo usuario pendiente",
			Message: fmt.Sprintf("El usuario %s (%s) solicita acceso al sistema.", userName, userNomina),
		})
		if err != nil {
			log.Printf("[ERROR] Error creando notificacion para admin %d: %v", adminID, err)
		}
	}

	log.Printf("[INFO] Notificacion enviada a %d admins sobre nuevo usuario: %s", len(adminIDs), userNomina)
	return nil
}

// NotifyUserApproved notifica al usuario que su cuenta fue aprobada
func (n *NotificationService) NotifyUserApproved(ctx context.Context, userID int64) error {
	_, err := n.queries.CreateNotification(ctx, db.CreateNotificationParams{
		UserID:  userID,
		Type:    "user_approved",
		Title:   "Cuenta aprobada",
		Message: "Tu cuenta ha sido aprobada. Ya puedes acceder al sistema.",
	})
	if err != nil {
		return fmt.Errorf("error creando notificacion de aprobacion: %w", err)
	}

	log.Printf("[INFO] Notificacion de aprobacion enviada al usuario %d", userID)
	return nil
}

// NotifyUserRejected notifica al usuario que su cuenta fue rechazada
func (n *NotificationService) NotifyUserRejected(ctx context.Context, userID int64) error {
	_, err := n.queries.CreateNotification(ctx, db.CreateNotificationParams{
		UserID:  userID,
		Type:    "user_rejected",
		Title:   "Cuenta rechazada",
		Message: "Tu solicitud de acceso ha sido rechazada. Contacta al administrador para mas informacion.",
	})
	if err != nil {
		return fmt.Errorf("error creando notificacion de rechazo: %w", err)
	}

	log.Printf("[INFO] Notificacion de rechazo enviada al usuario %d", userID)
	return nil
}

// NotifySecurityAlert notifica a los admins sobre una alerta de seguridad
func (n *NotificationService) NotifySecurityAlert(ctx context.Context, userName, filterName string) error {
	adminIDs, err := n.queries.GetAdminUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("error obteniendo admins: %w", err)
	}

	for _, adminID := range adminIDs {
		_, err := n.queries.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:  adminID,
			Type:    "security_alert",
			Title:   "Alerta de seguridad",
			Message: fmt.Sprintf("El usuario %s activo el filtro '%s'.", userName, filterName),
		})
		if err != nil {
			log.Printf("[ERROR] Error creando notificacion de seguridad para admin %d: %v", adminID, err)
		}
	}

	return nil
}

// GetUnreadCount obtiene el conteo de notificaciones no leídas para un usuario
func (n *NotificationService) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	count, err := n.queries.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetUserNotifications obtiene las notificaciones de un usuario
func (n *NotificationService) GetUserNotifications(ctx context.Context, userID int64, limit int64) ([]db.Notification, error) {
	return n.queries.GetUserNotifications(ctx, db.GetUserNotificationsParams{
		UserID: userID,
		Limit:  limit,
	})
}

// MarkAsRead marca una notificación como leída
func (n *NotificationService) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	_, err := n.queries.MarkNotificationRead(ctx, db.MarkNotificationReadParams{
		ID:     notificationID,
		UserID: userID,
	})
	return err
}

// MarkAllAsRead marca todas las notificaciones de un usuario como leídas
func (n *NotificationService) MarkAllAsRead(ctx context.Context, userID int64) error {
	_, err := n.queries.MarkAllNotificationsRead(ctx, userID)
	return err
}
