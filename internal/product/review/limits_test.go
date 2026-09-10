package review

import (
	"context"
	"encoding/json"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPendingRecordReservesDecisionAndReceipt(t *testing.T) {
	r := Record{ID: "00000000-0000-0000-0000-000000000001", Org: "B", Owner: "owner", Revision: 1, State: "pending", Title: strings.Repeat("x", 4096), Original: enrichment.Proposal{Evidence: []enrichment.Evidence{{Metadata: map[string]string{"large": strings.Repeat("m", 54000)}}}}}
	raw, e := json.Marshal(r)
	require.NoError(t, e)
	require.Less(t, len(raw), MaxRecordBytes)
	require.ErrorIs(t, r.ValidateStorage(), ErrTooLarge)
	r.Original.Evidence[0].Metadata["large"] = strings.Repeat("m", 45000)
	require.NoError(t, r.ValidateStorage())
	require.NoError(t, r.Decide(strings.Repeat("a", 128), DecisionInput{Action: "accept", ExpectedRevision: 1}))
	require.NoError(t, r.ValidateStorage())
}
func TestLastPendingRevisionCanStillBeAccepted(t *testing.T) {
	r := Record{Revision: 99, State: "pending", Title: "title"}
	require.ErrorIs(t, r.Decide("owner", DecisionInput{Action: "edit", ExpectedRevision: 99, Title: "edited"}), ErrConflict)
	require.NoError(t, r.Decide("admin", DecisionInput{Action: "accept", ExpectedRevision: 99}))
}

type deadlineStore struct {
	Tx
	saw bool
}

func (d *deadlineStore) Read(context.Context, Scope, string) (Record, error)    { panic("unused") }
func (d *deadlineStore) List(context.Context, Scope, PageRequest) (Page, error) { panic("unused") }
func (d *deadlineStore) Preflight(context.Context, Operation) (View, bool, error) {
	return View{}, false, nil
}
func (d *deadlineStore) Run(_ context.Context, _ Operation, fn func(Tx) (View, error)) (View, error) {
	return fn(d)
}
func (d *deadlineStore) Load(string) (Record, error) {
	return Record{Org: "B", ID: "00000000-0000-0000-0000-000000000001", Input: CreateInput{ProductKey: "p", BaseVersion: 1}, Title: "t", State: "accepted", Revision: 2}, nil
}
func (d *deadlineStore) Replay() (View, bool, error)             { return View{}, false, nil }
func (d *deadlineStore) Reader() catalog.VersionedSnapshotReader { return d }
func (d *deadlineStore) SourceReader() SourcePublicationReader   { return &sourcePublicationReaderStub{} }
func (d *deadlineStore) GetSnapshot(ctx context.Context, _ catalog.SnapshotIdentity, _ uint64) (catalog.PublishedSnapshot, error) {
	_, d.saw = ctx.Deadline()
	return catalog.PublishedSnapshot{}, context.Canceled
}
func TestMutationUsesServiceDeadline(t *testing.T) {
	d := &deadlineStore{}
	auth, e := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, e)
	source := &sourcePublicationReaderStub{}
	s := &Service{store: d, auth: auth, reader: d, sourceReader: source}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "a", TenantID: "B", EffectiveOrganizationID: "B", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)})
	_, e = s.Apply(ctx, "op", "00000000-0000-0000-0000-000000000001", ApplyInput{ExpectedRevision: 2})
	require.ErrorIs(t, e, context.Canceled)
	require.True(t, d.saw, "transaction reads must retain service deadline")
}
