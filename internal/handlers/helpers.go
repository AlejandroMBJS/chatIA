package handlers

import (
	"net/http"

	"chat-empleados/internal/i18n"
	"chat-empleados/internal/middleware"
)

// TemplateData crea el mapa base para templates con traducciones
func TemplateData(r *http.Request, data map[string]interface{}) map[string]interface{} {
	if data == nil {
		data = make(map[string]interface{})
	}

	lang := middleware.GetLanguageFromContext(r.Context())
	data["Lang"] = string(lang)
	data["T"] = i18n.TrMap(lang)
	data["IsEnglish"] = lang == i18n.English
	data["IsSpanish"] = lang == i18n.Spanish

	return data
}

// Tr es un helper para obtener traducciones en handlers
func Tr(r *http.Request, key string) string {
	lang := middleware.GetLanguageFromContext(r.Context())
	return i18n.Tr(lang, key)
}
