package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"task-processor/internal/listing/record"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSheinRecordScopedBoundedSQLAndConstraints(t *testing.T) {
	db := recordTestDB(t)
	publishRecordProduct(t, db, "200", "product")
	server, reader := recordApplication(t, db, &recordGrants{})
	status, body := recordPost(t, server, "operator", "scope", recordBody)
	require.Equal(t, 201, status)
	var receipt record.Receipt
	require.NoError(t, json.Unmarshal(body, &receipt))
	var query string
	var args []any
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("issue319:capture-scope", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "listing_shein_records") {
			query = tx.Statement.SQL.String()
			args = append([]any(nil), tx.Statement.Vars...)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Row().Remove("issue319:capture-scope") })
	stored, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
	require.NoError(t, err)
	require.Contains(t, query, "CASE WHEN octet_length(payload) <= 2097152")
	require.Contains(t, query, "organization_id =")
	require.Contains(t, query, "owner_user_id =")
	require.Equal(t, []any{receipt.RecordID, "200", "operator"}, args)
	_, err = reader.ReadOfflinePackage(context.Background(), recordActor("admin", "listingkit_admin"), receipt.RecordID)
	require.NoError(t, err)
	require.Equal(t, []any{receipt.RecordID, "200"}, args)
	require.Contains(t, query, "organization_id =")
	require.Contains(t, query, "owner_user_id <> ''")
	require.Error(t, db.Exec("UPDATE listing_shein_records SET owner_user_id = '' WHERE id = ?", receipt.RecordID).Error)
	require.Error(t, db.Exec("UPDATE listing_shein_records SET payload = ? WHERE id = ?", []byte(strings.Repeat("x", record.MaxPayloadBytes+1)), receipt.RecordID).Error)
	unchanged, err := reader.ReadOfflinePackage(context.Background(), recordActor("operator", "listingkit_operator"), receipt.RecordID)
	require.NoError(t, err)
	require.Equal(t, stored.Payload, unchanged.Payload)
	require.Equal(t, stored.OwnerUserID, unchanged.OwnerUserID)
}
