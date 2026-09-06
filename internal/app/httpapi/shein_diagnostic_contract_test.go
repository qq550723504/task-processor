package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	"task-processor/internal/workbenchcontext"

	"github.com/stretchr/testify/require"
)

type diagnosticReaderFunc func(context.Context, listingtask.Actor, string) (record.Record, error)

func (f diagnosticReaderFunc) ReadOfflinePackage(ctx context.Context, a listingtask.Actor, id string) (record.Record, error) {
	return f(ctx, a, id)
}

type diagnosticEvaluatorFunc func(contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error)

func (f diagnosticEvaluatorFunc) Validate(r contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error) {
	return f(r)
}

// Narrow failure seams use the same real routes/middleware and permission code.
// The PostgreSQL suite, not these seams, proves the actual success chain.
func diagnosticSeamServer(t *testing.T, reader record.Reader, evaluator record.DiagnosticEvaluator, budget time.Duration) *httptest.Server {
	t.Helper()
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	service, err := record.NewDiagnosticService(reader, evaluator, auth, sheinvalidator.DiagnosticRuleVersion, sheinvalidator.BindingVersion)
	require.NoError(t, err)
	server := buildHTTPServerFromRoutesAtWithAuthDependencies("127.0.0.1", 0, sheinDiagnosticRoutes(service), routeAuthDependencies{workbenchVerifier: recordVerifier{}, organizationResolver: workbenchcontext.NewResolver(&diagnosticGrants{}, "project", "v1", nil), authorizer: auth})
	handler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	ts := httptest.NewServer(server.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestSheinDiagnosticTypedFailureHTTP(t *testing.T) {
	for _, cause := range []string{"stale", "expired", "expired_at_evaluation", "subject_mismatch"} {
		t.Run(cause, func(t *testing.T) {
			reader := diagnosticReaderFunc(func(context.Context, listingtask.Actor, string) (record.Record, error) {
				return record.Record{Payload: []byte(`{}`), ReadAt: time.Now().UTC()}, nil
			})
			evaluator := diagnosticEvaluatorFunc(func(contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error) {
				return contract.DiagnosticResult{}, &contract.Error{Code: contract.StaleInput, Message: "must not leak secret", Freshness: &contract.FreshnessFailure{Status: contract.FreshnessStale, Coverage: []string{"template_only"}, Causes: []string{cause}}}
			})
			ts := diagnosticSeamServer(t, reader, evaluator, time.Second)
			status, body := diagnosticGet(t, ts, "00000000-0000-0000-0000-000000000001", "action=publish", "operator", "200")
			require.Equal(t, 409, status)
			require.JSONEq(t, `{"error":"stale_input","freshness":{"status":"stale","coverage":["template_only"],"causes":["`+cause+`"]}}`, string(body))
		})
	}
}

func TestSheinDiagnosticBoundaryFailuresHTTP(t *testing.T) {
	for _, scenario := range []string{"reader unavailable", "cancelled read", "clock rollback", "report oversized", "report encoding", "compute deadline"} {
		t.Run(scenario, func(t *testing.T) {
			reader := diagnosticReaderFunc(func(ctx context.Context, _ listingtask.Actor, _ string) (record.Record, error) {
				switch scenario {
				case "reader unavailable":
					return record.Record{}, errors.New("secret SQL")
				case "cancelled read":
					return record.Record{}, context.Canceled
				case "clock rollback":
					return record.Record{Payload: []byte(`{}`), ReadAt: time.Now().Add(time.Hour)}, nil
				}
				return record.Record{Payload: []byte(`{}`), ReadAt: time.Now().UTC()}, nil
			})
			evaluator := diagnosticEvaluatorFunc(func(req contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error) {
				got, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
				if err != nil {
					return got, err
				}
				switch scenario {
				case "report oversized":
					got.Scope = strings.Repeat("x", maxDiagnosticResponseBytes)
				case "report encoding":
					got.Input.ReadAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
				case "compute deadline":
					time.Sleep(150 * time.Millisecond)
				}
				return got, nil
			})
			ts := diagnosticSeamServer(t, reader, evaluator, 100*time.Millisecond)
			status, body := diagnosticGet(t, ts, "00000000-0000-0000-0000-000000000001", "action=publish", "operator", "200")
			want, code := 500, "evaluation_failed"
			if scenario == "reader unavailable" {
				want, code = 503, "unavailable"
			}
			if scenario == "cancelled read" || scenario == "compute deadline" {
				want, code = 504, "deadline_exceeded"
			}
			require.Equal(t, want, status, string(body))
			require.JSONEq(t, `{"error":"`+code+`"}`, string(body))
		})
	}
}

func TestSheinDiagnosticDeniedBeforeReadHTTP(t *testing.T) {
	var reads atomic.Int32
	reader := diagnosticReaderFunc(func(context.Context, listingtask.Actor, string) (record.Record, error) {
		reads.Add(1)
		return record.Record{}, record.ErrNotFound
	})
	ts := diagnosticSeamServer(t, reader, sheinvalidator.DiagnosticValidator{}, time.Second)
	for _, user := range []string{"", "viewer", "store", "no-grant"} {
		status, _ := diagnosticGet(t, ts, "00000000-0000-0000-0000-000000000001", "action=publish", user, "200")
		require.Contains(t, []int{401, 403}, status)
	}
	require.Zero(t, reads.Load())
}
