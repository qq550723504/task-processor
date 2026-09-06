package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProductReviewWireDTOIsExactAndVersionSafe(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 2, 3, 456789000, time.UTC)
	wire, err := marshalProductReviewView(review.View{
		ID: "12345678-1234-4234-8234-123456789abc", Owner: "operator",
		Input:  review.CreateInput{ProductKey: "product", BaseVersion: 9007199254740993},
		Before: "before", Title: "after", OriginalTitle: "suggested", Policy: "title-review-v1", State: "applied", Revision: 9007199254740994,
		Evidence: []review.Evidence{{ID: "evidence", ReferenceType: "captured", ReferenceID: "ref", SnapshotID: "snapshot", Checksum: "checksum"}},
		Quality:  enrichment.QualityScore{Overall: 0.75, EvidenceCoverage: 0.5, RequiredFieldCoverage: 1}, Unresolved: []string{"needs review"},
		History: []review.Decision{{Action: "accept", Actor: "admin", Revision: 9007199254740994, Before: "before", After: "after", At: at}},
		Receipt: &review.Receipt{ProposalID: "12345678-1234-4234-8234-123456789abc", Revision: 9007199254740994, ProductVersion: 9007199254740995, PublicationID: "publication", Actor: "admin", At: at},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"schema_version":1,"coverage":"product-title-proposals-only","proposal_id":"12345678-1234-4234-8234-123456789abc","owner":"operator",
		"input":{"product_key":"product","base_version":"9007199254740993"},"before":"before","after":"after","original_title":"suggested","policy":"title-review-v1","state":"applied","revision":"9007199254740994",
		"evidence":[{"id":"evidence","reference_type":"captured","reference_id":"ref","snapshot_id":"snapshot","checksum":"checksum"}],
		"quality":{"overall":0.75,"evidence_coverage":0.5,"required_field_coverage":1},"unresolved":["needs review"],
		"decisions":[{"action":"accept","actor":"admin","revision":"9007199254740994","before":"before","after":"after","at":"2026-09-07T01:02:03.456789Z"}],
		"apply_receipt":{"proposal_id":"12345678-1234-4234-8234-123456789abc","revision":"9007199254740994","product_version":"9007199254740995","publication_id":"publication","actor":"admin","at":"2026-09-07T01:02:03.456789Z"}
	}`, string(wire))
}

func TestProductReviewWireDTOUsesArraysAndOmitsMissingReceipt(t *testing.T) {
	wire, err := marshalProductReviewView(review.View{ID: "12345678-1234-4234-8234-123456789abc", Owner: "operator", Input: review.CreateInput{ProductKey: "product", BaseVersion: 1}, Policy: "title-review-v1", State: "pending", Revision: 1})
	require.NoError(t, err)
	require.JSONEq(t, `{"schema_version":1,"coverage":"product-title-proposals-only","proposal_id":"12345678-1234-4234-8234-123456789abc","owner":"operator","input":{"product_key":"product","base_version":"1"},"before":"","after":"","original_title":"","policy":"title-review-v1","state":"pending","revision":"1","evidence":[],"quality":{"overall":0,"evidence_coverage":0,"required_field_coverage":0},"unresolved":[],"decisions":[]}`, string(wire))
	require.NotContains(t, string(wire), "apply_receipt")
}

func TestProductReviewWireDTORejectsInvalidPersistedState(t *testing.T) {
	_, err := marshalProductReviewView(review.View{ID: "12345678-1234-4234-8234-123456789abc", Owner: "operator", Input: review.CreateInput{ProductKey: "product", BaseVersion: 1}, Policy: "title-review-v1", State: "accepted", Revision: 2, History: []review.Decision{{Action: "accept", Actor: "admin", Revision: 2}}})
	require.ErrorIs(t, err, review.ErrUnavailable)
	_, err = marshalProductReviewView(review.View{ID: "12345678-1234-4234-8234-123456789abc", Owner: "operator", Input: review.CreateInput{ProductKey: "product"}, Policy: "title-review-v1", State: "pending", Revision: 1})
	require.ErrorIs(t, err, review.ErrUnavailable)
	_, err = marshalProductReviewPage(review.Page{Items: []review.CollectionItem{{ID: "12345678-1234-4234-8234-123456789abc", ProductKey: "product", BaseVersion: 1, State: "rejected"}}})
	require.ErrorIs(t, err, review.ErrUnavailable)
}

func TestProductReviewCollectionQueryCursorAndDTOContract(t *testing.T) {
	cursor, err := encodeProductReviewCursor(review.PageCursor{ID: "12345678-1234-4234-8234-123456789abc"})
	require.NoError(t, err)
	require.NotContains(t, cursor, "=")
	request, err := parseProductReviewCollectionQuery(url.Values{"view": {"actionable"}, "limit": {"20"}, "cursor": {cursor}})
	require.NoError(t, err)
	require.Equal(t, 20, request.Limit)
	require.Equal(t, "12345678-1234-4234-8234-123456789abc", request.Cursor.ID)
	defaults, err := parseProductReviewCollectionQuery(url.Values{"view": {"actionable"}})
	require.NoError(t, err)
	require.Equal(t, 20, defaults.Limit)
	require.Nil(t, defaults.Cursor)
	for _, query := range []url.Values{{}, {"view": {"other"}}, {"view": {"actionable", "actionable"}}, {"view": {"actionable"}, "limit": {"0"}}, {"view": {"actionable"}, "limit": {"101"}}, {"view": {"actionable"}, "limit": {"020"}}, {"view": {"actionable"}, "limit": {"+20"}}, {"view": {"actionable"}, "limit": {"20", "21"}}, {"view": {"actionable"}, "cursor": {""}}, {"view": {"actionable"}, "cursor": {cursor, cursor}}, {"view": {"actionable"}, "unknown": {"x"}}} {
		_, err := parseProductReviewCollectionQuery(query)
		require.ErrorIs(t, err, review.ErrInvalid, "%v", query)
	}
	for _, changed := range []string{cursor + "A", "eyJ2IjoxLCJwcm9wb3NhbF9pZCI6Im5vdC1hLXV1aWQifQ"} {
		_, err := decodeProductReviewCursor(changed)
		require.ErrorIs(t, err, review.ErrInvalid)
	}
	pageWire, err := marshalProductReviewPage(review.Page{Items: []review.CollectionItem{{ID: "12345678-1234-4234-8234-123456789abc", ProductKey: "product", BaseVersion: 9007199254740993, Revision: 9007199254740994, State: "accepted"}}})
	require.NoError(t, err)
	require.JSONEq(t, `{"schema_version":1,"coverage":"product-title-proposals-only","items":[{"proposal_id":"12345678-1234-4234-8234-123456789abc","product_key":"product","base_version":"9007199254740993","proposal_revision":"9007199254740994","state":"accepted"}],"next_cursor":null}`, string(pageWire))
}

func TestProductReviewBusinessErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{review.ErrInvalid, 400, "invalid_request"}, {review.ErrForbidden, 403, "permission_denied"}, {review.ErrNotFound, 404, "not_found"}, {catalog.ErrSnapshotNotReady, 404, "not_found"}, {catalog.ErrStaleSnapshot, 409, "stale_product_version"}, {review.ErrConflict, 409, "operation_conflict"}, {catalog.ErrPublicationConflict, 409, "operation_conflict"}, {review.ErrTooLarge, 413, "input_too_large"}, {context.DeadlineExceeded, 504, "deadline_exceeded"}, {errors.New("private dependency detail"), 503, "unavailable"},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		writeProductReviewError(c, test.err)
		require.Equal(t, test.status, recorder.Code)
		require.JSONEq(t, `{"error":"`+test.code+`"}`, recorder.Body.String())
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		require.NotContains(t, recorder.Body.String(), "private dependency detail")
	}
}

func TestProductReviewSuccessResponseLimitFailsBeforeWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	reviewResponse(c, review.View{ID: "12345678-1234-4234-8234-123456789abc", Owner: "operator", Input: review.CreateInput{ProductKey: "product", BaseVersion: 1}, Policy: "title-review-v1", State: "pending", Revision: 1, Unresolved: []string{strings.Repeat("<", maxProductReviewResponseBytes)}}, nil)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.JSONEq(t, `{"error":"unavailable"}`, recorder.Body.String())
}
