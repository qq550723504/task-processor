package podapp

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type approvalReader struct{ commit asset.ApprovalCommit }

func (r approvalReader) ReadApprovalCommit(context.Context, string, string) (asset.ApprovalCommit, error) {
	return r.commit, nil
}

type snapshotReader struct{ snapshot catalog.PublishedSnapshot }

func (r snapshotReader) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return r.snapshot, nil
}

type imageTransport func(*http.Request) (*http.Response, error)

func (f imageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestApprovedArtworkRequiresCompleteImageBeforeAnyProviderUpload(t *testing.T) {
	scope := collection.Scope{"org", "actor", "member"}
	pub := uuid.NewString()
	source := uuid.NewString()
	action := uuid.NewString()
	url := "https://images.example.org/approved.png"
	ref := pod.ArtworkReference{Input: pod.InputReference{ItemID: uuid.NewString(), Revision: 1, Source: collection.Source{ProductKey: "own-product", PublicationID: pub, Version: 1, Kind: "own"}}, Version: 1, ActionID: action, AssetID: "approved"}
	commit := asset.ApprovalCommit{TenantID: scope.OrganizationID, ProductKey: ref.Input.Source.ProductKey, TargetPlatform: "sds", ActionID: action, SourceSnapshotVersion: 1, Assets: []asset.ApprovedAsset{{ID: ref.AssetID, Role: asset.RoleDesign, URL: url, SourceAssetID: source, SourceApproval: &asset.SourceApprovalProvenance{OriginalPublicationID: pub, OriginalSnapshotVersion: 1, ActorID: scope.ActorID, MemberID: scope.MemberID, ReferenceHash: asset.ReferenceHash(source, url)}}}}
	var content bytes.Buffer
	require.NoError(t, png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	raw := content.Bytes()[:33]
	inputs := OriginalInputs{Approvals: approvalReader{commit}, Snapshots: snapshotReader{catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: ref.Input.Source.ProductKey}, PublicationID: pub, Version: 1}}, ImageHTTP: &http.Client{Transport: imageTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
	})}}
	_, _, e := inputs.bytes(context.Background(), scope, ref)
	require.ErrorIs(t, e, pod.ErrInvalid)
	raw = content.Bytes()
	_, frozen, e := inputs.bytes(context.Background(), scope, ref)
	require.NoError(t, e)
	require.Len(t, frozen.Hash, 64)
	raw = append(append([]byte(nil), raw...), 1)
	_, _, e = inputs.bytes(context.Background(), scope, frozen)
	require.ErrorIs(t, e, pod.ErrConflict)
	commit.Assets[0].SourceApproval.MemberID = "another-member"
	_, _, e = inputs.bytes(context.Background(), scope, ref)
	require.ErrorIs(t, e, pod.ErrForbidden)
}
