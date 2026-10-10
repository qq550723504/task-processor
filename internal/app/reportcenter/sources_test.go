package reportcenterapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	validator "task-processor/internal/marketplace/validator"
	"task-processor/internal/product/review"
	rc "task-processor/internal/reportcenter"
	"testing"
	"time"
)

type allowPolicy bool

func (p allowPolicy) Authorize(string, []string, string) bool { return bool(p) }

type reviewFunc func(context.Context, string) (review.View, error)

func (f reviewFunc) Get(c context.Context, id string) (review.View, error) { return f(c, id) }

type recordFunc func(context.Context, listingtask.Actor, string) (record.Record, error)

func (f recordFunc) ReadOfflinePackage(c context.Context, a listingtask.Actor, id string) (record.Record, error) {
	return f(c, a, id)
}
func TestSourceAuthorizationUsesTheVerifiedUserSubject(t *testing.T) {
	policy, err := authz.NewListingKitAuthorizer([]string{"configured-user"}, nil)
	require.NoError(t, err)
	calls := 0
	s := &Sources{Policy: policy, Reviews: reviewFunc(func(context.Context, string) (review.View, error) { calls++; return review.View{}, review.ErrNotFound }), Records: recordFunc(func(context.Context, listingtask.Actor, string) (record.Record, error) {
		calls++
		return record.Record{}, record.ErrNotFound
	})}
	for _, user := range []string{"configured-user", "unconfigured-user"} {
		ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: user, TenantID: "org-a", EffectiveOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)})
		for _, kind := range []string{"TITLE_REVIEW", "SHEIN_RECORD"} {
			_, err := s.Read(ctx, rc.Scope{OrganizationID: "org-a", ActorID: user}, kind, uuid.NewString())
			if user == "configured-user" {
				require.ErrorIs(t, err, rc.ErrNotFound)
			} else {
				require.ErrorIs(t, err, rc.ErrForbidden)
			}
		}
	}
	require.Equal(t, 2, calls)
}

func TestReviewSourceCapturesApplyAtSameRevisionAndEmptyBefore(t *testing.T) {
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: scope.ActorID, TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, TokenExpiresAt: time.Now().Add(time.Hour)})
	id := uuid.NewString()
	view := review.View{ID: id, Owner: scope.ActorID, Input: review.CreateInput{ProductKey: "product-a", BaseVersion: 1}, Before: "", Title: "suggested title", Policy: "title-review-v1", State: "accepted", Revision: 2}
	require.NoError(t, review.ValidateView(view))
	s := Sources{Policy: allowPolicy(true), Reviews: reviewFunc(func(context.Context, string) (review.View, error) { return view, nil })}
	first, e := s.Read(ctx, scope, "TITLE_REVIEW", id)
	require.NoError(t, e)
	_, _, e = first.Bytes()
	require.NoError(t, e)
	require.Equal(t, "2:accepted", first.Ref.Version)
	view.State = "applied"
	view.Receipt = &review.Receipt{ProposalID: id, Revision: 2, ProductVersion: 2, PublicationID: uuid.NewString(), Actor: scope.ActorID, At: time.Now().UTC()}
	applied, e := s.Read(ctx, scope, "TITLE_REVIEW", id)
	require.NoError(t, e)
	_, _, e = applied.Bytes()
	require.NoError(t, e)
	require.Equal(t, "2:applied", applied.Ref.Version)
	require.Len(t, applied.Content.Sections, 2)
}

func TestRecordSourceUsesSavedDiagnosticWithoutReadingPayloadOrCurrentTime(t *testing.T) {
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: scope.ActorID, TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, TokenExpiresAt: time.Now().Add(time.Hour)})
	at := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	// record.Reader returns canonical digests with the algorithm prefix.
	hash := "sha256:" + strings.Repeat("a", 64)
	diagnostic := validator.DiagnosticResult{DiagnosticOnly: true, Scope: "offline", Target: validator.Target{Marketplace: "shein"}, Action: validator.SaveDraft, RuleVersion: "rules-v1", Input: validator.BoundInput{Digest: hash, BindingVersion: "policy-v1", ReadAt: at, EvaluatedAt: at}, Freshness: validator.ExternalFreshness{Status: validator.NotEvaluated}, OfflineChecks: validator.OfflineChecks{Status: validator.Blocked, Checks: []validator.Check{{Code: "ASSET_MISSING", Status: validator.CheckBlocking, Message: "缺少图片"}}}, NotEvaluated: []string{"platform_submission"}}
	raw, _ := json.Marshal(diagnostic)
	sum := sha256.Sum256(raw)
	r := record.Record{ID: uuid.NewString(), OrganizationID: scope.OrganizationID, OwnerUserID: scope.ActorID, Input: record.Input{ProductKey: "product-a", SnapshotVersion: 1, StoreID: uuid.NewString(), Country: "US", Language: "en", Action: validator.SaveDraft}, InputHash: hash, PackageHash: hash, DiagnosticHash: "sha256:" + hex.EncodeToString(sum[:]), RuleRevision: "rules-v1", PolicyRevision: "policy-v1", DiagnosticStatus: validator.Blocked, Diagnostic: raw, CreatedAt: at, Payload: []byte("secret complete payload must not be copied")}
	s := Sources{Policy: allowPolicy(true), Records: recordFunc(func(context.Context, listingtask.Actor, string) (record.Record, error) {
		r.ReadAt = time.Now()
		return r, nil
	})}
	first, e := s.Read(ctx, scope, "SHEIN_RECORD", r.ID)
	require.NoError(t, e)
	body, digest, e := first.Bytes()
	require.NoError(t, e)
	require.NotContains(t, string(body), "secret complete payload")
	require.Contains(t, string(body), "ASSET_MISSING")
	require.Contains(t, string(body), "not_evaluated")
	second, e := s.Read(ctx, scope, "SHEIN_RECORD", r.ID)
	require.NoError(t, e)
	_, nextDigest, e := second.Bytes()
	require.NoError(t, e)
	require.Equal(t, digest, nextDigest)
}
func TestSourcesNeverInheritAdminOwnershipBypass(t *testing.T) {
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: scope.ActorID, TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)})
	id := uuid.NewString()
	s := Sources{Policy: allowPolicy(true), Reviews: reviewFunc(func(context.Context, string) (review.View, error) { return review.View{ID: id, Owner: "other"}, nil }), Records: recordFunc(func(_ context.Context, a listingtask.Actor, _ string) (record.Record, error) {
		require.Equal(t, scope.ActorID, a.UserID)
		return record.Record{ID: id, OrganizationID: scope.OrganizationID, OwnerUserID: "other"}, nil
	})}
	for _, kind := range []string{"TITLE_REVIEW", "SHEIN_RECORD"} {
		_, e := s.Read(ctx, scope, kind, id)
		require.ErrorIs(t, e, rc.ErrNotFound)
	}
	s.Policy = allowPolicy(false)
	_, e := s.Read(ctx, scope, "TITLE_REVIEW", id)
	require.ErrorIs(t, e, rc.ErrForbidden)
	s.Policy = allowPolicy(true)
	_, e = s.Read(ctx, rc.Scope{OrganizationID: "org-b", ActorID: scope.ActorID}, "TITLE_REVIEW", id)
	require.ErrorIs(t, e, rc.ErrForbidden)
	var _ authz.StaticAuthorizer = allowPolicy(true)
}
