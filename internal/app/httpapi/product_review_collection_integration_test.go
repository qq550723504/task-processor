package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"task-processor/internal/product/review"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func titleList(t *testing.T, client *http.Client, origin, path, actor, org string, status int) productReviewPageDTO {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, origin+path, nil)
	require.NoError(t, err)
	if actor != "" {
		req.Header.Set("Authorization", "Bearer "+actor)
	}
	req.Header.Set("X-Requested-Organization-ID", org)
	response, err := client.Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	var raw bytes.Buffer
	_, err = raw.ReadFrom(response.Body)
	require.NoError(t, err)
	require.Equal(t, status, response.StatusCode, raw.String())
	var page productReviewPageDTO
	if status == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw.Bytes(), &page))
		require.Equal(t, productReviewSchemaVersion, page.SchemaVersion)
		require.Equal(t, productReviewCoverage, page.Coverage)
	}
	return page
}

func TestProductReviewPostgresActionableCollectionScopePaginationAndState(t *testing.T) {
	f := newTitleFixture(t)
	server := f.server(t)
	pending := titleCreateFor(t, server, "pending", "operator", "B", "product")
	accepted := titleCreateFor(t, server, "accepted", "other", "B", "product")
	accepted = titleDecision(t, server, accepted, "accept", "admin", "", http.StatusOK)
	rejected := titleCreateFor(t, server, "rejected", "operator", "B", "product")
	rejected = titleDecision(t, server, rejected, "reject", "admin", "", http.StatusOK)
	edited := titleCreateFor(t, server, "edited", "operator", "B", "product")
	edited = titleDecision(t, server, edited, "accept", "admin", "", http.StatusOK)
	edited = titleDecision(t, server, edited, "edit", "operator", "Human edit", http.StatusOK)
	applied := titleCreateFor(t, server, "applied", "operator", "B", "product")
	applied = titleDecision(t, server, applied, "accept", "admin", "", http.StatusOK)
	applied = titleApply(t, server, applied, "apply-collection", http.StatusOK)
	otherOrg := titleCreateFor(t, server, "other-org", "admin", "A", "product-a")

	var proposalQueries atomic.Int32
	callbackName := "issue343_count_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register(callbackName, func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.Table == "product_title_proposals" {
			proposalQueries.Add(1)
		}
	}))
	t.Cleanup(func() { _ = f.db.Callback().Query().Remove(callbackName) })

	first := titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable&limit=2", "admin", "B", http.StatusOK)
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor)
	second := titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable&limit=2&cursor="+*first.NextCursor, "admin", "B", http.StatusOK)
	require.Len(t, second.Items, 1)
	require.Nil(t, second.NextCursor)
	require.Equal(t, int32(2), proposalQueries.Load(), "one bounded proposal query per page; no per-item GET")

	gotIDs := []string{first.Items[0].ProposalID, first.Items[1].ProposalID, second.Items[0].ProposalID}
	wantIDs := []string{pending.ID, accepted.ID, edited.ID}
	sort.Strings(wantIDs)
	require.Equal(t, wantIDs, gotIDs)
	require.Equal(t, len(gotIDs), len(map[string]struct{}{gotIDs[0]: {}, gotIDs[1]: {}, gotIDs[2]: {}}), "fixed pages must not duplicate IDs")
	for _, item := range append(first.Items, second.Items...) {
		require.Equal(t, "1", item.BaseVersion)
		require.Contains(t, []string{"pending", "accepted"}, item.State)
		if item.ProposalID == accepted.ID {
			require.Equal(t, "2", item.ProposalRevision)
		}
		if item.ProposalID == edited.ID {
			require.Equal(t, "3", item.ProposalRevision)
		}
	}
	operatorPage := titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable&limit=100", "operator", "B", http.StatusOK)
	operatorIDs := []string{operatorPage.Items[0].ProposalID, operatorPage.Items[1].ProposalID}
	sort.Strings(operatorIDs)
	wantOperator := []string{pending.ID, edited.ID}
	sort.Strings(wantOperator)
	require.Equal(t, wantOperator, operatorIDs)
	otherPage := titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "other", "B", http.StatusOK)
	require.Equal(t, []productReviewCollectionItemDTO{{ProposalID: accepted.ID, ProductKey: "product", BaseVersion: "1", ProposalRevision: "2", State: "accepted"}}, otherPage.Items)
	orgPage := titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "admin", "A", http.StatusOK)
	require.Equal(t, otherOrg.ID, orgPage.Items[0].ProposalID)
	require.Equal(t, "rejected", titleCall(t, server, http.MethodGet, titleBasePath+"/"+rejected.ID, "admin", "B", "", "", http.StatusOK).State)
	require.Equal(t, "applied", titleCall(t, server, http.MethodGet, titleBasePath+"/"+applied.ID, "admin", "B", "", "", http.StatusOK).State)
}

func TestProductReviewCollectionStrictTransportAndErrors(t *testing.T) {
	f := newTitleFixture(t)
	server := f.server(t)
	proposal := titleCreate(t, server, "transport")
	for _, path := range []string{titleBasePath, titleBasePath + "?view=other", titleBasePath + "?view=actionable&view=actionable", titleBasePath + "?view=actionable&limit=020", titleBasePath + "?view=actionable&unknown=x", titleBasePath + "?view=actionable&unknown=" + strings.Repeat("x", maxProductReviewQueryBytes), titleBasePath + "?view=actionable&" + strings.Repeat("a=b&", 300)} {
		code, _, err := titleRequest(server, http.MethodGet, path, "admin", "B", "", "")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, code, path)
	}
	for _, path := range []string{titleBasePath + "?view=actionable", titleBasePath + "/" + proposal.ID} {
		code, _, err := titleRequest(server, http.MethodGet, path, "admin", "B", "", `{}`)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, code)
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+titleBasePath+"?view=actionable", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer admin")
	request.Header.Set("X-Requested-Organization-ID", "B")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	require.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"))
	request, err = http.NewRequest(http.MethodGet, server.URL+titleBasePath+"?view=actionable", nil)
	require.NoError(t, err)
	response, err = server.Client().Do(request)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	require.Equal(t, "nosniff", response.Header.Get("X-Content-Type-Options"))

	f.grants.failed.Store(true)
	dependencyServer := f.server(t)
	titleList(t, dependencyServer.Client(), dependencyServer.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusServiceUnavailable)
	f.grants.failed.Store(false)
	f.grants.revoked.Store(true)
	revokedServer := f.server(t)
	titleList(t, revokedServer.Client(), revokedServer.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusForbidden)
}

func TestProductReviewCollectionCorruptionAndDependencyFailureAreNotEmpty(t *testing.T) {
	t.Run("database prevents reverse state drift", func(t *testing.T) {
		f := newTitleFixture(t)
		server := f.server(t)
		proposal := titleCreate(t, server, "drift")
		err := f.db.Exec("UPDATE product_title_proposals SET state = 'rejected' WHERE org = ? AND id = ?", "B", proposal.ID).Error
		require.Error(t, err)
	})
	t.Run("malformed actionable row fails closed", func(t *testing.T) {
		f := newTitleFixture(t)
		server := f.server(t)
		proposal := titleCreate(t, server, "malformed")
		require.NoError(t, f.db.Exec("ALTER TABLE product_title_proposals DROP CONSTRAINT ck_product_title_proposals_state_payload").Error)
		require.NoError(t, f.db.Exec("UPDATE product_title_proposals SET payload = ? WHERE org = ? AND id = ?", []byte(`[]`), "B", proposal.ID).Error)
		titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusServiceUnavailable)
	})
	t.Run("oversized actionable row is selected then rejected", func(t *testing.T) {
		f := newTitleFixture(t)
		server := f.server(t)
		proposal := titleCreate(t, server, "oversized")
		require.NoError(t, f.db.Exec("ALTER TABLE product_title_proposals DROP CONSTRAINT ck_product_title_proposals_state_payload").Error)
		require.NoError(t, f.db.Exec("ALTER TABLE product_title_proposals DROP CONSTRAINT ck_product_title_proposals_payload_size").Error)
		require.NoError(t, f.db.Exec("UPDATE product_title_proposals SET payload = ? WHERE org = ? AND id = ?", bytes.Repeat([]byte{'x'}, review.MaxRecordBytes+1), "B", proposal.ID).Error)
		titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusServiceUnavailable)
	})
	t.Run("dependency failure is not an empty page", func(t *testing.T) {
		f := newTitleFixture(t)
		server := f.server(t)
		require.NoError(t, f.db.Exec("DROP TABLE product_title_proposals").Error)
		titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusServiceUnavailable)
	})
}

func TestProductReviewCollectionHonorsCancellationWhileWaitingForDatabase(t *testing.T) {
	f := newTitleFixture(t)
	server := f.server(t)
	titleCreate(t, server, "cancel-list")
	lock := f.db.Begin()
	require.NoError(t, lock.Error)
	defer lock.Rollback()
	require.NoError(t, lock.Exec("LOCK TABLE product_title_proposals IN ACCESS EXCLUSIVE MODE").Error)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+titleBasePath+"?view=actionable", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer admin")
	request.Header.Set("X-Requested-Organization-ID", "B")
	_, err = server.Client().Do(request)
	require.Error(t, err)
	require.NoError(t, lock.Rollback().Error)
	titleList(t, server.Client(), server.URL, titleBasePath+"?view=actionable", "admin", "B", http.StatusOK)
}
