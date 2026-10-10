package dataacquisition

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/product/collection"
)

type resultRepository struct {
	Repository
	job  Job
	item Item
}

func (r *resultRepository) Read(context.Context, collection.Scope, string) (Job, error) {
	return r.job, nil
}
func (r *resultRepository) Items(context.Context, Job) ([]Item, error) { return []Item{r.item}, nil }

type resultDependencies struct{}

func (resultDependencies) EnsureExecution(context.Context, Job) error { return nil }
func (resultDependencies) Verify(context.Context, collection.Scope, collection.Source, Evidence) error {
	return nil
}

func TestResultsMissingFieldsFollowRequestedProjection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fields  []string
		missing []string
	}{
		{"narrow complete projection", []string{"asin", "title"}, nil},
		{"requested missing field", []string{"asin", "brand"}, []string{"brand"}},
		{"several requested missing fields", []string{"rating", "variants"}, []string{"variants", "rating"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.NewString()
			scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original-grant"}
			evidence := &Evidence{Site: "us", ASIN: "B000123456", Title: "captured title", MainImage: "https://m.media-amazon.com/images/I/fixture.jpg", Availability: "available", Price: 10, Currency: "USD", CapturedAt: "2026-10-10T00:00:00Z", ParserVersion: "amazon-v1", Missing: []string{"variants", "rating", "brand"}}
			require.NoError(t, evidence.Validate())
			repo := &resultRepository{job: Job{ID: id, Scope: scope, Query: Query{Fields: tc.fields}}, item: Item{ID: uuid.NewString(), State: "SAVED", Source: &collection.Source{}, Evidence: evidence}}
			svc := &Service{repo: repo, live: &revokedAccess{}, starter: resultDependencies{}}
			page, err := svc.Results(context.Background(), Principal{Scope: scope}, id, "", 100, resultDependencies{})
			require.NoError(t, err)
			require.Len(t, page.Items, 1)
			require.Equal(t, tc.missing, page.Items[0].Missing)
			require.Equal(t, []string{"variants", "rating", "brand"}, evidence.Missing, "projection must not mutate captured facts")
			for field := range page.Items[0].Data {
				require.Contains(t, tc.fields, field)
			}
		})
	}
}
