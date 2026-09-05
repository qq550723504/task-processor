package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func TestImportHTTPCompleteResponseProjection(t *testing.T) {
	const identity = `"source_identity":{"source_type":"crawler","source_platform":"1688","source_id":"123","source_url":"url","source_version":"v1","source_fingerprint":"fp"}`
	const publication = `"publication":{"identity":{"product_key":"product-1"},"publication_id":"pub-1","version":7}`
	for _, tc := range []struct {
		name     string
		warnings []sourcing.SourceWarning
		suffix   string
	}{
		{"nil warnings", nil, ""},
		{"empty warnings", []sourcing.SourceWarning{}, ""},
		{"warnings", []sourcing.SourceWarning{{Code: "missing_title", Message: "missing title", Field: "title"}, {}}, `,"source_warnings":[{"code":"missing_title","message":"missing title","field":"title"},{"code":"","message":"","field":""}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := ImportResult{
				Publication:    catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "internal-org-must-not-leak", ProductKey: "product-1"}, PublicationID: "pub-1", Version: 7, Snapshot: catalog.ProductSnapshot{Title: "internal-snapshot-must-not-leak"}},
				SourceIdentity: sourcing.SourceIdentity{SourceType: "crawler", SourcePlatform: "1688", SourceID: "123", SourceURL: "url", SourceVersion: "v1", SourceFingerprint: "fp", Platform: "legacy", Region: "legacy", ProductID: "legacy", StoreID: 99},
				SourceWarnings: tc.warnings,
			}
			for _, invalid := range []bool{false, true} {
				t.Run(map[bool]string{false: "success", true: "invalid_source"}[invalid], func(t *testing.T) {
					h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
						if invalid {
							return result, ErrInvalidImport
						}
						return result, nil
					}))
					out := httptest.NewRecorder()
					h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"source_account_id":0,"store_id":"s"}`)).WithContext(requestContext()))
					require.Equal(t, "application/json", out.Header().Get("Content-Type"))
					if invalid {
						require.Equal(t, 400, out.Code)
						require.JSONEq(t, `{"error":"invalid_source"`+tc.suffix+`}`, out.Body.String())
					} else {
						require.Equal(t, 200, out.Code)
						require.JSONEq(t, `{`+publication+`,`+identity+tc.suffix+`}`, out.Body.String())
					}
				})
			}
		})
	}
}

func TestImportHTTPEmptyResponseAndOtherErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		body   string
	}{
		{nil, 200, `{"publication":{"identity":{"product_key":""},"version":0,"publication_id":""},"source_identity":{"source_type":"","source_platform":"","source_id":"","source_url":"","source_version":"","source_fingerprint":""}}`},
		{ErrAccessDenied, 403, `{"error":"source_access_denied"}`},
		{catalog.ErrPublicationConflict, 409, `{"error":"publication_conflict"}`},
		{context.Canceled, 504, `{"error":"import_deadline_exceeded"}`},
		{context.DeadlineExceeded, 504, `{"error":"import_deadline_exceeded"}`},
		{errors.New("internal details"), 500, `{"error":"import_failed"}`},
	} {
		h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
			result := ImportResult{}
			if tc.err != nil {
				result.SourceWarnings = []sourcing.SourceWarning{{Code: "private"}}
			}
			return result, tc.err
		}))
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"source_account_id":0,"store_id":"s"}`)).WithContext(requestContext()))
		require.Equal(t, tc.status, out.Code)
		require.JSONEq(t, tc.body, out.Body.String())
	}
}

func TestImportHTTPTransportErrorResponseProjection(t *testing.T) {
	canceled, cancel := context.WithCancel(requestContext())
	cancel()
	for _, tc := range []struct {
		name, method, body string
		ctx                context.Context
		unavailable        bool
		status             int
		response           string
	}{
		{"method", "GET", "", requestContext(), false, 405, `{"error":"method_not_allowed"}`},
		{"unauthenticated", "POST", "{}", context.Background(), false, 401, `{"error":"verified_organization_required"}`},
		{"unavailable", "POST", "{}", requestContext(), true, 503, `{"error":"service_unavailable"}`},
		{"canceled", "POST", "{}", canceled, false, 504, `{"error":"import_deadline_exceeded"}`},
		{"invalid JSON", "POST", "{", requestContext(), false, 400, `{"error":"invalid_request"}`},
		{"missing fields", "POST", "{}", requestContext(), false, 400, `{"error":"invalid_request"}`},
		{"oversize", "POST", `{"raw_snapshot":"` + strings.Repeat("x", MaxImportBytes) + `"}`, requestContext(), false, 413, `{"error":"import_too_large"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var importer Importer
			if !tc.unavailable {
				importer = importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
					t.Fatal("invalid transport reached importer")
					return ImportResult{}, nil
				})
			}
			out := httptest.NewRecorder()
			NewHandler(importer).ServeHTTP(out, httptest.NewRequest(tc.method, "/unregistered", strings.NewReader(tc.body)).WithContext(tc.ctx))
			require.Equal(t, tc.status, out.Code)
			require.JSONEq(t, tc.response, out.Body.String())
		})
	}
}
