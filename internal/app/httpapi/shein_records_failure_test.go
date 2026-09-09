package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var errRecordCommitConnection = errors.New("injected connection lost at commit")

// Fault injection is confined to the test database driver boundary. The real
// application, Catalog adapter and Listing transaction all execute unchanged.
type recordFaultPool struct {
	gorm.ConnPool
	beginner    *sql.DB
	commitFirst bool
}

func (p recordFaultPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &recordFaultTx{Tx: tx, commitFirst: p.commitFirst}, nil
}

type recordFaultTx struct {
	*sql.Tx
	commitFirst bool
}

func (t recordFaultTx) Commit() error {
	if t.commitFirst {
		if err := t.Tx.Commit(); err != nil {
			return err
		}
	} else {
		if err := t.Tx.Rollback(); err != nil {
			return err
		}
	}
	return errRecordCommitConnection
}
func TestSheinRecordCommitFailureAndUnknownOutcomeHTTP(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic rollback", true: "commit succeeded response lost"}[committed], func(t *testing.T) {
			db := recordTestDB(t)
			publishRecordProduct(t, db, "200", "product")
			raw, err := db.DB()
			require.NoError(t, err)
			faulty := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
			faulty.Statement.ConnPool = recordFaultPool{ConnPool: raw, beginner: raw, commitFirst: committed}
			server, _ := recordApplication(t, faulty, &recordGrants{})
			status, body := recordPost(t, server, "operator", "commit-fault", recordBody)
			require.Equal(t, 503, status, string(body))
			require.NotContains(t, string(body), "connection")
			expected := int64(0)
			if committed {
				expected = 1
			}
			requireDraftS1RowCounts(t, db, expected)
			var prior string
			if committed {
				require.NoError(t, db.Raw("SELECT id FROM listing_shein_records").Row().Scan(&prior))
			}
			// New composition/repository after an unknown commit: retry the same key,
			// with fresh authorization, without compensating or rewriting ownership.
			restarted, reader := recordApplication(t, db, &recordGrants{})
			status, body = recordPost(t, restarted, "operator", "commit-fault", recordBody)
			require.Equal(t, 201, status, string(body))
			var receipt record.Receipt
			require.NoError(t, json.Unmarshal(body, &receipt))
			if committed {
				require.Equal(t, prior, receipt.RecordID)
			}
			stored, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
			require.NoError(t, err)
			require.NotEmpty(t, stored.Payload)
			require.Equal(t, "operator", stored.OwnerUserID)
			requireDraftS1RowCounts(t, db, 1)
		})
	}
}

func TestSheinRecordDefaultProductionHasNoRoute(t *testing.T) {
	composition, cfg := buildPersistentProductionCompositionFixture(t)
	bundle, err := composition.buildRuntimeBundle(cfg)
	require.NoError(t, err)
	server, routes := bundle.buildServerBundle(8080, appHTTPTestRouteAuthorization)
	for _, route := range routes {
		require.NotEqual(t, "/api/listing/shein-records", route.Path)
		require.NotEqual(t, sheinDiagnosticPath, route.Path)
	}
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/listing/shein-records", strings.NewReader(recordBody)))
	require.Equal(t, 404, response.Code)
	getResponse := httptest.NewRecorder()
	server.Handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/listing/shein-records?limit=20", nil))
	require.Equal(t, 404, getResponse.Code)
	getResponse = httptest.NewRecorder()
	server.Handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/listing/shein-records/00000000-0000-0000-0000-000000000001/offline-diagnostic?action=publish", nil))
	require.Equal(t, 404, getResponse.Code)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	application, reader, err := NewSheinRecordApplication(nil, recordVerifier{}, workbenchcontext.NewResolver(&recordGrants{}, "project", "v1", nil), auth)
	require.ErrorIs(t, err, record.ErrUnavailable)
	require.Nil(t, application)
	require.Nil(t, reader)
}

func TestSheinRecordCancellationAndIsolatedSource(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	server, reader, err := NewSheinRecordApplication(db, recordVerifier{}, workbenchcontext.NewResolver(&recordGrants{}, "project", "v1", nil), auth)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("POST", "/api/listing/shein-records", strings.NewReader(recordBody)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer operator")
	req.Header.Set("X-Requested-Organization-ID", "200")
	req.Header.Set("Idempotency-Key", "cancelled")
	out := httptest.NewRecorder()
	server.Handler.ServeHTTP(out, req)
	require.Equal(t, 504, out.Code)
	_, err = reader.ReadOfflinePackage(ctx, recordActor("operator", "listingkit_operator"), "none")
	require.ErrorIs(t, err, context.Canceled)
	requireDraftS1RowCounts(t, db, 0)
	// A different explicitly bound storage scope cannot see another scope's
	// Product, even when organization/key/version have exactly the same strings.
	empty := recordTestDB(t)
	other, _ := recordApplication(t, empty, &recordGrants{})
	status, _ := recordPost(t, other, "operator", "old-source", recordBody)
	require.Equal(t, 404, status)
	_, err = reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), "old-task-id")
	require.ErrorIs(t, err, record.ErrNotFound)
}

func TestSheinRecordCancellationWhileDatabaseIsLocked(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, _ := recordApplication(t, db, &recordGrants{})
	lock := db.Begin()
	require.NoError(t, lock.Error)
	require.NoError(t, lock.Exec("LOCK TABLE listing_shein_records IN ACCESS EXCLUSIVE MODE").Error)
	defer lock.Rollback()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/listing/shein-records", strings.NewReader(recordBody))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer operator")
	request.Header.Set("X-Requested-Organization-ID", "200")
	request.Header.Set("Idempotency-Key", "blocked-query")
	response, err := server.Client().Do(request)
	if response != nil {
		_ = response.Body.Close()
	}
	require.ErrorIs(t, err, context.DeadlineExceeded)
	// Wait for the actual HTTP handler to unwind before releasing the database
	// lock. A request that ignores cancellation would keep this server alive.
	server.Close()
	require.NoError(t, lock.Rollback().Error)
	requireDraftS1RowCounts(t, db, 0)
}

func TestSheinRecordApplicationDeadlineReturnsHTTPTimeout(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, _ := recordApplication(t, db, &recordGrants{})
	lock := db.Begin()
	require.NoError(t, lock.Error)
	require.NoError(t, lock.Exec("LOCK TABLE listing_shein_records IN ACCESS EXCLUSIVE MODE").Error)
	defer lock.Rollback()
	// No shorter client deadline: the application budget must expire while
	// the real server still has time to transmit its bounded error response.
	server.Client().Timeout = 2 * record.Timeout
	status, body := recordPost(t, server, "operator", "application-deadline", recordBody)
	require.Equal(t, http.StatusGatewayTimeout, status, string(body))
	require.NoError(t, lock.Rollback().Error)
	requireDraftS1RowCounts(t, db, 0)
}

func TestSheinRecordSlowBodyReturnsHTTPTimeout(t *testing.T) {
	db := recordTestDB(t)
	server, _ := recordApplication(t, db, &recordGrants{})
	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(2*record.Timeout)))
	_, err = fmt.Fprintf(conn, "POST /api/listing/shein-records HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer operator\r\nX-Requested-Organization-ID: 200\r\nIdempotency-Key: slow-body\r\nContent-Length: %d\r\n\r\n{", len(recordBody))
	require.NoError(t, err)
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusGatewayTimeout, response.StatusCode)
	requireDraftS1RowCounts(t, db, 0)
}

type recordDeadlineGrants struct{ recordGrants }

func (*recordDeadlineGrants) Load(ctx context.Context, _ workbenchcontext.GrantSource, _ workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	<-ctx.Done()
	return workbenchcontext.GrantResult{}, ctx.Err()
}

type recordDeadlineVerifier struct{}

func (recordDeadlineVerifier) Verify(ctx context.Context, _ string) (authidentity.AuthenticatedIdentity, error) {
	<-ctx.Done()
	return authidentity.AuthenticatedIdentity{}, ctx.Err()
}

func TestSheinRecordAuthorizationDeadlineReturnsHTTPTimeout(t *testing.T) {
	for _, stage := range []string{"authentication", "organization grants"} {
		t.Run(stage, func(t *testing.T) {
			db := recordTestDB(t)
			auth, err := authz.NewListingKitAuthorizer(nil, nil)
			require.NoError(t, err)
			resolver := workbenchcontext.NewResolver(&recordDeadlineGrants{}, "project", "v1", nil)
			application, _, err := NewSheinRecordApplication(db, recordVerifier{}, resolver, auth)
			if stage == "authentication" {
				application, _, err = NewSheinRecordApplication(db, recordDeadlineVerifier{}, resolver, auth)
			}
			require.NoError(t, err)
			server := httptest.NewUnstartedServer(application.Handler)
			server.Config.ReadTimeout = application.ReadTimeout
			server.Config.WriteTimeout = application.WriteTimeout
			server.Start()
			defer server.Close()
			server.Client().Timeout = 2 * record.Timeout
			status, body := recordPost(t, server, "operator", "auth-deadline", recordBody)
			require.Equal(t, http.StatusGatewayTimeout, status, string(body))
			requireDraftS1RowCounts(t, db, 0)
		})
	}
}
