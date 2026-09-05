package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

func TestImportHTTPWireSnapshot(t *testing.T) {
	body, err := os.ReadFile("testdata/snapshot.json")
	require.NoError(t, err)
	calls := 0
	h := NewHandler(importFunc(func(_ context.Context, cmd ImportCommand) (ImportResult, error) {
		calls++
		p := cmd.Product
		require.Equal(t, "123", p.ID)
		require.Equal(t, "https://img.test/main.jpg", p.MainImage)
		require.Equal(t, 12.5, p.MinPrice)
		require.Equal(t, 19.5, p.MaxPrice)
		require.Equal(t, 2, p.MinOrderQuantity)
		require.Equal(t, "description", p.ProductDetails[0].Content)
		require.Equal(t, []string{"https://img.test/detail.jpg"}, p.ProductDetails[0].Images)
		require.Equal(t, "https://video.test/1", p.Videos[0].VideoURL)
		require.Equal(t, "Supplier Ltd", p.Supplier.CompanyName)
		require.True(t, p.Supplier.IsGoldSupplier)
		require.Equal(t, "steel", p.Specifications[0].Value)
		require.Equal(t, "box", p.PackInfo.PackageType)
		require.Equal(t, []string{"red"}, p.VariationValues[0].Values)
		require.Equal(t, "red", p.Variants[0].Attributes["color"])
		require.Equal(t, 3, p.Variants[0].Stock)
		require.Equal(t, "CN", p.Shipping.ShippingFrom)
		require.True(t, p.IsCustomized)
		return ImportResult{}, nil
	}))
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(string(body))).WithContext(requestContext()))
	require.Equal(t, 200, out.Code, out.Body.String())
	require.Equal(t, 1, calls)
}

func TestImportHTTPExplicitAccess(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		status       int
		account      int64
	}{
		{"omitted", "", 400, 0}, {"null", `,"source_account_id":null`, 400, 0},
		{"public", `,"source_account_id":0`, 200, 0}, {"account", `,"source_account_id":42`, 200, 42},
		{"negative", `,"source_account_id":-1`, 400, 0}, {"string", `,"source_account_id":"0"`, 400, 0},
		{"fraction", `,"source_account_id":1.5`, 400, 0}, {"overflow", `,"source_account_id":9223372036854775808`, 400, 0},
		{"conflicting mode", `,"source_account_id":42,"access_mode":"public"`, 400, 0},
		{"alternate public", `,"public":true`, 400, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := NewHandler(importFunc(func(_ context.Context, cmd ImportCommand) (ImportResult, error) {
				calls++
				require.Equal(t, tc.account, cmd.SourceAccountID)
				return ImportResult{}, nil
			}))
			out := httptest.NewRecorder()
			h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"store_id":"s"`+tc.fields+`}`)).WithContext(requestContext()))
			require.Equal(t, tc.status, out.Code, out.Body.String())
			if tc.status == 400 {
				require.Zero(t, calls)
				require.JSONEq(t, `{"error":"invalid_request"}`, out.Body.String())
			} else {
				require.Equal(t, 1, calls)
			}
		})
	}
}

func TestImportHTTPSourceNeutralResponse(t *testing.T) {
	for _, id := range []sourcing.SourceIdentity{
		{SourceType: "crawler", SourcePlatform: "1688", SourceID: "123", SourceURL: "https://detail.1688.com/offer/123.html", SourceVersion: "v1", SourceFingerprint: "fingerprint", Platform: "legacy", Region: "CN", ProductID: "old", StoreID: 99},
		{Platform: "legacy", Region: "CN", ProductID: "old", StoreID: 99},
	} {
		h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) {
			return ImportResult{SourceIdentity: id}, nil
		}))
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"store_id":"s","source_account_id":0}`)).WithContext(requestContext()))
		require.Equal(t, 200, out.Code)
		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(out.Body.Bytes(), &result))
		require.Len(t, result, 2)
		require.Contains(t, result, "publication")
		expected, err := json.Marshal(map[string]string{"source_type": id.SourceType, "source_platform": id.SourcePlatform, "source_id": id.SourceID, "source_url": id.SourceURL, "source_version": id.SourceVersion, "source_fingerprint": id.SourceFingerprint})
		require.NoError(t, err)
		require.JSONEq(t, string(expected), string(result["source_identity"]))
	}
}

func TestImportHTTPAccountDenialNeverFallsBackToPublic(t *testing.T) {
	calls := 0
	h := NewHandler(importFunc(func(_ context.Context, cmd ImportCommand) (ImportResult, error) {
		calls++
		require.Equal(t, int64(42), cmd.SourceAccountID)
		return ImportResult{}, ErrAccessDenied
	}))
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":{},"store_id":"s","source_account_id":42}`)).WithContext(requestContext()))
	require.Equal(t, 1, calls)
	require.Equal(t, 403, out.Code)
	require.JSONEq(t, `{"error":"source_access_denied"}`, out.Body.String())
}

func TestImportHTTPRejectsUnknownSnapshotFields(t *testing.T) {
	for _, product := range []string{`{"MainImage":"x"}`, `{"supplier":{"company_secret":"x"}}`, `{"variants":[{"unknown":true}]}`} {
		calls := 0
		h := NewHandler(importFunc(func(context.Context, ImportCommand) (ImportResult, error) { calls++; return ImportResult{}, nil }))
		out := httptest.NewRecorder()
		h.ServeHTTP(out, httptest.NewRequest("POST", "/unregistered", strings.NewReader(`{"url":"x","product":`+product+`,"store_id":"s","source_account_id":0}`)).WithContext(requestContext()))
		require.Zero(t, calls)
		require.Equal(t, 400, out.Code)
		require.JSONEq(t, `{"error":"invalid_request"}`, out.Body.String())
	}
}
