package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
)

func TestSheinDiagnosticPersistedFailuresPostgres(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	ts := diagnosticApplication(t, db, &diagnosticGrants{})
	status, raw := recordPost(t, ts, "operator", "failures", recordBody)
	require.Equal(t, 201, status)
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	// Fault injection only into an isolated record produced by the real POST.
	for _, tc := range []struct {
		payload string
		status  int
		code    string
	}{{`{"future_field":true}`, 503, "unavailable"}, {`{"spu_name":"a","spu_name":"b"}`, 503, "unavailable"}, {`{"preview_payload":{"spu_name":"` + strings.Repeat("x", 1100000) + `"}}`, 503, "unavailable"}, {`{"metadata":{"variant_image_coverage_status":"blocked","variant_image_coverage_message":"` + strings.Repeat("x", 1100000) + `"}}`, 503, "unavailable"}} {
		require.NoError(t, db.Exec("UPDATE listing_shein_records SET payload=? WHERE id=?", []byte(tc.payload), receipt.RecordID).Error)
		status, body := diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
		require.Equal(t, tc.status, status)
		require.JSONEq(t, fmt.Sprintf(`{"error":%q}`, tc.code), string(body))
	}
	// Simulate corrupt storage beyond the adapter's bounded SELECT contract.
	require.NoError(t, db.Exec("ALTER TABLE listing_shein_records DROP CONSTRAINT listing_shein_records_payload_check").Error)
	require.NoError(t, db.Exec("UPDATE listing_shein_records SET payload=? WHERE id=?", []byte(strings.Repeat("x", record.MaxPayloadBytes+1)), receipt.RecordID).Error)
	status, raw = diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 413, status)
	require.JSONEq(t, `{"error":"input_too_large"}`, string(raw))
}

func TestSheinDiagnosticDatabaseDeadlinePostgres(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	ts := diagnosticApplication(t, db, &diagnosticGrants{})
	status, raw := recordPost(t, ts, "operator", "lock", recordBody)
	require.Equal(t, 201, status)
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	before := diagnosticBusinessState(t, db)
	lock := db.Begin()
	require.NoError(t, lock.Error)
	defer lock.Rollback()
	require.NoError(t, lock.Exec("LOCK TABLE listing_shein_records IN ACCESS EXCLUSIVE MODE").Error)
	ts.Client().Timeout = 2 * record.Timeout
	status, raw = diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 504, status, string(raw))
	require.JSONEq(t, `{"error":"deadline_exceeded"}`, string(raw))
	require.NoError(t, lock.Rollback().Error)
	require.Equal(t, before, diagnosticBusinessState(t, db))
	status, _ = diagnosticGet(t, ts, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 200, status)
}

func TestSheinDiagnosticAuthorizationAndSlowBodyDeadlinePostgres(t *testing.T) {
	for _, stage := range []string{"authentication", "grants", "slow body"} {
		t.Run(stage, func(t *testing.T) {
			db := recordTestDB(t)
			auth, err := authz.NewListingKitAuthorizer(nil, nil)
			require.NoError(t, err)
			resolver := workbenchcontext.NewResolver(&diagnosticGrants{}, "project", "v1", nil)
			if stage == "grants" {
				resolver = workbenchcontext.NewResolver(&recordDeadlineGrants{}, "project", "v1", nil)
			}
			app, _, err := NewSheinRecordApplication(db, recordVerifier{}, resolver, auth)
			if stage == "authentication" {
				app, _, err = NewSheinRecordApplication(db, recordDeadlineVerifier{}, resolver, auth)
			}
			require.NoError(t, err)
			ts := httptest.NewUnstartedServer(app.Handler)
			ts.Config.ReadTimeout = app.ReadTimeout
			ts.Config.WriteTimeout = app.WriteTimeout
			ts.Start()
			defer ts.Close()
			ts.Client().Timeout = 2 * record.Timeout
			if stage == "slow body" {
				conn, err := net.Dial("tcp", ts.Listener.Addr().String())
				require.NoError(t, err)
				defer conn.Close()
				require.NoError(t, conn.SetDeadline(time.Now().Add(2*record.Timeout)))
				_, err = fmt.Fprintf(conn, "GET /api/listing/shein-records/00000000-0000-0000-0000-000000000001/offline-diagnostic?action=publish HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer operator\r\nContent-Length: 1\r\n\r\n")
				require.NoError(t, err)
				response, err := http.ReadResponse(bufio.NewReader(conn), nil)
				require.NoError(t, err)
				defer response.Body.Close()
				require.Equal(t, 504, response.StatusCode)
			} else {
				status, body := diagnosticGet(t, ts, "00000000-0000-0000-0000-000000000001", "action=publish", "operator", "200")
				require.Equal(t, 504, status, string(body))
				require.Contains(t, string(body), "DEADLINE_EXCEEDED")
			}
			var count int64
			require.NoError(t, db.Table("listing_shein_records").Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestSheinDiagnosticCanceledClientPostgres(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	ts := diagnosticApplication(t, db, &diagnosticGrants{})
	status, raw := recordPost(t, ts, "operator", "cancel", recordBody)
	require.Equal(t, 201, status)
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	lock := db.Begin()
	defer lock.Rollback()
	require.NoError(t, lock.Exec("LOCK TABLE listing_shein_records IN ACCESS EXCLUSIVE MODE").Error)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/listing/shein-records/"+receipt.RecordID+"/offline-diagnostic?action=publish", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer operator")
	_, err = ts.Client().Do(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	ts.Close()
	require.NoError(t, lock.Rollback().Error)
	restarted := diagnosticApplication(t, db, &diagnosticGrants{})
	status, _ = diagnosticGet(t, restarted, receipt.RecordID, "action=publish", "operator", "200")
	require.Equal(t, 200, status)
}
