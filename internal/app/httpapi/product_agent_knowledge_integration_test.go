package httpapi

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strings"
	knowledgestore "task-processor/internal/integration/persistence/knowledge"
	"task-processor/internal/knowledge"
	"testing"
)

// The consumer exercises real metadata/processing owners. It performs no
// object I/O or remote model call.
type consumerKnowledgeObjects struct{}

func (consumerKnowledgeObjects) PutImmutable(context.Context, knowledge.Object) error {
	return knowledge.ErrUnavailable
}
func (consumerKnowledgeObjects) Inspect(context.Context, knowledge.Object) (knowledge.Inspection, error) {
	return knowledge.Inspection{}, knowledge.ErrUnavailable
}
func (consumerKnowledgeObjects) ReadBounded(context.Context, knowledge.Object, int64) ([]byte, error) {
	return nil, knowledge.ErrUnavailable
}

type consumerKnowledgeFixture struct {
	repo    *knowledgestore.Repository
	service *knowledge.Service
	base    knowledge.Base
	source  knowledge.Source
	t       *testing.T
}

func newConsumerKnowledgeFixture(t *testing.T, db *gorm.DB) *consumerKnowledgeFixture {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, knowledgestore.Install(ctx, db))
	repo, err := knowledgestore.NewRepository(ctx, db)
	require.NoError(t, err)
	service, err := knowledge.NewService(repo, consumerKnowledgeObjects{})
	require.NoError(t, err)
	base, err := service.Mutate(ctx, knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_create", Key: uuid.NewString(), Name: "Brand guide"})
	require.NoError(t, err)
	f := &consumerKnowledgeFixture{repo: repo, service: service, base: *base.Base, t: t}
	f.source = f.addSource(f.base.ID, "Frozen brand text", "INCOMPLETE_EXTRACTION")
	return f
}
func (f *consumerKnowledgeFixture) addSource(base, text, warning string) knowledge.Source {
	f.t.Helper()
	key := uuid.NewString()
	result, err := f.repo.Apply(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "source_create", BaseID: base, Key: key, Fingerprint: knowledge.Digest([]byte(key)), Name: "Brand wording", Upload: &knowledge.Revision{Filename: "guide.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: knowledge.Digest([]byte("hello"))}})
	require.NoError(f.t, err)
	f.promote(*result.Revision, text, warning)
	source, err := f.repo.GetSource(context.Background(), "B", result.Source.ID)
	require.NoError(f.t, err)
	return source
}
func (f *consumerKnowledgeFixture) promote(revision knowledge.Revision, text, warning string) {
	ctx := context.Background()
	claim, ok, err := f.repo.ClaimUpload(ctx, "B", revision.ID, uuid.NewString())
	require.NoError(f.t, err)
	require.True(f.t, ok)
	require.NoError(f.t, f.repo.ConfirmObject(ctx, claim))
	revisions, err := f.repo.ClaimProcessing(ctx, uuid.NewString(), 4)
	require.NoError(f.t, err)
	for _, revision := range revisions {
		require.NoError(f.t, f.repo.Finish(ctx, revision, knowledge.ParseResult{Text: text, Warning: warning}))
	}
}
func (f *consumerKnowledgeFixture) replaceSource() {
	key := uuid.NewString()
	result, err := f.repo.Apply(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "revision_create", SourceID: f.source.ID, Version: f.source.Version, Key: key, Fingerprint: knowledge.Digest([]byte(key)), Name: f.source.Name, Upload: &knowledge.Revision{Filename: "guide2.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: knowledge.Digest([]byte("newer"))}})
	require.NoError(f.t, err)
	f.promote(*result.Revision, "Newer brand text", "")
}
func (f *consumerKnowledgeFixture) oversizedBase() string {
	result, err := f.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_create", Key: uuid.NewString(), Name: "Oversized guide"})
	require.NoError(f.t, err)
	f.addSource(result.Base.ID, strings.Repeat("x", knowledge.MaxContextRevisionBytes+1), "")
	return result.Base.ID
}
func (f *consumerKnowledgeFixture) escapedBase() string {
	result, err := f.service.Mutate(context.Background(), knowledge.Command{Scope: knowledge.Scope{OrganizationID: "B", ActorID: "operator"}, Kind: "base_create", Key: uuid.NewString(), Name: "Escaped guide"})
	require.NoError(f.t, err)
	for range 3 {
		f.addSource(result.Base.ID, "Guide"+strings.Repeat("\n", 14600)+"end", "")
	}
	return result.Base.ID
}
func TestProductAgentKnowledgeRealOwnersFreezeRetryClaimAndReview(t *testing.T) {
	testProductAgentOwners(t, "knowledge")
}
