package accountaudit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	registry "task-processor/internal/sourceaccountregistry"
)

type memberAuditHistory struct{ items []AdditionalAuditEvent }

func (h memberAuditHistory) ListRecentAudit(_ context.Context, _ string, limit int, actor, operation string, after *AuditPosition) (AdditionalAuditPage, error) {
	page := AdditionalAuditPage{Items: []AdditionalAuditEvent{}}
	for _, item := range h.items {
		if actor != "" && actor != item.Actor || operation != "" && operation != item.Operation || after != nil && (item.Time.After(after.CreatedAt) || item.Time.Equal(after.CreatedAt) && item.Key >= after.Key) {
			continue
		}
		if len(page.Items) == limit {
			last := page.Items[len(page.Items)-1]
			page.Next = &AuditPosition{CreatedAt: last.Time, Key: last.Key}
			break
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func TestMemberAuditMergesPagesWithoutLossAndBindsFiltersAndTenant(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	base := AdditionalAuditEvent{EventType: "account_member_resource.changed", ObjectType: "member_resource", ObjectReference: "member", Actor: "admin", Operation: "allocate_member_resource", Version: 1, Resource: &ResourceDetail{Type: "data_row", Quantity: "100"}}
	first, second := base, base
	first.Time, first.Key, first.RelationReference = now, "00000000000000000002", "allocate-2"
	second.Time, second.Key, second.RelationReference = now.Add(-2*time.Second), "00000000000000000001", "allocate-1"
	sourceID := "0198d4f0-0000-7000-8000-000000000001"
	history := pagingSourceHistory{events: []registry.CommittedOperation{{OrganizationID: "B", AccountID: sourceID, ActorSubject: "admin", Kind: registry.OperationRegister, Version: 1, OccurredAt: now.Add(-time.Second)}}}
	q, err := NewCurrentAuditSources(history, nil, nil, nil, nil, memberAuditHistory{items: []AdditionalAuditEvent{first, second}})
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "B", TenantID: "B", TokenExpiresAt: now.Add(time.Hour)})
	cursor := ""
	var seen []string
	for i := 0; i < 3; i++ {
		page, err := q.Read(ctx, 1, cursor)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		seen = append(seen, page.Items[0].Relation.Reference)
		if i < 2 {
			require.NotNil(t, page.NextCursor)
			cursor = *page.NextCursor
		} else {
			require.Nil(t, page.NextCursor)
		}
	}
	require.Equal(t, []string{"allocate-2", sourceID, "allocate-1"}, seen)
	other := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "user", EffectiveOrganizationID: "A", TenantID: "A", TokenExpiresAt: now.Add(time.Hour)})
	_, err = q.Read(other, 1, cursor)
	require.ErrorIs(t, err, registry.ErrInvalid)
	_, err = q.ReadFiltered(ctx, 1, cursor, Filter{ResourceOperation: "allocate_member_resource"})
	require.ErrorIs(t, err, registry.ErrInvalid)
	page, err := q.ReadFiltered(ctx, 10, "", Filter{ActorSubject: "admin", ResourceOperation: "allocate_member_resource"})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, "100", page.Items[0].Resource.Quantity)
	q.resources = &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{first}}}
	_, err = q.ReadFiltered(ctx, 10, "", Filter{ActorSubject: "other"})
	require.True(t, errors.Is(err, registry.ErrUnavailable))
	first.Resource = &ResourceDetail{Type: "ai_point", Quantity: "100"}
	q.resources = &additionalHistoryStub{page: AdditionalAuditPage{Items: []AdditionalAuditEvent{first}}}
	_, err = q.Read(ctx, 10, "")
	require.ErrorIs(t, err, registry.ErrUnavailable)
}
