package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"task-processor/internal/listing/record"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createCollectionRecords(t *testing.T, serverURL string, serverClient *http.Client, subject, prefix string, count int) []string {
	t.Helper()
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		req, err := http.NewRequest(http.MethodPost, serverURL+"/api/listing/shein-records", strings.NewReader(recordBody))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+subject)
		req.Header.Set("Idempotency-Key", prefix+"-"+string(rune('a'+i)))
		req.Header.Set("X-Requested-Organization-ID", "200")
		response, err := serverClient.Do(req)
		require.NoError(t, err)
		var receipt record.Receipt
		require.NoError(t, json.NewDecoder(response.Body).Decode(&receipt))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusCreated, response.StatusCode)
		ids = append(ids, receipt.RecordID)
	}
	return ids
}

func decodeCollection(t *testing.T, raw []byte) sheinRecordCollectionDTO {
	t.Helper()
	var page sheinRecordCollectionDTO
	require.NoError(t, json.Unmarshal(raw, &page))
	return page
}

func TestSheinRecordCollectionPostgresPaginationAndScope(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, _ := recordApplication(t, db, &recordGrants{})
	ownerIDs := createCollectionRecords(t, server.URL, server.Client(), "operator", "owner", 22)
	otherIDs := createCollectionRecords(t, server.URL, server.Client(), "other", "other", 3)
	status, raw := recordGet(t, server, "operator", "limit=20")
	require.Equal(t, http.StatusOK, status, string(raw))
	first := decodeCollection(t, raw)
	require.Len(t, first.Items, 20)
	require.NotNil(t, first.NextCursor)
	for _, item := range first.Items {
		require.Contains(t, ownerIDs, item.RecordID)
		require.Equal(t, "1", item.SnapshotVersion)
		require.Equal(t, recordStoreID, item.StoreID)
		require.Equal(t, "save_draft", item.Action)
	}
	status, raw = recordGet(t, server, "operator", "limit=20&cursor="+url.QueryEscape(*first.NextCursor))
	require.Equal(t, http.StatusOK, status, string(raw))
	second := decodeCollection(t, raw)
	require.Len(t, second.Items, 2)
	require.Nil(t, second.NextCursor)
	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		require.False(t, seen[item.RecordID], "page boundary repeated %s", item.RecordID)
		seen[item.RecordID] = true
	}
	require.Len(t, seen, len(ownerIDs))

	status, raw = recordGet(t, server, "other", "limit=20")
	require.Equal(t, http.StatusOK, status, string(raw))
	require.Len(t, decodeCollection(t, raw).Items, len(otherIDs))
	status, raw = recordGet(t, server, "admin", "limit=100")
	require.Equal(t, http.StatusOK, status, string(raw))
	adminPage := decodeCollection(t, raw)
	require.Len(t, adminPage.Items, len(ownerIDs)+len(otherIDs))
	status, raw = recordGet(t, server, "readonly", "limit=100")
	require.Equal(t, http.StatusOK, status, string(raw))
	require.Len(t, decodeCollection(t, raw).Items, len(ownerIDs)+len(otherIDs))

	// An operator cannot reuse an admin cursor whose anchor belongs to another owner.
	status, raw = recordGet(t, server, "admin", "limit=1")
	adminFirst := decodeCollection(t, raw)
	require.NotNil(t, adminFirst.NextCursor)
	anchorID := adminFirst.Items[0].RecordID
	anchorIsOther := false
	for _, id := range otherIDs {
		anchorIsOther = anchorIsOther || id == anchorID
	}
	if !anchorIsOther {
		// Make an other-owned row the newest scoped admin anchor without seeding a row.
		require.NoError(t, db.Exec("UPDATE listing_shein_records SET created_at = ? WHERE id = ?", time.Now().UTC().Add(time.Minute), otherIDs[0]).Error)
		status, raw = recordGet(t, server, "admin", "limit=1")
		adminFirst = decodeCollection(t, raw)
	}
	before := diagnosticBusinessState(t, db)
	status, raw = recordGet(t, server, "operator", "limit=20&cursor="+url.QueryEscape(*adminFirst.NextCursor))
	require.Equal(t, http.StatusBadRequest, status)
	require.JSONEq(t, `{"error":"invalid_request"}`, string(raw))
	require.NotContains(t, string(raw), otherIDs[0])

	status, raw = recordGet(t, server, "store", "limit=20")
	require.Equal(t, http.StatusForbidden, status)
	require.NotContains(t, string(raw), "items")
	require.Equal(t, before, diagnosticBusinessState(t, db), "collection GET changed business rows or xmin")
}

func TestSheinRecordCollectionStableSameTimeAndRefresh(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, _ := recordApplication(t, db, &recordGrants{})
	ids := createCollectionRecords(t, server.URL, server.Client(), "operator", "same", 3)
	shared := time.Date(2026, 9, 6, 2, 3, 4, 500000000, time.UTC)
	require.NoError(t, db.Exec("UPDATE listing_shein_records SET created_at = ? WHERE id IN ?", shared, ids).Error)
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))

	status, raw := recordGet(t, server, "operator", "limit=2")
	require.Equal(t, http.StatusOK, status)
	first := decodeCollection(t, raw)
	require.Equal(t, []string{ids[0], ids[1]}, []string{first.Items[0].RecordID, first.Items[1].RecordID})
	status, raw = recordGet(t, server, "operator", "limit=2&cursor="+url.QueryEscape(*first.NextCursor))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, ids[2], decodeCollection(t, raw).Items[0].RecordID)

	newID := createCollectionRecords(t, server.URL, server.Client(), "operator", "insert", 1)[0]
	status, raw = recordGet(t, server, "operator", "limit=2&cursor="+url.QueryEscape(*first.NextCursor))
	require.Equal(t, http.StatusOK, status)
	for _, item := range decodeCollection(t, raw).Items {
		require.NotEqual(t, newID, item.RecordID, "a later insert must not appear after an older keyset anchor")
	}
	status, raw = recordGet(t, server, "operator", "limit=1")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, newID, decodeCollection(t, raw).Items[0].RecordID, "refreshing the first page discovers the new record")
}

func TestSheinRecordCollectionSQLIsScopedAndMetadataOnly(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, _ := recordApplication(t, db, &recordGrants{})
	createCollectionRecords(t, server.URL, server.Client(), "operator", "sql", 2)

	queries := []string{}
	capture := func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if strings.Contains(query, "listing_shein_records") && strings.HasPrefix(strings.TrimSpace(query), "SELECT") {
			queries = append(queries, query)
		}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("issue327:capture-query", capture))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("issue327:capture-row", capture))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("issue327:capture-query")
		_ = db.Callback().Row().Remove("issue327:capture-row")
	})
	status, raw := recordGet(t, server, "operator", "limit=1")
	require.Equal(t, http.StatusOK, status)
	first := decodeCollection(t, raw)
	queries = nil
	status, raw = recordGet(t, server, "operator", "limit=1&cursor="+url.QueryEscape(*first.NextCursor))
	require.Equal(t, http.StatusOK, status, string(raw))
	require.Len(t, queries, 2, queries)
	joined := strings.ToLower(strings.Join(queries, "\n"))
	require.NotContains(t, joined, " payload")
	require.Contains(t, joined, "organization_id =")
	require.Contains(t, joined, "owner_user_id =")
	require.Contains(t, joined, "order by created_at desc, id desc limit")
}

func TestSheinRecordCollectionUsesBothScopedIndexes(t *testing.T) {
	db := recordTestDB(t)
	for _, fixture := range []struct {
		name, query, index, drop string
		args                     []any
	}{
		{name: "operator", query: "EXPLAIN (COSTS OFF) SELECT id, product_key, snapshot_version, country, language, created_at FROM listing_shein_records WHERE organization_id = ? AND owner_user_id <> '' AND owner_user_id = ? ORDER BY created_at DESC, id DESC LIMIT 21", args: []any{"200", "operator"}, index: "listing_shein_records_owner_collection_idx", drop: "listing_shein_records_admin_collection_idx"},
		{name: "admin", query: "EXPLAIN (COSTS OFF) SELECT id, product_key, snapshot_version, country, language, created_at FROM listing_shein_records WHERE organization_id = ? AND owner_user_id <> '' ORDER BY created_at DESC, id DESC LIMIT 21", args: []any{"200"}, index: "listing_shein_records_admin_collection_idx", drop: "listing_shein_records_owner_collection_idx"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var plan []struct {
				Plan string `gorm:"column:QUERY PLAN"`
			}
			tx := db.WithContext(context.Background()).Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			require.NoError(t, tx.Exec("DROP INDEX "+fixture.drop).Error)
			require.NoError(t, tx.Exec("SET LOCAL enable_seqscan = off").Error)
			require.NoError(t, tx.Raw(fixture.query, fixture.args...).Scan(&plan).Error)
			lines := make([]string, 0, len(plan))
			for _, row := range plan {
				lines = append(lines, row.Plan)
			}
			require.Contains(t, strings.Join(lines, "\n"), fixture.index)
		})
	}
}
