package httpapi

import (
	"net/url"
	"testing"
	"time"

	"task-processor/internal/listing/record"
	contract "task-processor/internal/marketplace/validator"

	"github.com/stretchr/testify/require"
)

func TestSheinRecordCollectionQueryAndCursorContract(t *testing.T) {
	created := time.Date(2026, 9, 6, 1, 2, 3, 456789000, time.UTC)
	cursor, err := encodeSheinRecordCursor(record.PageCursor{ID: "12345678-1234-4234-8234-123456789abc", CreatedAt: created})
	require.NoError(t, err)
	require.NotContains(t, cursor, "=")

	request, err := parseSheinRecordPageQuery(url.Values{"limit": {"20"}, "cursor": {cursor}})
	require.NoError(t, err)
	require.Equal(t, 20, request.Limit)
	require.Equal(t, created, request.Cursor.CreatedAt)
	require.Equal(t, "12345678-1234-4234-8234-123456789abc", request.Cursor.ID)

	defaults, err := parseSheinRecordPageQuery(url.Values{})
	require.NoError(t, err)
	require.Equal(t, 20, defaults.Limit)
	require.Nil(t, defaults.Cursor)

	for _, query := range []url.Values{
		{"limit": {"0"}}, {"limit": {"101"}}, {"limit": {"020"}}, {"limit": {"+20"}},
		{"limit": {"20", "21"}}, {"cursor": {""}}, {"cursor": {cursor, cursor}}, {"unknown": {"x"}},
	} {
		_, err := parseSheinRecordPageQuery(query)
		require.ErrorIs(t, err, record.ErrInvalid, "%v", query)
	}

	for _, changed := range []string{
		cursor + "A",
		"eyJ2IjoxLCJjcmVhdGVkX2F0IjoiMjAyNi0wOS0wNlQwMTowMjowMy40NTY3ODlaIiwicmVjb3JkX2lkIjoibm90LWEtdXVpZCJ9",
	} {
		_, err := decodeSheinRecordCursor(changed)
		require.ErrorIs(t, err, record.ErrInvalid)
	}
}

func TestSheinRecordCollectionDTOUsesStringVersionAndExactFields(t *testing.T) {
	created := time.Date(2026, 9, 6, 1, 2, 3, 456789000, time.UTC)
	wire, err := marshalSheinRecordPage(record.Page{Items: []record.CollectionItem{{
		ID:        "12345678-1234-4234-8234-123456789abc",
		Input:     record.Input{ProductKey: "source-1", SnapshotVersion: 9007199254740993, StoreID: recordStoreID, Country: "US", Language: "en", Action: contract.SaveDraft},
		CreatedAt: created,
	}}})
	require.NoError(t, err)
	require.JSONEq(t, `{"items":[{"record_id":"12345678-1234-4234-8234-123456789abc","product_key":"source-1","snapshot_version":"9007199254740993","store_id":"11111111-1111-4111-8111-111111111111","country":"US","language":"en","action":"save_draft","created_at":"2026-09-06T01:02:03.456789Z"}],"next_cursor":null}`, string(wire))
}
