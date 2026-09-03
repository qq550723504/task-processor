package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/listingkit/core"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

type revisionResolutionCategoryResolver struct {
	resolution *sheinpub.CategoryResolution
}

func (r revisionResolutionCategoryResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.CategoryResolution {
	return r.resolution
}

type revisionResolutionAttributeResolver struct {
	resolution *sheinpub.AttributeResolution
}

func (r revisionResolutionAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	return r.resolution
}

type revisionRegenerationAttributeResolver struct {
	resolved     *sheinpub.AttributeResolution
	fresh        *sheinpub.AttributeResolution
	clearErr     error
	resolveCalls int
	freshCalls   int
	clearCalls   int
	events       []string
}

func (r *revisionRegenerationAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	r.resolveCalls++
	r.events = append(r.events, "resolve")
	return r.resolved
}

func (r *revisionRegenerationAttributeResolver) ResolveFreshAttributeResolution(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	r.freshCalls++
	r.events = append(r.events, "resolve_fresh")
	return r.fresh
}

func (r *revisionRegenerationAttributeResolver) RememberAttributeResolution(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package, *sheinpub.AttributeResolution) {
}

func (r *revisionRegenerationAttributeResolver) ClearAttributeResolution(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) error {
	r.clearCalls++
	r.events = append(r.events, "clear")
	return r.clearErr
}

type revisionResolutionSaleAttributeResolver struct {
	resolution *sheinpub.SaleAttributeResolution
}

func (r revisionResolutionSaleAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	return r.resolution
}

type revisionAtomicRepository struct {
	Repository
	task             *Task
	mutationAttempts int
	mutationCommits  int
}

func (r *revisionAtomicRepository) GetTask(context.Context, string) (*Task, error) {
	return cloneRevisionAtomicTask(r.task)
}

func (r *revisionAtomicRepository) MutateTaskResult(_ context.Context, _ string, mutate TaskResultMutation) (*Task, error) {
	r.mutationAttempts++
	candidate, err := cloneRevisionAtomicTask(r.task)
	if err != nil {
		return nil, err
	}
	if mutate != nil {
		if err := mutate(candidate); err != nil {
			return nil, err
		}
	}
	r.task = candidate
	r.mutationCommits++
	return cloneRevisionAtomicTask(r.task)
}

func cloneRevisionAtomicTask(task *Task) (*Task, error) {
	if task == nil {
		return nil, nil
	}
	raw, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}
	var cloned Task
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func TestApplyTaskRevisionFailsClosedWithoutCommittingNilResolution(t *testing.T) {
	t.Parallel()

	manualRefresh := "manual_refresh"
	tests := []struct {
		name       string
		request    *ApplyRevisionRequest
		configure  func(*sheinRuntimeDependencies)
		wantErrKey string
	}{
		{
			name: "manual category refresh",
			request: &ApplyRevisionRequest{Platform: "shein", Actor: "reviewer", Reason: "refresh category", Shein: &SheinRevisionInput{
				CategoryResolution: &SheinCategoryResolutionPatch{Source: &manualRefresh},
			}},
			configure: func(deps *sheinRuntimeDependencies) {
				deps.categoryResolver = revisionResolutionCategoryResolver{}
			},
			wantErrKey: "category resolution is unavailable",
		},
		{
			name: "regenerate attributes",
			request: &ApplyRevisionRequest{Platform: "shein", Actor: "reviewer", Reason: "refresh attributes", Shein: &SheinRevisionInput{
				RegenerateAttributes: true,
			}},
			configure: func(deps *sheinRuntimeDependencies) {
				deps.attributeResolver = revisionResolutionAttributeResolver{}
			},
			wantErrKey: "attribute resolution is unavailable",
		},
		{
			name: "regenerate sale attributes",
			request: &ApplyRevisionRequest{Platform: "shein", Actor: "reviewer", Reason: "refresh sale attributes", Shein: &SheinRevisionInput{
				RegenerateSaleAttributes: true,
			}},
			configure: func(deps *sheinRuntimeDependencies) {
				deps.saleAttributeResolver = revisionResolutionSaleAttributeResolver{}
			},
			wantErrKey: "sale-attribute resolution is unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := revisionFailClosedTaskFixture()
			repoTask, err := cloneRevisionAtomicTask(original)
			if err != nil {
				t.Fatal(err)
			}
			repo := &revisionAtomicRepository{task: repoTask}
			deps := completeRevisionResolutionDependencies()
			tt.configure(&deps)
			svc := &service{repo: repo, sheinRuntimeDeps: deps}

			preview, err := svc.ApplyTaskRevision(context.Background(), original.ID, tt.request)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrKey) {
				t.Fatalf("ApplyTaskRevision() preview/error = %#v/%v, want %q", preview, err, tt.wantErrKey)
			}
			if preview != nil {
				t.Fatalf("ApplyTaskRevision() preview = %#v, want nil", preview)
			}
			if repo.mutationAttempts != 1 || repo.mutationCommits != 0 {
				t.Fatalf("mutation attempts/commits = %d/%d, want 1/0", repo.mutationAttempts, repo.mutationCommits)
			}
			assertRevisionTaskJSONEqual(t, repo.task, original)
		})
	}
}

func TestRegenerateAttributesResolvesFreshStateBeforeClearingCacheAndCommitsOnlyAfterClear(t *testing.T) {
	t.Parallel()

	clearErr := errors.New("attribute cache clear failed")
	tests := []struct {
		name             string
		resolver         *revisionRegenerationAttributeResolver
		wantErr          string
		wantEvents       []string
		wantCommit       bool
		wantAttributeVal string
	}{
		{
			name:       "missing resolver",
			wantErr:    "attribute resolver is required",
			wantEvents: nil,
		},
		{
			name: "fresh resolution fails",
			resolver: &revisionRegenerationAttributeResolver{
				resolved: &sheinpub.AttributeResolution{Status: "resolved", Source: "stale-cache", CategoryID: 42},
			},
			wantErr:    "attribute resolution is unavailable",
			wantEvents: []string{"resolve_fresh"},
		},
		{
			name: "cache clear fails",
			resolver: &revisionRegenerationAttributeResolver{
				resolved: &sheinpub.AttributeResolution{Status: "resolved", Source: "stale-cache", CategoryID: 42},
				fresh:    &sheinpub.AttributeResolution{Status: "resolved", Source: "fresh", CategoryID: 42},
				clearErr: clearErr,
			},
			wantErr:    clearErr.Error(),
			wantEvents: []string{"resolve_fresh", "clear"},
		},
		{
			name: "successful regeneration",
			resolver: &revisionRegenerationAttributeResolver{
				resolved: &sheinpub.AttributeResolution{Status: "resolved", Source: "stale-cache", CategoryID: 42},
				fresh: &sheinpub.AttributeResolution{
					Status: "resolved", Source: "fresh", CategoryID: 42,
					ResolvedAttributes: []sheinpub.ResolvedAttribute{{Name: "Color", Value: "Blue"}},
				},
			},
			wantEvents:       []string{"resolve_fresh", "clear"},
			wantCommit:       true,
			wantAttributeVal: "Blue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := revisionFailClosedTaskFixture()
			repoTask, err := cloneRevisionAtomicTask(original)
			require.NoError(t, err)
			repo := &revisionAtomicRepository{task: repoTask}
			deps := completeRevisionResolutionDependencies()
			if tt.resolver == nil {
				deps.attributeResolver = nil
			} else {
				deps.attributeResolver = tt.resolver
			}
			svc := &service{repo: repo, sheinRuntimeDeps: deps}

			preview, err := svc.ApplyTaskRevision(context.Background(), original.ID, &ApplyRevisionRequest{
				Platform: "shein", Actor: "reviewer", Reason: "regenerate attributes",
				Shein: &SheinRevisionInput{RegenerateAttributes: true},
			})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, preview)
			} else {
				require.NoError(t, err)
				require.NotNil(t, preview)
			}
			if tt.resolver != nil {
				require.Equal(t, tt.wantEvents, tt.resolver.events)
				require.Zero(t, tt.resolver.resolveCalls, "regeneration must bypass the cached Resolve path")
			}
			if tt.wantCommit {
				require.Equal(t, 1, repo.mutationCommits)
				require.Equal(t, "fresh", repo.task.Result.Shein.AttributeResolution.Source)
				require.Equal(t, tt.wantAttributeVal, repo.task.Result.Shein.AttributeResolution.ResolvedAttributes[0].Value)
			} else {
				require.Zero(t, repo.mutationCommits)
				assertRevisionTaskJSONEqual(t, repo.task, original)
			}
		})
	}
}

func TestRefreshSheinDerivedStateAcceptsExplicitPartialResolutionOutputs(t *testing.T) {
	t.Parallel()

	manualRefresh := "manual_refresh"
	tests := []struct {
		name    string
		request *ApplyRevisionRequest
	}{
		{
			name: "manual category refresh",
			request: &ApplyRevisionRequest{Platform: "shein", Shein: &SheinRevisionInput{
				CategoryResolution: &SheinCategoryResolutionPatch{Source: &manualRefresh},
			}},
		},
		{
			name: "regenerate attributes",
			request: &ApplyRevisionRequest{Platform: "shein", Shein: &SheinRevisionInput{
				RegenerateAttributes: true,
			}},
		},
		{
			name: "regenerate sale attributes",
			request: &ApplyRevisionRequest{Platform: "shein", Shein: &SheinRevisionInput{
				RegenerateSaleAttributes: true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := revisionFailClosedTaskFixture()
			deps := sheinRuntimeDependencies{
				categoryResolver: revisionResolutionCategoryResolver{resolution: &sheinpub.CategoryResolution{
					Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote category ambiguity"},
				}},
				attributeResolver: revisionResolutionAttributeResolver{resolution: &sheinpub.AttributeResolution{
					Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote attribute ambiguity"},
				}},
				saleAttributeResolver: revisionResolutionSaleAttributeResolver{resolution: &sheinpub.SaleAttributeResolution{
					Status: "partial", Source: "remote_api", CategoryID: 42, ReviewNotes: []string{"remote sale-attribute ambiguity"},
				}},
			}
			svc := &service{sheinRuntimeDeps: deps}

			if err := svc.refreshSheinDerivedState(task, tt.request); err != nil {
				t.Fatalf("refreshSheinDerivedState() error = %v, want explicit business partial to remain valid", err)
			}
			pkg := task.Result.Shein
			if pkg.CategoryResolution == nil || pkg.AttributeResolution == nil || pkg.SaleAttributeResolution == nil {
				t.Fatalf("refreshSheinDerivedState() resolutions = %#v/%#v/%#v, want non-nil partial resolutions", pkg.CategoryResolution, pkg.AttributeResolution, pkg.SaleAttributeResolution)
			}
			if pkg.CategoryResolution.Status != "partial" && tt.name == "manual category refresh" {
				t.Fatalf("category status = %q, want partial", pkg.CategoryResolution.Status)
			}
			if pkg.AttributeResolution.Status != "partial" {
				t.Fatalf("attribute status = %q, want partial", pkg.AttributeResolution.Status)
			}
			if tt.name != "regenerate attributes" && pkg.SaleAttributeResolution.Status != "partial" {
				t.Fatalf("sale-attribute status = %q, want partial", pkg.SaleAttributeResolution.Status)
			}
		})
	}
}

func completeRevisionResolutionDependencies() sheinRuntimeDependencies {
	return sheinRuntimeDependencies{
		categoryResolver: revisionResolutionCategoryResolver{resolution: &sheinpub.CategoryResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
		attributeResolver: revisionResolutionAttributeResolver{resolution: &sheinpub.AttributeResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
		saleAttributeResolver: revisionResolutionSaleAttributeResolver{resolution: &sheinpub.SaleAttributeResolution{
			Status: "resolved", Source: "remote_fixture", CategoryID: 42,
		}},
	}
}

func revisionFailClosedTaskFixture() *Task {
	now := time.Date(2026, 9, 3, 9, 30, 0, 0, time.UTC)
	return &Task{
		ID: "revision-fail-closed-task", TenantID: "tenant-test", Status: core.TaskStatusNeedsReview, Error: "original review state",
		Request: &GenerateRequest{Platforms: []string{"shein"}, Country: "US", Language: "en", SheinStoreID: 869},
		Result: &ListingKitResult{
			TaskID: "revision-fail-closed-task", Status: string(core.TaskStatusNeedsReview), ReviewReasons: []string{"original review state"},
			CanonicalProduct: &canonical.Product{
				Title: "Original Product", Attributes: map[string]canonical.Attribute{"color": {Value: "Black"}},
				Variants: []canonical.Variant{{SKU: "SKU-1", Attributes: map[string]canonical.Attribute{"color": {Value: "Black"}}}},
			},
			Shein: &sheinpub.Package{
				SpuName: "Original SPU", ProductNameEn: "Original Product", CategoryID: 42,
				CategoryResolution:      &sheinpub.CategoryResolution{Status: "resolved", Source: "original", CategoryID: 42},
				AttributeResolution:     &sheinpub.AttributeResolution{Status: "resolved", Source: "original", CategoryID: 42},
				SaleAttributeResolution: &sheinpub.SaleAttributeResolution{Status: "resolved", Source: "original", CategoryID: 42},
				DraftPayload:            &sheinpub.RequestDraft{SpuName: "Original SPU", SupplierCode: "SKU-1"},
				ReviewNotes:             []string{"original package review"},
			},
			Summary:              &GenerationSummary{NeedsReview: true, Warnings: []string{"original warning"}},
			Revision:             &ListingKitRevisionSummary{UpdatedAt: now, UpdatedBy: "original actor", Reason: "original reason", Platform: "shein"},
			RevisionHistoryTotal: 1,
			RevisionHistory: []ListingKitRevisionRecord{{
				RevisionID: "revision-original", UpdatedAt: now, UpdatedBy: "original actor", Reason: "original reason", Platform: "shein",
			}},
			CreatedAt: now, UpdatedAt: now,
		},
		CreatedAt: now, UpdatedAt: now,
	}
}

func assertRevisionTaskJSONEqual(t *testing.T, got, want *Task) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("persisted task changed after failed revision\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}
