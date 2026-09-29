package orgresourceadapter

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/ledger/orgresource"
)

func TestResourceEventReadIsScopedStableAndBoundToFilters(t *testing.T) {
	db := openSQLiteStore(t)
	require.NoError(t, AutoMigrate(db))
	repo, err := NewGormRepository(db, TransactionConfig{})
	require.NoError(t, err)
	at := time.Now().UTC().Truncate(time.Microsecond)
	for _, org := range []string{"org-a", "org-b"} {
		for i := 0; i < 3; i++ {
			op := uuid.NewString()
			require.NoError(t, db.Create(&organizationResourceOperationRow{OrganizationID: org, OperationID: op, OperationType: "synthetic", RequestFingerprint: "synthetic", State: "succeeded", ImmutableResult: "{}"}).Error)
			require.NoError(t, db.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: org, OperationID: op, ResourceType: string(orgresource.ResourceDataRow), Quantity: 1, AvailableAfter: 10, AllocatedAfter: 8, Reason: "synthetic", SourceType: "synthetic", SourceIdentity: op, CreatedAt: at}).Error)
		}
	}
	query := orgresource.EventQuery{OrganizationID: "org-a", ResourceType: orgresource.ResourceDataRow, Limit: 2}
	first, err := repo.ListEvents(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor)
	query.Cursor = *first.NextCursor
	second, err := repo.ListEvents(context.Background(), query)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.Nil(t, second.NextCursor)
	for _, item := range first.Items {
		require.NotEqual(t, item.EventID, second.Items[0].EventID)
		require.Equal(t, "8", item.AllocatedAfter)
	}
	query.OrganizationID = "org-b"
	_, err = repo.ListEvents(context.Background(), query)
	require.ErrorIs(t, err, orgresource.ErrInvalidInput)
	query.OrganizationID = "org-a"
	query.ResourceType = orgresource.ResourceAIPoint
	_, err = repo.ListEvents(context.Background(), query)
	require.ErrorIs(t, err, orgresource.ErrInvalidInput)
	query.Cursor = ""
	query.Limit = 51
	_, err = repo.ListEvents(context.Background(), query)
	require.ErrorIs(t, err, orgresource.ErrInvalidInput)
}
