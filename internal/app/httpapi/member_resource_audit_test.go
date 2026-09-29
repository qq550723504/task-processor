package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/app/accountaudit"
	"task-processor/internal/authz"
	resourceadapter "task-processor/internal/integration/orgresource"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
	"task-processor/internal/workbenchcontext"
)

func TestAccountAuditHTTPReadsNativeMemberResourceFactsWithFreshAuthorization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	require.NoError(t, resourceadapter.AutoMigrate(db))
	require.NoError(t, db.Exec("INSERT INTO saas_organization_resource_buckets(organization_id, resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", "B", "data_row", 100, 0, 0, 0, time.Now().UTC(), time.Now().UTC()).Error)
	transfers, err := resourceadapter.NewGormMemberAllocationRepository(db, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	c := orgresource.MemberResourceTransfer{OrganizationID: "B", MemberID: "member", ActorID: "safe-actor", OperationID: "allocate-1", ResourceType: orgresource.ResourceDataRow, Action: orgresource.MemberResourceAllocate, Quantity: 10}
	_, err = transfers.Transfer(context.Background(), c)
	require.NoError(t, err)
	c.OperationID, c.Action, c.Quantity, c.ExpectedVersion = "reclaim-1", orgresource.MemberResourceReclaim, 3, 1
	_, err = transfers.Transfer(context.Background(), c)
	require.NoError(t, err)
	limits, err := resourceadapter.NewGormMemberLimitRepository(db, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = limits.SetMonthlyLimit(context.Background(), orgresource.SetMemberLimitExecution{OrganizationID: "B", MemberID: "member", ActorID: "safe-actor", OperationID: "limit key", Target: 1000})
	require.NoError(t, err)
	reader, err := resourceadapter.NewGormRepository(db, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	query, err := accountaudit.NewCurrentAuditSources(&auditHTTPHistory{}, nil, nil, nil, nil, memberResourceAuditReader{repository: reader})
	require.NoError(t, err)
	modules := kernelmodule.NewRegistry()
	require.NoError(t, (accountAuditModule{query: query}).Register(modules))
	grants := &auditHTTPGrants{role: "listingkit_viewer"}
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	server := buildIsolatedApplicationHTTPServer(modules.Routes(), routeAuthDependencies{workbenchVerifier: applicationVerifier{}, organizationResolver: workbenchcontext.NewResolver(grants, "project", "v1", nil), authorizer: authorizer}, registry.Timeout)
	get := func(org, query string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", accountAuditPath+query, nil)
		r.Header.Set("Authorization", "Bearer fixture")
		r.Header.Set("X-Requested-Organization-ID", org)
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	for operation, quantity := range map[string]string{"allocate_member_resource": "10", "reclaim_member_resource": "3", "set_member_ai_point_limit": "1000"} {
		w := get("B", "?actor=safe-actor&operation="+operation)
		require.Equal(t, 200, w.Code, w.Body.String())
		var page accountaudit.Page
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.Len(t, page.Items, 1)
		require.Equal(t, "member", page.Items[0].ObjectReference)
		require.Equal(t, quantity, page.Items[0].Resource.Quantity)
		require.Equal(t, operation, page.Items[0].Operation)
		if operation == "set_member_ai_point_limit" {
			require.Equal(t, "limit key", page.Items[0].Relation.Reference)
		}
	}
	require.Equal(t, 400, get("B", "?operation=set_target").Code)
	require.Equal(t, 403, get("A", "").Code)
	grants.revoked = true
	require.Equal(t, 403, get("B", "").Code)
	var count int64
	require.NoError(t, db.Table("saas_organization_resource_operations").Count(&count).Error)
	require.Equal(t, int64(3), count)
}
