package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type diagnosticGrants struct {
	revoked atomic.Bool
	reads   atomic.Int32
}

func (g *diagnosticGrants) Load(_ context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	if source != workbenchcontext.GrantLive {
		g.reads.Add(1)
	}
	roles := []string{"listingkit_operator"}
	switch request.Subject {
	case "viewer":
		roles = []string{"listingkit_viewer"}
	case "admin":
		roles = []string{"listingkit_admin"}
	case "readonly":
		roles = []string{"admin"}
	case "store":
		roles = []string{"store_viewer"}
	}
	grants := []authidentity.OrganizationGrant{{OrganizationID: "200", ProjectID: "project", Roles: roles}}
	if request.Subject == "foreign-admin" {
		grants = []authidentity.OrganizationGrant{{OrganizationID: "100", ProjectID: "project", Roles: []string{"listingkit_admin"}}}
	}
	if request.Subject == "no-grant" || g.revoked.Load() {
		grants = nil
	}
	return workbenchcontext.GrantResult{Source: source, Grants: grants}, nil
}
func (*diagnosticGrants) Invalidate(string, string) {}
func diagnosticApplication(t *testing.T, db *gorm.DB, g *diagnosticGrants) *httptest.Server {
	t.Helper()
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	server, _, err := NewSheinRecordApplication(db, recordVerifier{}, workbenchcontext.NewResolver(g, "project", "v1", nil), auth)
	require.NoError(t, err)
	ts := httptest.NewUnstartedServer(server.Handler)
	ts.Config.ReadTimeout = server.ReadTimeout
	ts.Config.WriteTimeout = server.WriteTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}
func diagnosticGet(t *testing.T, ts *httptest.Server, id, query, user, org string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/listing/shein-records/"+id+"/offline-diagnostic?"+query, nil)
	require.NoError(t, err)
	if user != "" {
		req.Header.Set("Authorization", "Bearer "+user)
	}
	if org != "" {
		req.Header.Set("X-Requested-Organization-ID", org)
	}
	response, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, body
}
func TestSheinDiagnosticPostgresHTTPRoundTrip(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	g := &diagnosticGrants{}
	ts := diagnosticApplication(t, db, g)
	status, raw := recordPost(t, ts, "operator", "diagnostic", recordBody)
	require.Equal(t, 201, status, string(raw))
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	status, raw = diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 200, status, string(raw))
	var result contract.DiagnosticResult
	require.NoError(t, json.Unmarshal(raw, &result))
	require.True(t, result.DiagnosticOnly)
	require.NotEmpty(t, result.OfflineChecks.Blockers)
	require.Equal(t, contract.NotEvaluated, result.Freshness.Status)
	require.Equal(t, "no_authoritative_package_freshness", result.NotEvaluatedReasons[contract.ExternalPackageFreshness])
	var durable []byte
	require.NoError(t, db.Raw("SELECT payload FROM listing_shein_records WHERE id=?", receipt.RecordID).Row().Scan(&durable))
	expected, err := (sheinvalidator.DiagnosticValidator{}).Validate(contract.BoundRequest[[]byte]{Input: durable, Target: contract.Target{Marketplace: "shein"}, Action: contract.Publish, RuleVersion: sheinvalidator.DiagnosticRuleVersion, BindingVersion: sheinvalidator.BindingVersion, ReadAt: result.Input.ReadAt, EvaluatedAt: result.Input.EvaluatedAt, Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}})
	require.NoError(t, err)
	require.Equal(t, expected, result)
	t.Logf("synthetic POST record_id=%s; actual GET response=%s", receipt.RecordID, raw)
	restarted := diagnosticApplication(t, db, g)
	status, raw = diagnosticGet(t, restarted, receipt.RecordID, "action=publish&expected_digest="+result.Input.Digest, "operator", "")
	require.Equal(t, 200, status, string(raw))
	var again contract.DiagnosticResult
	require.NoError(t, json.Unmarshal(raw, &again))
	require.Equal(t, result.Input.Digest, again.Input.Digest)
	require.Equal(t, result.OfflineChecks, again.OfflineChecks)
	require.Positive(t, g.reads.Load())
	// PostgreSQL itself rejects business mutations in this application instance.
	readOnly := db.Begin(&sql.TxOptions{ReadOnly: true})
	require.NoError(t, readOnly.Error)
	defer readOnly.Rollback()
	roServer := diagnosticApplication(t, readOnly, g)
	status, _ = diagnosticGet(t, roServer, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 200, status)
}

// Snapshot all tables in the task's isolated schema, including row versions.
// The success chain never seeds Listing; only the real POST writes the record.
func diagnosticBusinessState(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = current_schema() ORDER BY tablename").Scan(&tables).Error)
	state := map[string]string{}
	for _, table := range tables {
		var rows string
		require.NoError(t, db.Raw(fmt.Sprintf(`SELECT coalesce(json_agg(r ORDER BY r::text)::text,'[]') FROM (SELECT t.*,xmin::text AS row_version FROM %q t) r`, table)).Row().Scan(&rows))
		state[table] = rows
	}
	return state
}
func TestSheinDiagnosticPermissionsAndReadOnlyPostgres(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	g := &diagnosticGrants{}
	ts := diagnosticApplication(t, db, g)
	status, raw := recordPost(t, ts, "operator", "permission", recordBody)
	require.Equal(t, 201, status, string(raw))
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	before := diagnosticBusinessState(t, db)
	for _, tc := range []struct {
		user, org string
		status    int
	}{{"operator", "200", 200}, {"operator", "", 200}, {"readonly", "200", 200}, {"admin", "200", 200}, {"", "200", 401}, {"viewer", "200", 403}, {"store", "200", 403}, {"no-grant", "200", 403}, {"operator", "100", 403}, {"foreign-admin", "100", 404}, {"other", "200", 404}, {"admin", "100", 403}} {
		t.Run(tc.user+tc.org, func(t *testing.T) {
			status, body := diagnosticGet(t, ts, receipt.RecordID, "action=publish", tc.user, tc.org)
			require.Equal(t, tc.status, status, string(body))
			if status != 200 {
				require.NotContains(t, string(body), "actual_digest")
				require.NotContains(t, string(body), "Bottle")
				require.NotContains(t, string(body), "owner")
			}
		})
	}
	// Read permission does not imply the POST's write permission.
	status, _ = recordPost(t, ts, "readonly", "cannot-write", recordBody)
	require.Equal(t, 403, status)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := diagnosticGet(t, ts, receipt.RecordID, "action=save_draft", "operator", "200")
			require.Equal(t, 200, status)
		}()
	}
	wg.Wait()
	g.revoked.Store(true)
	status, _ = diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 403, status)
	require.Equal(t, before, diagnosticBusinessState(t, db))
}
func TestSheinDiagnosticQueryAndResponseContractPostgres(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	ts := diagnosticApplication(t, db, &diagnosticGrants{})
	status, raw := recordPost(t, ts, "operator", "query", recordBody)
	require.Equal(t, 201, status, string(raw))
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	for _, query := range []string{"", "action=preview", "action=publish&action=save_draft", "action=publish&site=us", "action=publish&freshness=valid", "action=publish&rule_version=x", "action=publish&tenant_id=200", "action=publish&expected_digest=", "action=publish&expected_digest=x", "action=publish&expected_digest=x&expected_digest=y", "action=publish&%61ction=publish", "action=publish&bad=%zz", "action=publish&x=" + strings.Repeat("x", 1024)} {
		status, body := diagnosticGet(t, ts, receipt.RecordID, query, "operator", "200")
		require.Equal(t, 400, status, string(body))
		require.NotContains(t, string(body), "actual_digest")
	}
	status, raw = diagnosticGet(t, ts, "not-a-uuid", "action=publish", "operator", "200")
	require.Equal(t, 400, status)
	status, raw = diagnosticGet(t, ts, receipt.RecordID, "action=publish&expected_digest=sha256:"+strings.Repeat("a", 64), "operator", "200")
	require.Equal(t, 409, status)
	require.JSONEq(t, `{"error":"stale_input"}`, string(raw))
	status, raw = diagnosticGet(t, ts, receipt.RecordID, "action=save_draft", "operator", "200")
	require.Equal(t, 200, status)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(raw, &wire))
	keys := []string{}
	for key := range wire {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	require.Equal(t, []string{"action", "action_policy", "diagnostic_only", "external_freshness", "input", "not_evaluated", "not_evaluated_reasons", "offline_checks", "rule_version", "scope", "target"}, keys)
	require.Equal(t, map[string]any{"status": "not_evaluated", "coverage": []any{}}, wire["external_freshness"])
	checks := wire["offline_checks"].(map[string]any)
	require.NotEmpty(t, checks["blockers"])
	require.Equal(t, map[string]any{"readiness_blockers_allowed": true}, wire["action_policy"])
	for _, scope := range []string{"external_package_freshness", "online_template_freshness", "store_authorization", "cookie", "pod", "human_review", "approved_asset_provenance_and_consent", "submission_gate"} {
		require.Contains(t, wire["not_evaluated"], scope)
	}
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/listing/shein-records/"+receipt.RecordID+"/offline-diagnostic?action=publish", strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer operator")
	response, err := ts.Client().Do(req)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 400, response.StatusCode)
}
