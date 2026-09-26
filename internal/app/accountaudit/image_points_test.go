package accountaudit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/ledger/orgresource"
	registry "task-processor/internal/sourceaccountregistry"
)

type pointHistoryStub struct{ items []orgresource.ImagePointDebit }

func (s pointHistoryStub) ListImagePointDebits(_ context.Context, org, actor string, limit int, after *orgresource.ImagePointAuditPosition) (orgresource.ImagePointAuditPage, error) {
	p := orgresource.ImagePointAuditPage{}
	for _, v := range s.items {
		if v.OrganizationID != org || actor != "" && v.ActorID != actor || after != nil && (v.CreatedAt.After(after.CreatedAt) || v.CreatedAt.Equal(after.CreatedAt) && v.EventID >= after.EventID) {
			continue
		}
		if len(p.Items) == limit {
			last := p.Items[limit-1]
			p.Next = &orgresource.ImagePointAuditPosition{CreatedAt: last.CreatedAt, EventID: last.EventID}
			break
		}
		p.Items = append(p.Items, v)
	}
	return p, nil
}

func TestImagePointAuditMergesAcrossPagesAndBindsActorAndOrganization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	points := pointHistoryStub{items: []orgresource.ImagePointDebit{
		{OrganizationID: "B", EventID: "point-3", ActorID: "actor", MemberID: "member", RunID: "run-1", IntentID: "intent-1", PriceVersion: "price-1", Points: 9007199254740993, CreatedAt: now},
		{OrganizationID: "B", EventID: "point-1", ActorID: "other", MemberID: "member", RunID: "run-2", IntentID: "intent-2", PriceVersion: "price-1", Points: 12, CreatedAt: now.Add(-2 * time.Second)},
	}}
	usage := &usageHistoryStub{events: []UsageAuditEvent{{OrganizationID: "B", EventID: "token-2", MemberID: "member", InvocationID: "inv-1", Quantity: 7, Time: now.Add(-time.Second)}}}
	query, err := NewWithImagePointAuditSources(&historyStub{}, nil, nil, nil, usage, points)
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "B", EffectiveOrganizationID: "B", TokenExpiresAt: now.Add(time.Hour)})
	cursor := ""
	for i, expected := range []string{"point-3", "token-2", "point-1"} {
		page, err := query.Read(ctx, 1, cursor)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		require.Equal(t, expected, page.Items[0].Relation.Reference)
		if i == 0 {
			require.Equal(t, "actor", page.Items[0].Actor)
			require.Equal(t, "9007199254740993", page.Items[0].Points.Quantity)
			require.Equal(t, "price-1", page.Items[0].Points.PriceVersion)
			require.Nil(t, page.Items[0].Usage)
			require.NotNil(t, page.NextCursor)
			_, err = query.ReadFiltered(ctx, 1, *page.NextCursor, Filter{ActorSubject: "actor"})
			require.ErrorIs(t, err, registry.ErrInvalid)
			foreign := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "A", EffectiveOrganizationID: "A", TokenExpiresAt: now.Add(time.Hour)})
			_, err = query.Read(foreign, 1, *page.NextCursor)
			require.ErrorIs(t, err, registry.ErrInvalid)
		}
		if i < 2 {
			require.NotNil(t, page.NextCursor)
			cursor = *page.NextCursor
		} else {
			require.Nil(t, page.NextCursor)
		}
	}
	filtered, err := query.ReadFiltered(ctx, 20, "", Filter{ActorSubject: "actor"})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, "point-3", filtered.Items[0].Relation.Reference)
}
