//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/app/accountaudit"
	schema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	resourceadapter "task-processor/internal/integration/orgresource"
	profile "task-processor/internal/integration/persistence/accountprofile"
	memberstore "task-processor/internal/integration/persistence/organization/membership"
	store "task-processor/internal/integration/persistence/sourceaccountregistry"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	membership "task-processor/internal/organization/membership"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

func TestAccountAuditSummaryCommittedOwnersPostgres(t *testing.T) {
	owner, connection := commercialPostgres(t)
	ctx := context.Background()
	require.NoError(t, schema.Migrate(ctx, owner))
	pool, err := owner.DB()
	require.NoError(t, err)
	tx, err := pool.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, memberstore.InstallSchemaTx(ctx, tx))
	require.NoError(t, tx.Commit())
	require.NoError(t, resourceadapter.AutoMigrate(owner))
	auth, _ := authz.NewListingKitAuthorizer(nil, nil)
	repository, err := store.NewRepository(ctx, owner)
	require.NoError(t, err)
	at := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	service, err := registry.NewService(repository, auth, registry.WithClock(func() time.Time { return at }))
	require.NoError(t, err)
	scoped := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "u1", TenantID: "B", EffectiveOrganizationID: "B", Roles: []string{"listingkit_operator"}, TokenExpiresAt: at.Add(time.Hour)})
	for i := 0; i < 25; i++ {
		_, err = service.Register(scoped, uuid.NewString(), registry.RegisterInput{DisplayName: fmt.Sprintf("summary-%d", i), Platform: "1688"})
		require.NoError(t, err)
	}
	profiles, err := profile.New(owner)
	require.NoError(t, err)
	for i := 0; i < 105; i++ {
		_, err = profiles.SaveWithAudit(ctx, profile.BusinessProfile{OrganizationID: "B", UserID: "u1", UserRole: fmt.Sprintf("role-%d", i)}, profile.AuditContext{OrganizationID: "B", ActorID: "u1"})
		require.NoError(t, err)
	}
	_, err = profiles.SaveWithAudit(ctx, profile.BusinessProfile{OrganizationID: "A", UserID: "u1"}, profile.AuditContext{OrganizationID: "A", ActorID: "u1"})
	require.NoError(t, err)
	members, err := memberstore.NewRepository(ctx, owner, "project")
	require.NoError(t, err)
	original := membership.Operation{Scope: membership.OperationScope{ProjectID: "project", OrganizationID: "B", ActorID: "u1"}, Key: uuid.NewString(), Fingerprint: strings.Repeat("a", 64), Kind: membership.CommandRole, TargetUserID: "member", AuthorizationID: "grant", Role: "listingkit_viewer", ExpectedVersion: strings.Repeat("b", 64), Step: membership.StepRole, Phase: membership.PhaseReady, Revision: 1}
	started, err := members.Begin(ctx, original)
	require.NoError(t, err)
	dispatched, err := members.Apply(ctx, original.Scope, original.Key, started.Revision, membership.OperationChange{Event: membership.EventDispatch, DispatchID: uuid.NewString()})
	require.NoError(t, err)
	completed, err := members.Apply(ctx, original.Scope, original.Key, dispatched.Revision, membership.OperationChange{Event: membership.EventAcknowledge, DispatchID: dispatched.DispatchID, Acknowledgment: &membership.Acknowledgment{ID: "grant", At: time.Now().UTC().Format(time.RFC3339Nano)}})
	require.NoError(t, err)
	require.Equal(t, membership.PhaseCompleted, completed.Phase)
	_, err = members.Begin(ctx, original)
	require.NoError(t, err) // Original receipt replay.
	pending := original
	pending.Key = uuid.NewString()
	pending.TargetUserID = "pending"
	_, err = members.Begin(ctx, pending)
	require.NoError(t, err) // Not an audit success.
	require.NoError(t, owner.Exec("INSERT INTO saas_organization_resource_buckets(organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES('B','data_row',100,0,0,0,now(),now())").Error)
	transfers, err := resourceadapter.NewGormMemberAllocationRepository(owner, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	command := orgresource.MemberResourceTransfer{OrganizationID: "B", MemberID: "member", ActorID: "u1", OperationID: "allocate-summary", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10}
	_, err = transfers.Transfer(ctx, command)
	require.NoError(t, err)
	_, err = transfers.Transfer(ctx, command)
	require.NoError(t, err)
	command.OperationID, command.Action, command.ExpectedVersion, command.Quantity = "reclaim-summary", orgresource.MemberResourceReclaim, 1, 3
	_, err = transfers.Transfer(ctx, command)
	require.NoError(t, err)
	limits, err := resourceadapter.NewGormMemberLimitRepository(owner, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: "B", MemberID: "member", ActorID: "u1", OperationID: "limit-summary", Target: 1000})
	require.NoError(t, err)

	// Existing resource runtime role/grants, installed only in this temporary DB.
	require.NoError(t, owner.Exec("CREATE ROLE commercial_owner_runtime LOGIN PASSWORD 'synthetic-summary-password'").Error)
	for _, sql := range []string{
		"REVOKE CREATE ON SCHEMA public FROM PUBLIC",
		"GRANT USAGE ON SCHEMA public TO commercial_owner_runtime",
		"GRANT SELECT,INSERT,UPDATE ON saas_organization_resource_buckets,saas_organization_resource_operations,saas_organization_resource_reservations,saas_organization_resource_debts,saas_member_ai_point_limits,saas_member_ai_point_months,saas_member_resource_positions TO commercial_owner_runtime",
		"GRANT SELECT,INSERT ON saas_organization_resource_source_claims,saas_organization_resource_events,saas_organization_resource_audit_logs TO commercial_owner_runtime",
		"GRANT USAGE,SELECT ON SEQUENCE saas_organization_resource_audit_logs_id_seq TO commercial_owner_runtime",
	} {
		require.NoError(t, owner.Exec(sql).Error)
	}
	runtimeDB, err := gorm.Open(postgres.Open(fmt.Sprintf("host=127.0.0.1 port=%d dbname=issue347 user=commercial_owner_runtime password=synthetic-summary-password sslmode=disable", connection.Port)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	runtimePool, err := runtimeDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { runtimePool.Close() })
	module, err := buildAccountAuditModule(ctx, owner, owner, runtimeDB, nil, auth, "project")
	require.NoError(t, err)
	modules := kernelmodule.NewRegistry()
	require.NoError(t, module.Register(modules))
	grants := &auditHTTPGrants{role: "listingkit_viewer"}
	server := buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: auth}, registry.Timeout)
	before := commercialTableSnapshot(t, owner)
	get := func(org, path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("GET", accountAuditPath+path, nil)
		request.Header.Set("Authorization", "Bearer fixture")
		request.Header.Set("X-Requested-Organization-ID", org)
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, request)
		return response
	}
	response := get("B", "/summary")
	require.Equal(t, 200, response.Code, response.Body.String())
	var summary accountaudit.Summary
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &summary))
	require.Equal(t, accountaudit.SummaryCounts{Operations: "134", Members: "1", Permissions: "1", Resources: "3"}, summary.Counts)
	response = get("B", "?limit=20")
	require.Equal(t, 200, response.Code)
	var page accountaudit.Page
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.Len(t, page.Items, 20)
	require.NotNil(t, page.NextCursor)
	require.Equal(t, 400, get("B", "/summary?limit=20").Code)
	require.Equal(t, 403, get("A", "/summary").Code)
	grants.revoked = true
	require.Equal(t, 403, get("B", "/summary").Code)
	grants.revoked = false
	require.NoError(t, owner.Exec("REVOKE SELECT ON saas_organization_resource_events FROM commercial_owner_runtime").Error)
	response = get("B", "/summary")
	require.Equal(t, 503, response.Code)
	require.NotContains(t, response.Body.String(), "\"counts\"")
	require.Equal(t, before, commercialTableSnapshot(t, owner), "summary must not mutate resource facts")
}
