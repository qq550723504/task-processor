package workbenchcontext

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestStructuredAuditRecorderWritesOnlyTheNarrowIdentityAuditContract(t *testing.T) {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	recorder := NewStructuredAuditRecorder(logger)
	event := AuditEvent{
		Subject: "user-1", HomeOrganizationID: "org-home", EffectiveOrganizationID: "org-target",
		Resource: "/api/v1/workbench/context/effective-organization", Action: AuditActionOrganizationSwitch,
		Result: AuditResultSuccess, Timestamp: time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC), RequestID: "req-1",
	}

	if err := recorder.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	logged := output.String()
	for _, value := range []string{"user-1", "org-home", "org-target", "organization.switch", "req-1"} {
		if !strings.Contains(logged, value) {
			t.Fatalf("structured audit log missing %q: %s", value, logged)
		}
	}
	for _, forbidden := range []string{"bearer", "cookie", "payload", "credential", "authorization_response"} {
		if strings.Contains(strings.ToLower(logged), forbidden) {
			t.Fatalf("structured audit log contains forbidden field %q: %s", forbidden, logged)
		}
	}
}
