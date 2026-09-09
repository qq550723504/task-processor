package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	contract "task-processor/internal/marketplace/validator"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/workbenchcontext"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Only external identity verification and grant acquisition are substitutes.
// All middleware, Organization selection, authorization, Publisher, stores,
// assembler, evaluator and HTTP handlers are the production implementations.
type sharedRegressionGrants struct{ diagnosticGrants }

func (g *sharedRegressionGrants) Load(ctx context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	if request.Subject == "switcher" {
		return workbenchcontext.GrantResult{Source: source, Grants: []authidentity.OrganizationGrant{
			{OrganizationID: "200", ProjectID: "project", Roles: []string{"listingkit_operator"}},
			{OrganizationID: "300", ProjectID: "project", Roles: []string{"listingkit_operator"}},
		}}, nil
	}
	return g.diagnosticGrants.Load(ctx, source, request)
}

func sharedRegressionApplication(t *testing.T, db *gorm.DB, g *sharedRegressionGrants) (*httptest.Server, record.Reader) {
	t.Helper()
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	server, reader, err := NewSheinRecordApplication(db, recordVerifier{}, workbenchcontext.NewResolver(g, "project", "v1", nil), auth)
	require.NoError(t, err)
	ts := httptest.NewUnstartedServer(server.Handler)
	ts.Config.ReadTimeout, ts.Config.WriteTimeout = server.ReadTimeout, server.WriteTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts, reader
}

func sharedPost(t *testing.T, server *httptest.Server, subject, org, operation string, input record.Input) (int, []byte) {
	t.Helper()
	body, err := json.Marshal(input)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/listing/shein-records", strings.NewReader(string(body)))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+subject)
	req.Header.Set("X-Requested-Organization-ID", org)
	req.Header.Set("Idempotency-Key", operation)
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, raw
}

func TestSharedRegressionPostgres(t *testing.T) {
	m, cases := loadSharedRegression(t, "postgres")
	if os.Getenv("ISSUE376_TEST_DSN") == "" {
		t.Skip("layer=postgres result=SKIP: ISSUE376_TEST_DSN must target task-isolated PostgreSQL; not acceptance PASS")
	}
	consumed := []string{}
	for _, c := range cases {
		if !slices.Contains(c.Layers, "postgres") {
			continue
		}
		consumed = append(consumed, c.CaseID)
		t.Run(c.CaseID, func(t *testing.T) {
			db := recordTestDB(t)
			var environment struct{ Version, Schema string }
			require.NoError(t, db.Raw("SELECT version() AS version, current_schema() AS schema").Scan(&environment).Error)
			t.Logf("environment=%s isolated_schema=%s", environment.Version, environment.Schema)
			snapshot, err := sourcing.ToSnapshot(*c.Source)
			require.NoError(t, err)
			require.Equal(t, *c.Snapshot, snapshot)
			repository, err := catalogstore.NewRepository(db)
			require.NoError(t, err)
			publisher, err := catalog.NewPublisher(repository)
			require.NoError(t, err)
			publicationID := uuid.NewString()
			request := catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: "200", ProductKey: c.CaseID}, PublicationID: publicationID, Snapshot: snapshot}
			published, err := publisher.Publish(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, request.Identity, published.Identity)
			require.Equal(t, publicationID, published.PublicationID)
			require.EqualValues(t, 1, published.Version)
			require.Equal(t, *c.Snapshot, published.Snapshot)
			prepareRecordStore(t, db, "200")
			prepareApprovedAssets(t, db, "200", c.CaseID, published.Version)
			replayedPublication, err := publisher.Publish(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, published, replayedPublication)
			g := &sharedRegressionGrants{}
			server, reader := sharedRegressionApplication(t, db, g)
			input := sharedRecordInput(c, published.Version)
			operation := uuid.NewString()
			status, raw := sharedPost(t, server, "operator", "200", operation, input)
			require.Equal(t, 201, status, string(raw))
			var receipt record.Receipt
			sharedDecode(t, raw, &receipt)
			id, err := uuid.Parse(receipt.RecordID)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, id)
			require.Equal(t, receipt.RecordID, id.String())
			stored, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
			require.NoError(t, err)
			require.Equal(t, receipt.RecordID, stored.ID)
			require.Equal(t, "200", stored.OrganizationID)
			require.Equal(t, "operator", stored.OwnerUserID)
			require.Equal(t, operation, stored.OperationID)
			require.Equal(t, input, stored.Input)
			require.False(t, stored.CreatedAt.IsZero())
			require.False(t, stored.CreatedAt.After(stored.ReadAt))
			assertSharedIncompletePackage(t, stored.Payload, c)
			var durable []byte
			require.NoError(t, db.Raw("SELECT payload FROM listing_shein_records WHERE id=?", receipt.RecordID).Row().Scan(&durable))
			require.Equal(t, durable, stored.Payload)

			before := diagnosticBusinessState(t, db) // Setup writes intentionally precede the read-only window.
			first := map[contract.Action]contract.DiagnosticResult{}
			for _, want := range c.Reports {
				start := time.Now().UTC()
				status, raw = diagnosticGet(t, server, receipt.RecordID, "action="+string(want.Action), "operator", "200")
				end := time.Now().UTC()
				require.Equal(t, 200, status, string(raw))
				var got contract.DiagnosticResult
				sharedDecode(t, raw, &got)
				assertSharedReport(t, m, want, got, true) // Oracle is shared data, adjusted only for the controlled exact approved image.
				assertSharedTimes(t, start, end, got)
				first[want.Action] = got
			}
			require.NotEqual(t, first[contract.Publish].Input.Digest, first[contract.SaveDraft].Input.Digest)
			require.Equal(t, before, diagnosticBusinessState(t, db))
			server.Close()
			server, reader = sharedRegressionApplication(t, db, g) // Reconstruct repositories, use cases and server.
			for _, want := range c.Reports {
				previous := first[want.Action]
				start := time.Now().UTC()
				status, raw = diagnosticGet(t, server, receipt.RecordID, "action="+string(want.Action)+"&expected_digest="+previous.Input.Digest, "operator", "")
				require.Equal(t, 200, status, string(raw))
				var again contract.DiagnosticResult
				sharedDecode(t, raw, &again)
				assertSharedReport(t, m, want, again, true)
				assertSharedTimes(t, start, time.Now().UTC(), again)
				require.False(t, again.Input.ReadAt.Before(previous.Input.ReadAt))
				require.Equal(t, previous.Input.Digest, again.Input.Digest)
				require.Equal(t, previous.OfflineChecks, again.OfflineChecks)
			}
			again, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
			require.NoError(t, err)
			require.False(t, again.ReadAt.Before(stored.ReadAt))
			// Compare every durable identity, receipt linkage, option, payload and creation timestamp.
			again.ReadAt = stored.ReadAt
			require.Equal(t, stored, again)
			status, raw = sharedPost(t, server, "operator", "200", operation, input)
			require.Equal(t, 201, status, string(raw))
			var replay record.Receipt
			sharedDecode(t, raw, &replay)
			require.Equal(t, receipt, replay)
			changed := input
			changed.SnapshotVersion++
			status, raw = sharedPost(t, server, "operator", "200", operation, changed)
			require.Equal(t, 409, status, string(raw))
			status, raw = diagnosticGet(t, server, receipt.RecordID, "action=publish&expected_digest=sha256:"+strings.Repeat("a", 64), "operator", "200")
			require.Equal(t, 409, status)
			require.JSONEq(t, `{"error":"stale_input"}`, string(raw))
			for _, access := range []struct {
				user, org string
				status    int
			}{
				{"operator", "200", 200}, {"readonly", "200", 200}, {"admin", "200", 200},
				{"other", "200", 404}, {"foreign-admin", "100", 404}, {"operator", "100", 403},
				{"viewer", "200", 403}, {"store", "200", 403}, {"no-grant", "200", 403}, {"", "200", 401},
			} {
				status, raw = diagnosticGet(t, server, receipt.RecordID, "action=publish", access.user, access.org)
				require.Equal(t, access.status, status, string(raw))
				if status != 200 {
					require.NotContains(t, string(raw), "actual_digest")
					require.NotContains(t, string(raw), receipt.RecordID)
					require.NotContains(t, string(raw), c.CaseID)
				}
			}
			for _, user := range []string{"readonly", "store"} {
				status, raw = sharedPost(t, server, user, "200", uuid.NewString(), input)
				require.Equal(t, 403, status, string(raw))
			}
			g.revoked.Store(true)
			status, _ = diagnosticGet(t, server, receipt.RecordID, "action=publish", "operator", "200")
			require.Equal(t, 403, status)
			status, _ = sharedPost(t, server, "operator", "200", operation, input)
			require.Equal(t, 403, status)
			g.revoked.Store(false)
			require.Equal(t, before, diagnosticBusinessState(t, db), "GET, denied writes and idempotent replay preserve business rows and xmin")
			current, err := repository.GetCurrentSnapshot(context.Background(), published.Identity)
			require.NoError(t, err)
			require.Equal(t, published, current)
			assertSharedOrganizationSwitch(t, db, server, reader, publisher, c, input)
			t.Logf("layer=postgres result=PASS case_id=%s publication_id=%s version=%d record_id=%s digest=%s restart/replay/conflict/permissions/read-only=PASS", c.CaseID, publicationID, published.Version, receipt.RecordID, first[contract.Publish].Input.Digest)
		})
	}
	require.Equal(t, []string{"PS-001", "PS-002", "PS-003"}, consumed, "both consumers must exercise the same core case IDs")
}

func assertSharedTimes(t *testing.T, start, end time.Time, got contract.DiagnosticResult) {
	t.Helper()
	require.False(t, got.Input.ReadAt.Before(start))
	require.False(t, got.Input.EvaluatedAt.Before(got.Input.ReadAt))
	require.False(t, got.Input.EvaluatedAt.After(end))
}

func assertSharedOrganizationSwitch(t *testing.T, db *gorm.DB, server *httptest.Server, reader record.Reader, publisher *catalog.Publisher, c sharedCase, input record.Input) {
	t.Helper()
	_, err := publisher.Publish(context.Background(), catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: "300", ProductKey: c.CaseID}, PublicationID: uuid.NewString(), Snapshot: *c.Snapshot})
	require.NoError(t, err)
	const otherStoreID = "44444444-4444-4444-8444-444444444444"
	prepareRecordStoreID(t, db, "300", otherStoreID)
	prepareApprovedAssets(t, db, "300", c.CaseID, input.SnapshotVersion)
	ids := map[string]string{}
	for _, org := range []string{"200", "300"} {
		scopedInput := input
		if org == "300" {
			scopedInput.StoreID = otherStoreID
		}
		status, raw := sharedPost(t, server, "switcher", org, "same-operation-in-two-organizations", scopedInput)
		require.Equal(t, 201, status, string(raw))
		var receipt record.Receipt
		sharedDecode(t, raw, &receipt)
		ids[org] = receipt.RecordID
		stored, err := reader.ReadOfflinePackage(context.Background(), listingtask.Actor{TenantID: org, UserID: "switcher", Roles: []string{"listingkit_operator"}}, receipt.RecordID)
		require.NoError(t, err)
		require.Equal(t, org, stored.OrganizationID)
		require.Equal(t, "switcher", stored.OwnerUserID)
		require.Equal(t, scopedInput, stored.Input)
	}
	require.NotEqual(t, ids["200"], ids["300"])
	before := diagnosticBusinessState(t, db)
	for _, org := range []string{"200", "300", "200"} {
		status, raw := diagnosticGet(t, server, ids[org], "action=publish", "switcher", org)
		require.Equal(t, 200, status, string(raw))
		other := "200"
		if org == "200" {
			other = "300"
		}
		status, raw = diagnosticGet(t, server, ids[other], "action=publish", "switcher", org)
		require.Equal(t, 404, status, string(raw))
		require.NotContains(t, string(raw), "actual_digest")
	}
	require.Equal(t, before, diagnosticBusinessState(t, db))
}
