package service

import (
	"hawkeye-cli/internal/api"
)

// PaginationFilter represents a filter for session list queries.
type PaginationFilter = api.PaginationFilter

// SessionDisplay holds display-ready session info.
type SessionDisplay struct {
	UUID        string
	Name        string
	Created     string
	TypeIcon    string
	Status      string
	Pinned      bool
	RawStatus   string
	SessionType string
}

// BuildSessionFilters translates CLI flags into the API filter format.
func BuildSessionFilters(status, from, to, search string, uninvestigated bool) []api.PaginationFilter {
	var filters []api.PaginationFilter

	if uninvestigated {
		status = "not_started"
	}

	if status != "" {
		filters = append(filters, api.PaginationFilter{
			Key:      "investigation_status",
			Value:    normalizeStatus(status),
			Operator: "==",
		})
	}

	if from != "" {
		filters = append(filters, api.PaginationFilter{
			Key:      "create_time",
			Value:    from,
			Operator: "gte",
		})
	}

	if to != "" {
		filters = append(filters, api.PaginationFilter{
			Key:      "create_time",
			Value:    to,
			Operator: "lte",
		})
	}

	if search != "" {
		filters = append(filters, api.PaginationFilter{
			Key:      "incident_info.title",
			Value:    search,
			Operator: "in",
		})
	}

	return filters
}

// normalizeStatus converts short status names to the full API enum.
func normalizeStatus(status string) string {
	switch status {
	case "not_started":
		return "INVESTIGATION_STATUS_NOT_STARTED"
	case "in_progress":
		return "INVESTIGATION_STATUS_IN_PROGRESS"
	case "investigated":
		return "INVESTIGATION_STATUS_COMPLETED"
	case "completed":
		return "INVESTIGATION_STATUS_COMPLETED"
	default:
		return status
	}
}

// FormatSessionRow maps a raw SessionInfo to a display-ready struct.
func FormatSessionRow(s api.SessionInfo) SessionDisplay {
	name := s.Name
	if name == "" {
		name = "(unnamed)"
	}

	typeIcon := "💬"
	if s.SessionType == "SESSION_TYPE_INCIDENT" {
		typeIcon = "🚨"
	}

	return SessionDisplay{
		UUID:        s.SessionUUID,
		Name:        name,
		Created:     s.CreateTime,
		TypeIcon:    typeIcon,
		Status:      s.InvestigationStatus,
		Pinned:      s.Pinned,
		RawStatus:   s.InvestigationStatus,
		SessionType: s.SessionType,
	}
}
