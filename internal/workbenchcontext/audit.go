package workbenchcontext

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	AuditActionOrganizationSwitch = "organization.switch"

	AuditResultSuccess                   = "SUCCESS"
	AuditResultInvalidRequest            = "INVALID_REQUEST"
	AuditResultOrganizationAccessDenied  = "ORGANIZATION_ACCESS_DENIED"
	AuditResultOrganizationAccessRevoked = "ORGANIZATION_ACCESS_REVOKED"
	AuditResultOrganizationSuspended     = "ORGANIZATION_SUSPENDED"
	AuditResultSelectionRequired         = "ORGANIZATION_SELECTION_REQUIRED"
	AuditResultPermissionDenied          = "PERMISSION_DENIED"
	AuditResultDependencyUnavailable     = "DEPENDENCY_UNAVAILABLE"
)

// AuditEvent is the narrow, non-secret identity audit contract. It cannot
// carry bearer tokens, cookies, request payloads, or provider responses.
type AuditEvent struct {
	Subject                 string
	HomeOrganizationID      string
	EffectiveOrganizationID string
	Resource                string
	Action                  string
	Result                  string
	Timestamp               time.Time
	RequestID               string
}

// AuditRecorder records high-risk Workbench identity decisions.
type AuditRecorder interface {
	Record(context.Context, AuditEvent) error
}

type structuredAuditRecorder struct {
	logger *logrus.Logger
}

// NewStructuredAuditRecorder adapts the process structured logger to the
// Workbench identity audit port without introducing another audit database.
func NewStructuredAuditRecorder(logger *logrus.Logger) AuditRecorder {
	return &structuredAuditRecorder{logger: logger}
}

func (recorder *structuredAuditRecorder) Record(_ context.Context, event AuditEvent) error {
	event.Subject = strings.TrimSpace(event.Subject)
	event.HomeOrganizationID = strings.TrimSpace(event.HomeOrganizationID)
	event.EffectiveOrganizationID = strings.TrimSpace(event.EffectiveOrganizationID)
	event.Resource = strings.TrimSpace(event.Resource)
	event.Action = strings.TrimSpace(event.Action)
	event.Result = strings.TrimSpace(event.Result)
	event.RequestID = strings.TrimSpace(event.RequestID)
	if recorder == nil || recorder.logger == nil {
		return errors.New("workbench audit logger is not configured")
	}
	if event.Subject == "" || event.HomeOrganizationID == "" || event.Resource == "" || event.Action == "" || event.Result == "" || event.Timestamp.IsZero() {
		return errors.New("workbench audit event is incomplete")
	}
	recorder.logger.WithFields(logrus.Fields{
		"audit_type":                "workbench_identity",
		"subject":                   event.Subject,
		"home_organization_id":      event.HomeOrganizationID,
		"effective_organization_id": event.EffectiveOrganizationID,
		"resource":                  event.Resource,
		"action":                    event.Action,
		"result":                    event.Result,
		"event_timestamp":           event.Timestamp.UTC().Format(time.RFC3339Nano),
		"request_id":                event.RequestID,
	}).Info("workbench identity audit")
	return nil
}
