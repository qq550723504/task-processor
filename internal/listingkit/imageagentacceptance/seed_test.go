package imageagentacceptance

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"gorm.io/gorm"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
)

func TestSeedDerivesOwnerAndCreatesMinimalSheinTask(t *testing.T) {
	repo := &recordingRepository{Repository: store.NewMemTaskRepository()}
	verifier := stubVerifier{identity: authidentity.AuthenticatedIdentity{
		TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"},
	}}

	result, err := Seed(context.Background(), acceptingGuard{}, verifier, repo, SeedRequest{
		Token:     "real-token",
		SourceURL: "https://cdn.example.test/source.png",
		StyleURL:  "https://cdn.example.test/style.png",
	})
	if err != nil {
		t.Fatalf("Seed() error = %v", err)
	}
	if result.TenantID != "org-1" || result.UserID != "user-1" || result.TaskID == "" {
		t.Fatalf("Seed() result = %+v, want derived owner and task ID", result)
	}
	if len(result.TaskID) != 36 {
		t.Fatalf("Seed() task ID length = %d, want 36", len(result.TaskID))
	}
	if result.WorkspaceURL != "/listing-kits/"+result.TaskID+"/workspace" {
		t.Fatalf("Seed() workspace URL = %q, want task workspace URL", result.WorkspaceURL)
	}

	task, err := repo.GetTask(context.Background(), result.TaskID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.TenantID != "org-1" || task.UserID != "user-1" || task.Result == nil || task.Result.StandardProductSnapshot == nil {
		t.Fatalf("seeded task identity/result = %+v, want derived owner and standard snapshot", task)
	}
	bundle := task.Result.AssetBundlesByTarget["shein"]
	if bundle == nil || len(bundle.Assets) != 2 {
		t.Fatalf("seeded SHEIN assets = %+v, want source and style assets", bundle)
	}
	if !reflect.DeepEqual(bundle.Assets[0], asset.Asset{ID: "image-agent-acceptance-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png"}) {
		t.Fatalf("source asset = %+v, want the canonical source asset", bundle.Assets[0])
	}
	if !reflect.DeepEqual(bundle.Assets[1], asset.Asset{ID: "image-agent-acceptance-style", Kind: asset.KindGalleryImage, URL: "https://cdn.example.test/style.png"}) {
		t.Fatalf("style asset = %+v, want a non-source style asset", bundle.Assets[1])
	}
	if repo.creates != 1 {
		t.Fatalf("CreateTask calls = %d, want 1", repo.creates)
	}
}

func TestSeedRerunIsIdempotentButDifferentSourceOrOwnerFails(t *testing.T) {
	repo := &recordingRepository{Repository: store.NewMemTaskRepository()}
	verifier := stubVerifier{identity: authidentity.AuthenticatedIdentity{
		TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_admin"},
	}}
	request := SeedRequest{Token: "token", SourceURL: "https://cdn.example.test/source.png"}

	first, err := Seed(context.Background(), acceptingGuard{}, verifier, repo, request)
	if err != nil {
		t.Fatalf("first Seed() error = %v", err)
	}
	second, err := Seed(context.Background(), acceptingGuard{}, verifier, repo, request)
	if err != nil {
		t.Fatalf("identical Seed() error = %v", err)
	}
	if second != first || repo.creates != 1 {
		t.Fatalf("identical Seed() = %+v, creates = %d; want same result and one create", second, repo.creates)
	}

	if _, err := Seed(context.Background(), acceptingGuard{}, verifier, repo, SeedRequest{Token: "token", SourceURL: "https://cdn.example.test/changed.png"}); err == nil || !strings.Contains(err.Error(), "refuses to overwrite") {
		t.Fatalf("changed source Seed() error = %v, want overwrite rejection", err)
	}

	existing, err := repo.GetTask(context.Background(), first.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	existing.UserID = "different-user"
	repo.override = existing
	if _, err := Seed(context.Background(), acceptingGuard{}, verifier, repo, request); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("changed owner Seed() error = %v, want owner rejection", err)
	}
}

func TestSeedRejectsMissingRoleAndUnsafeURLs(t *testing.T) {
	tests := []struct {
		name     string
		identity authidentity.AuthenticatedIdentity
		request  SeedRequest
	}{
		{
			name:     "missing required role",
			identity: authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_viewer"}},
			request:  SeedRequest{Token: "token", SourceURL: "https://cdn.example.test/source.png"},
		},
		{
			name:     "unsafe source URL",
			identity: authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1", Roles: []string{"platform_admin"}},
			request:  SeedRequest{Token: "token", SourceURL: "http://localhost/source.png"},
		},
		{
			name:     "unsafe style URL",
			identity: authidentity.AuthenticatedIdentity{TenantID: "org-1", UserID: "user-1", Roles: []string{"platform_admin"}},
			request:  SeedRequest{Token: "token", SourceURL: "https://cdn.example.test/source.png", StyleURL: "http://localhost/style.png"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingRepository{Repository: store.NewMemTaskRepository()}
			if _, err := Seed(context.Background(), acceptingGuard{}, stubVerifier{identity: tt.identity}, repo, tt.request); err == nil {
				t.Fatal("Seed() accepted an invalid request")
			}
			if repo.creates != 0 {
				t.Fatalf("CreateTask calls = %d, want no mutation", repo.creates)
			}
		})
	}
}

type acceptingGuard struct{}

func (acceptingGuard) Verify(context.Context, RuntimeConfig) (*gorm.DB, error) { return nil, nil }

type stubVerifier struct {
	identity authidentity.AuthenticatedIdentity
	err      error
}

func (s stubVerifier) Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
	return s.identity, s.err
}

type recordingRepository struct {
	listingkit.Repository
	creates  int
	override *listingkit.Task
}

func (r *recordingRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	if r.override != nil && r.override.ID == taskID {
		return r.override, nil
	}
	return r.Repository.GetTask(ctx, taskID)
}

func (r *recordingRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	r.creates++
	return r.Repository.CreateTask(ctx, task)
}

var _ listingkit.Repository = (*recordingRepository)(nil)
