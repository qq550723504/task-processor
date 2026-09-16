//go:build integration

package membership

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	domain "task-processor/internal/organization/membership"
)

// This probe consumes only the fresh issue357 harness run explicitly prepared
// for #410. Its separate private credential file is cleaned by the task runner.
func TestNativeMembershipProvider(t *testing.T) {
	runID := os.Getenv("ISSUE410_NATIVE_RUN")
	if runID == "" {
		t.Skip("explicit task-owned native provider run required")
	}
	id, err := uuid.Parse(runID)
	require.NoError(t, err)
	require.Equal(t, id.String(), runID)
	directory := filepath.Join(os.TempDir(), "task-processor-issue357", runID)
	var manifest struct {
		RunID, ProjectID, Project string
		Origins                   struct{ Issuer string }
	}
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Equal(t, runID, manifest.RunID)
	require.Equal(t, "issue357-"+runID, manifest.Project)
	var credentials struct{ OrganizationID, ReadToken, WriteToken, DeniedToken string }
	data, err = os.ReadFile(filepath.Join(directory, "issue410-native-credentials.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &credentials))
	require.NotEqual(t, credentials.ReadToken, credentials.WriteToken)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	reader, err := NewClient(manifest.Origins.Issuer, credentials.ReadToken, manifest.ProjectID, nil)
	require.NoError(t, err)
	_, err = reader.List(ctx, credentials.OrganizationID, domain.PageRequest{Limit: 100})
	require.NoError(t, err, "dedicated org-wide read")
	denied, err := NewClient(manifest.Origins.Issuer, credentials.DeniedToken, manifest.ProjectID, nil)
	require.NoError(t, err)
	_, err = denied.List(ctx, credentials.OrganizationID, domain.PageRequest{Limit: 100})
	require.ErrorIs(t, err, domain.ErrUnavailable, "missing coverage must not become an empty directory")
	writer, err := NewWriter(manifest.Origins.Issuer, credentials.ReadToken, credentials.WriteToken, manifest.ProjectID, nil)
	require.NoError(t, err)
	user := uuid.NewString()
	op := domain.Operation{Scope: domain.OperationScope{ProjectID: manifest.ProjectID, OrganizationID: credentials.OrganizationID, ActorID: "task-probe"}, Key: uuid.NewString(), TargetUserID: user, Kind: domain.CommandInvite, Role: "listingkit_viewer", Step: domain.StepUser, Phase: domain.PhaseDispatched, DispatchID: uuid.NewString(), Invitation: &domain.Invitation{Email: "member-" + user + "@example.test", FirstName: "Issue410", LastName: "Native"}}
	ack, err := writer.Write(ctx, op)
	require.NoError(t, err, "dedicated create human")
	require.Equal(t, user, ack.ID)
	human, err := writer.ReadHuman(ctx, credentials.OrganizationID, user)
	require.NoError(t, err)
	require.Equal(t, user, human.ID)
	require.Equal(t, credentials.OrganizationID, human.OrganizationID)
	op.Step = domain.StepGrant
	op.DispatchID = uuid.NewString()
	ack, err = writer.Write(ctx, op)
	require.NoError(t, err, "dedicated create grant")
	require.NotEmpty(t, ack.ID)
	op.AuthorizationID = ack.ID
	op.Kind = domain.CommandRole
	op.Step = domain.StepRole
	op.DispatchID = uuid.NewString()
	_, err = writer.Write(ctx, op)
	require.NoError(t, err, "dedicated update grant")
	op.Kind = domain.CommandRemove
	op.Step = domain.StepRemove
	op.DispatchID = uuid.NewString()
	_, err = writer.Write(ctx, op)
	require.NoError(t, err, "dedicated delete grant")
	_, err = writer.ReadHuman(ctx, credentials.OrganizationID, user)
	require.NoError(t, err, "grant removal retains user")
}
