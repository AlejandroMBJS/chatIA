package services

import (
	"context"
	"database/sql"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"chat-empleados/db"
)

type FilterResult struct {
	Blocked      bool
	FilterID     int64
	FilterName   string
	Action       string
	Severity     string
	Reason       string
	MatchedText  string
}

type SecurityFilter struct {
	ID          int64
	Name        string
	Description string
	FilterType  string
	Pattern     string
	Action      string
	AppliesTo   string
	Severity    string
	compiled    *regexp.Regexp
	keywords    []string
}

type SecurityService struct {
	queries      *db.Queries
	filters      []SecurityFilter
	filtersMutex sync.RWMutex
	lastUpdate   time.Time
}

func NewSecurityService(queries *db.Queries) *SecurityService {
	s := &SecurityService{
		queries: queries,
	}
	s.ReloadFilters(context.Background())
	return s
}

func (s *SecurityService) ReloadFilters(ctx context.Context) error {
	s.filtersMutex.Lock()
	defer s.filtersMutex.Unlock()

	dbFilters, err := s.queries.GetActiveSecurityFilters(ctx)
	if err != nil {
		log.Printf("[ERROR] Error cargando filtros de seguridad: %v", err)
		return err
	}

	s.filters = make([]SecurityFilter, 0, len(dbFilters))

	for _, f := range dbFilters {
		sf := SecurityFilter{
			ID:          f.ID,
			Name:        f.Name,
			Description: stringValue(f.Description),
			FilterType:  f.FilterType,
			Pattern:     f.Pattern,
			Action:      f.Action,
			AppliesTo:   stringValue(f.AppliesTo),
			Severity:    stringValue(f.Severity),
		}

		switch f.FilterType {
		case "regex":
			compiled, err := regexp.Compile(f.Pattern)
			if err != nil {
				log.Printf("[WARN] Filtro regex invalido '%s': %v", f.Name, err)
				continue
			}
			sf.compiled = compiled
		case "keyword":
			sf.keywords = strings.Split(strings.ToLower(f.Pattern), ",")
			for i := range sf.keywords {
				sf.keywords[i] = strings.TrimSpace(sf.keywords[i])
			}
		}

		s.filters = append(s.filters, sf)
	}

	s.lastUpdate = time.Now()
	log.Printf("[INFO] Cargados %d filtros de seguridad activos", len(s.filters))
	return nil
}

func (s *SecurityService) CheckInput(ctx context.Context, content string) *FilterResult {
	return s.checkContent(ctx, content, "input")
}

func (s *SecurityService) CheckOutput(ctx context.Context, content string) *FilterResult {
	return s.checkContent(ctx, content, "output")
}

func (s *SecurityService) checkContent(ctx context.Context, content string, direction string) *FilterResult {
	s.filtersMutex.RLock()
	defer s.filtersMutex.RUnlock()

	contentLower := strings.ToLower(content)

	for _, filter := range s.filters {
		if filter.AppliesTo != "both" && filter.AppliesTo != direction {
			continue
		}

		var matched bool
		var matchedText string

		switch filter.FilterType {
		case "regex":
			if filter.compiled != nil {
				if match := filter.compiled.FindString(content); match != "" {
					matched = true
					matchedText = match
				}
			}
		case "keyword":
			for _, keyword := range filter.keywords {
				if strings.Contains(contentLower, keyword) {
					matched = true
					matchedText = keyword
					break
				}
			}
		case "category":
			if strings.Contains(contentLower, strings.ToLower(filter.Pattern)) {
				matched = true
				matchedText = filter.Pattern
			}
		}

		if matched {
			result := &FilterResult{
				FilterID:    filter.ID,
				FilterName:  filter.Name,
				Action:      filter.Action,
				Severity:    filter.Severity,
				MatchedText: matchedText,
			}

			switch filter.Action {
			case "block":
				result.Blocked = true
				result.Reason = "Contenido bloqueado por politica de seguridad: " + filter.Name
			case "warn":
				result.Blocked = false
				result.Reason = "Advertencia de seguridad: " + filter.Name
			case "log":
				result.Blocked = false
				result.Reason = "Contenido registrado: " + filter.Name
			}

			return result
		}
	}

	return nil
}

func (s *SecurityService) LogViolation(ctx context.Context, userID int64, filterID sql.NullInt64, content, action, ip, userAgent string) error {
	_, err := s.queries.CreateSecurityLog(ctx, db.CreateSecurityLogParams{
		UserID:          userID,
		FilterID:        filterID,
		OriginalContent: content,
		ActionTaken:     action,
		IpAddress:       sql.NullString{String: ip, Valid: ip != ""},
		UserAgent:       sql.NullString{String: userAgent, Valid: userAgent != ""},
	})
	return err
}

func (s *SecurityService) GetFilters() []SecurityFilter {
	s.filtersMutex.RLock()
	defer s.filtersMutex.RUnlock()

	result := make([]SecurityFilter, len(s.filters))
	copy(result, s.filters)
	return result
}

func (s *SecurityService) GetFilterStats(ctx context.Context) (db.GetSecurityStatsRow, error) {
	return s.queries.GetSecurityStats(ctx)
}

func (s *SecurityService) SanitizeForDisplay(content string) string {
	// Escapar caracteres HTML peligrosos
	content = strings.ReplaceAll(content, "&", "&amp;")
	content = strings.ReplaceAll(content, "<", "&lt;")
	content = strings.ReplaceAll(content, ">", "&gt;")
	content = strings.ReplaceAll(content, "\"", "&quot;")
	content = strings.ReplaceAll(content, "'", "&#39;")

	// Eliminar URLs de javascript
	contentLower := strings.ToLower(content)
	if strings.Contains(contentLower, "javascript:") {
		content = regexp.MustCompile(`(?i)javascript:`).ReplaceAllString(content, "")
	}
	if strings.Contains(contentLower, "data:") {
		content = regexp.MustCompile(`(?i)data:\s*text/html`).ReplaceAllString(content, "")
	}
	if strings.Contains(contentLower, "vbscript:") {
		content = regexp.MustCompile(`(?i)vbscript:`).ReplaceAllString(content, "")
	}

	return content
}

// SanitizeForStorage limpia el contenido antes de guardarlo en la base de datos
func (s *SecurityService) SanitizeForStorage(content string) string {
	// Eliminar caracteres nulos y de control
	content = strings.Map(func(r rune) rune {
		if r == 0 || (r >= 1 && r <= 31 && r != 10 && r != 13 && r != 9) {
			return -1
		}
		return r
	}, content)

	// Trim whitespace excesivo
	content = strings.TrimSpace(content)

	return content
}

func (s *SecurityService) ValidateConversationAccess(ctx context.Context, conversationID, userID int64) (bool, error) {
	conv, err := s.queries.GetConversationByID(ctx, conversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return conv.UserID == userID, nil
}

func stringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
