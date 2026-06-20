# ListingKit SDS Batch Task Gating And Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make SDS batch task creation rely on batch graph only, enforce compatibility-aware backend gating, and show real task/submission state in the batch UI without changing the approved-by-default design rule.

**Architecture:** Add a batch-native task preparation path inside `internal/listingkit` that computes compatibility fingerprints, validates every candidate against baseline/store/design ownership, and returns structured created/rejected/failed outcomes. Extend batch detail projections with explicit task lifecycle state, then update the Next.js workbench and grouped-selection helpers to consume those fields instead of inferring state from task titles or raw `shared_by_size`.

**Tech Stack:** Go, existing `internal/listingkit` service/repository tests, GORM-backed models, Next.js/TypeScript, Vitest, `apply_patch`

---

### Task 1: Add failing backend tests for compatibility fingerprint and explicit task state grouping

**Files:**
- Create: `internal/listingkit/studio_batch_compatibility_test.go`
- Modify: `internal/listingkit/studio_batch_status_groups_test.go`
- Test: `internal/listingkit/studio_batch_compatibility_test.go`
- Test: `internal/listingkit/studio_batch_status_groups_test.go`

- [ ] **Step 1: Write the failing compatibility tests**

Add tests that lock down the new fingerprint contract:

```go
func TestStudioBatchCompatibilityFingerprint_MatchesEquivalentSelections(t *testing.T) {
	left := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: " https://cdn.example.com/template-a.png ",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}
	right := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: "https://cdn.example.com/template-a.png",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}

	if got, want := buildStudioBatchCompatibilityFingerprint(left), buildStudioBatchCompatibilityFingerprint(right); got != want {
		t.Fatalf("fingerprint mismatch: %q != %q", got, want)
	}
}

func TestStudioBatchCompatibilityFingerprint_DiffersWhenTemplateOrMaskDiffers(t *testing.T) {
	base := SheinStudioSelection{
		ParentProductID:  2002,
		PrototypeGroupID: 4004,
		LayerID:          "layer-front",
		DesignType:       "material",
		PrintableWidth:   1200,
		PrintableHeight:  1200,
		TemplateImageURL: "https://cdn.example.com/template-a.png",
		MaskImageURL:     "https://cdn.example.com/mask-a.png",
	}
	changed := base
	changed.TemplateImageURL = "https://cdn.example.com/template-b.png"

	if got, want := buildStudioBatchCompatibilityFingerprint(base), buildStudioBatchCompatibilityFingerprint(changed); got == want {
		t.Fatalf("fingerprints unexpectedly matched: %q", got)
	}
}
```

- [ ] **Step 2: Add the failing status-group tests**

Extend `internal/listingkit/studio_batch_status_groups_test.go` with projection-based grouping expectations:

```go
func TestBuildStudioBatchStatusGroups_UsesExplicitCreatedTaskState(t *testing.T) {
	detail := &StudioBatchDetail{
		CreatedTasks: []SheinStudioCreatedTask{
			{ID: "task-1", Title: "Style 1", DesignID: "design-1", Status: "task_created"},
			{ID: "task-2", Title: "Style 2", DesignID: "design-2", Status: "draft_saved"},
			{ID: "task-3", Title: "Style 3", DesignID: "design-3", Status: "published"},
		},
	}

	groups := BuildStudioBatchStatusGroups(detail)
	if got := groups.ByKey["task_created"].Count; got != 1 {
		t.Fatalf("task_created count = %d, want 1", got)
	}
	if got := groups.ByKey["draft_saved"].Count; got != 1 {
		t.Fatalf("draft_saved count = %d, want 1", got)
	}
	if got := groups.ByKey["published"].Count; got != 1 {
		t.Fatalf("published count = %d, want 1", got)
	}
}
```

- [ ] **Step 3: Run the targeted backend tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestStudioBatchCompatibilityFingerprint|TestBuildStudioBatchStatusGroups_UsesExplicitCreatedTaskState" -count=1`

Expected: FAIL because the fingerprint helper and explicit `SheinStudioCreatedTask.Status` grouping do not exist yet.

### Task 2: Implement backend compatibility fingerprint and real created-task state projection

**Files:**
- Create: `internal/listingkit/studio_batch_compatibility.go`
- Modify: `internal/listingkit/studio_session_model.go`
- Modify: `internal/listingkit/studio_batch_status_groups.go`
- Modify: `internal/listingkit/studio_batch_service.go`
- Test: `internal/listingkit/studio_batch_compatibility_test.go`
- Test: `internal/listingkit/studio_batch_status_groups_test.go`

- [ ] **Step 1: Add the compatibility fingerprint helper**

Create `internal/listingkit/studio_batch_compatibility.go`:

```go
package listingkit

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

func buildStudioBatchCompatibilityFingerprint(selection SheinStudioSelection) string {
	normalized := strings.Join([]string{
		int64String(selection.ParentProductID),
		int64String(selection.PrototypeGroupID),
		strings.TrimSpace(selection.LayerID),
		strings.TrimSpace(selection.DesignType),
		intString(selection.PrintableWidth),
		intString(selection.PrintableHeight),
		strings.TrimSpace(selection.TemplateImageURL),
		strings.TrimSpace(selection.MaskImageURL),
	}, "|")
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}
```

- [ ] **Step 2: Extend the created-task model with explicit projection fields**

Update `internal/listingkit/studio_session_model.go`:

```go
type SheinStudioCreatedTask struct {
	ID                       string `json:"id,omitempty"`
	Title                    string `json:"title,omitempty"`
	DesignID                 string `json:"design_id,omitempty"`
	ItemID                   string `json:"item_id,omitempty"`
	SelectionID              string `json:"selection_id,omitempty"`
	CompatibilityFingerprint string `json:"compatibility_fingerprint,omitempty"`
	Status                   string `json:"status,omitempty"`
	SubmissionState          string `json:"submission_state,omitempty"`
	LastSubmissionAction     string `json:"last_submission_action,omitempty"`
	ReasonCode               string `json:"reason_code,omitempty"`
	Message                  string `json:"message,omitempty"`
}
```

- [ ] **Step 3: Switch batch status grouping to explicit state**

Update `internal/listingkit/studio_batch_status_groups.go`:

```go
func studioBatchCreatedTaskGroup(task SheinStudioCreatedTask) (string, string) {
	switch strings.TrimSpace(task.Status) {
	case "task_created":
		return "task_created", "任务已创建"
	case "needs_review":
		return "needs_review", "待审核"
	case "ready_to_submit":
		return "ready_to_submit", "待提交"
	case "draft_saved":
		return "draft_saved", "草稿已保存"
	case "published":
		return "published", "已发布"
	case "submit_failed":
		return "submission_failed", "提交失败"
	default:
		return "task_created", "任务已创建"
	}
}
```

- [ ] **Step 4: Add rejected task support to the service result shape**

Update `internal/listingkit/studio_batch_service.go`:

```go
type SheinStudioRejectedTask struct {
	DesignID    string `json:"design_id,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	SelectionID string `json:"selection_id,omitempty"`
	ReasonCode  string `json:"reason_code,omitempty"`
	Message     string `json:"message,omitempty"`
}

type CreateStudioBatchTasksResult struct {
	Batch         *StudioBatchRecord        `json:"batch,omitempty"`
	Items         []StudioBatchItemDetail   `json:"items,omitempty"`
	CreatedTasks  []SheinStudioCreatedTask  `json:"created_tasks,omitempty"`
	RejectedTasks []SheinStudioRejectedTask `json:"rejected_tasks,omitempty"`
	FailedTasks   []SheinStudioFailedTask   `json:"failed_tasks,omitempty"`
}
```

- [ ] **Step 5: Run the targeted backend tests to verify they pass**

Run: `go test ./internal/listingkit -run "TestStudioBatchCompatibilityFingerprint|TestBuildStudioBatchStatusGroups_UsesExplicitCreatedTaskState" -count=1`

Expected: PASS.

### Task 3: Add failing batch-only task creation tests and remove the mandatory session dependency

**Files:**
- Modify: `internal/listingkit/studio_batch_service_test.go`
- Modify: `internal/listingkit/task_studio_batch_task_flow_support.go`
- Modify: `internal/listingkit/task_studio_batch_task_creation_adapter.go`
- Modify: `internal/listingkit/task_studio_batch_task_execute_adapter.go`
- Modify: `internal/listingkit/task_studio_batch_task_request_support.go`
- Create: `internal/listingkit/task_studio_batch_candidate_support.go`
- Test: `internal/listingkit/studio_batch_service_test.go`

- [ ] **Step 1: Add failing task-creation tests that do not provide a session**

Extend `internal/listingkit/studio_batch_service_test.go`:

```go
func TestServiceCreateStudioBatchTasks_UsesBatchGraphWithoutSession(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	if err := repo.CreateStudioBatchGraph(ctx, newStudioBatchRecordForTest("batch-1", now), newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}
	svc.studioDeps.sessionRepo = nil

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.CreatedTasks) != 1 {
		t.Fatalf("created tasks = %+v, want 1", result.CreatedTasks)
	}
}

func TestServiceCreateStudioBatchTasks_RejectsCompatibilityMismatch(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	taskRepo := newStudioBatchTaskRepositoryStub()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Now().UTC()

	batch := newStudioBatchRecordForTest("batch-1", now)
	batch.GroupedImageMode = "shared_by_size"
	batch.GroupedSelections = SheinStudioGroupedSelectionList{
		{
			SelectionID: "selection-1",
			Selection: SheinStudioSelection{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3003,
				PrototypeGroupID: 4004,
				LayerID:          "layer-front",
				DesignType:       "material",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
				TemplateImageURL: "https://cdn.example.com/template-a.png",
				MaskImageURL:     "https://cdn.example.com/mask-a.png",
			},
			Eligible: true,
		},
		{
			SelectionID: "selection-2",
			Selection: SheinStudioSelection{
				ProductID:        1001,
				ParentProductID:  2002,
				VariantID:        3004,
				PrototypeGroupID: 4004,
				LayerID:          "layer-front",
				DesignType:       "material",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
				TemplateImageURL: "https://cdn.example.com/template-b.png",
				MaskImageURL:     "https://cdn.example.com/mask-a.png",
			},
			Eligible: true,
		},
	}

	if err := repo.CreateStudioBatchGraph(ctx, batch, newStudioBatchItemsForTest("batch-1", now), newStudioBatchAttemptsForTest("item-1", now), []StudioMaterializedDesignRecord{
		{
			ID:              "design-1",
			BatchID:         "batch-1",
			ItemID:          "item-1",
			SourceAttemptID: "attempt-1",
			ImageURL:        "https://cdn.example.com/design-1.png",
			ReviewStatus:    StudioMaterializedDesignReviewStatusApproved,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}); err != nil {
		t.Fatalf("CreateStudioBatchGraph() error = %v", err)
	}

	svc := &service{studioDeps: studioDependencies{batchRepo: repo}}
	svc.repo = taskRepo
	svc.taskDeps.taskSubmitter = &studioBatchTaskSubmitterStub{}

	result, err := svc.CreateStudioBatchTasks(ctx, "batch-1", &CreateStudioBatchTasksRequest{
		DesignIDs: []string{"design-1"},
	})
	if err != nil {
		t.Fatalf("CreateStudioBatchTasks() error = %v", err)
	}
	if len(result.RejectedTasks) != 1 {
		t.Fatalf("rejected tasks = %+v, want 1", result.RejectedTasks)
	}
	if got := result.RejectedTasks[0].ReasonCode; got != "compatibility_mismatch" {
		t.Fatalf("reason code = %q, want compatibility_mismatch", got)
	}
}
```

- [ ] **Step 2: Run the targeted backend tests to verify they fail**

Run: `go test ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_UsesBatchGraphWithoutSession|TestServiceCreateStudioBatchTasks_RejectsCompatibilityMismatch" -count=1`

Expected: FAIL because task creation still loads `SheinStudioSession` and has no compatibility rejection path.

- [ ] **Step 3: Add a batch-native candidate builder**

Create `internal/listingkit/task_studio_batch_candidate_support.go`:

```go
type StudioBatchTaskState struct {
	Batch         *StudioBatchRecord
	DesignIDs     []string
	Candidates    []studioBatchTaskCandidate
	RejectedTasks []SheinStudioRejectedTask
}

type studioBatchTaskCandidate struct {
	Design                   StudioMaterializedDesignRecord
	Item                     StudioBatchItemRecord
	Selection                SheinStudioGroupedSelection
	SelectionSnapshot        SheinStudioSelection
	SelectionID              string
	CompatibilityFingerprint string
	StyleID                  string
	Title                    string
}

func (s *taskStudioBatchService) buildStudioBatchTaskCandidates(
	ctx context.Context,
	batch *StudioBatchRecord,
	detail *StudioBatchDetailGraph,
	designs []StudioMaterializedDesignRecord,
) ([]studioBatchTaskCandidate, []SheinStudioRejectedTask, error) {
	// derive selection snapshots from batch graph, compute fingerprint, and reject mismatches
}
```

- [ ] **Step 4: Remove mandatory session loads from task prepare/execute adapters**

Refactor `internal/listingkit/task_studio_batch_task_creation_adapter.go` and `internal/listingkit/task_studio_batch_task_execute_adapter.go` so they no longer call `LoadSession`:

```go
PrepareState: func(ctx context.Context, batchID string, designIDs []string) (studiodomain.BatchTaskPrepareState[*StudioBatchTaskState, StudioBatchRecord], error) {
	state, err := s.prepareStudioBatchTaskCreation(ctx, batchID, &CreateStudioBatchTasksRequest{
		DesignIDs: append([]string(nil), designIDs...),
	})
	if err != nil {
		return studiodomain.BatchTaskPrepareState[*StudioBatchTaskState, StudioBatchRecord]{}, err
	}
	return studiodomain.BatchTaskPrepareState[*StudioBatchTaskState, StudioBatchRecord]{
		Session:   state,
		Batch:     state.Batch,
		DesignIDs: state.DesignIDs,
	}, nil
},
```

- [ ] **Step 5: Build task requests from batch-native candidate data**

Update `internal/listingkit/task_studio_batch_task_request_support.go`:

```go
func buildStudioBatchTaskGenerateRequest(
	batch *StudioBatchRecord,
	candidate studioBatchTaskCandidate,
	design StudioMaterializedDesignRecord,
) *GenerateRequest {
	styleID := buildStudioBatchTaskStyleID(batch.ID, candidate.Item.ID, design.ID, candidate.SelectionID)
	return &GenerateRequest{
		Text:      strings.TrimSpace(batch.Prompt),
		Platforms: []string{"shein"},
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{
				StyleID: styleID,
				StyleName: candidate.Title,
			},
			SDS: buildStudioBatchTaskSDSOptions(candidate.SelectionSnapshot, styleID, candidate.Title),
		},
	}
}
```

- [ ] **Step 6: Replace the fragile style-id helper**

Update the helper signature and implementation:

```go
func buildStudioBatchTaskStyleID(batchID string, itemID string, designID string, selectionID string) string {
	raw := strings.Join([]string{batchID, itemID, designID, selectionID}, "|")
	sum := sha1.Sum([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))[:10]
}
```

- [ ] **Step 7: Run the targeted backend tests to verify they pass**

Run: `go test ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_UsesBatchGraphWithoutSession|TestServiceCreateStudioBatchTasks_RejectsCompatibilityMismatch|TestServiceCreateStudioBatchTasksUsesApprovedDesignOwnership" -count=1`

Expected: PASS.

### Task 4: Add failing frontend tests for grouped eligibility and explicit created-task status display

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Add the failing grouped-mode eligibility tests**

Extend `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`:

```ts
it("rejects shared_by_size task creation when grouped selections do not share a compatibility fingerprint", async () => {
  await expect(
    createGroupedSheinReviewTasks({
      prompt: "retro cherries",
      groupedImageMode: "shared_by_size",
      groups: [
        {
          sheinStoreId: "42",
          selections: [
            {
              selection: baseSelection,
              eligible: true,
              baselineStatus: "ready",
              compatibilityFingerprint: "fp-a",
            },
            {
              selection: { ...baseSelection, templateImageUrl: "https://cdn.example.com/template-b.png" },
              eligible: true,
              baselineStatus: "ready",
              compatibilityFingerprint: "fp-b",
            },
          ],
          designs: [baseDesign],
          selectedIds: [baseDesign.id],
        },
      ],
    }),
  ).rejects.toThrow("compatibility fingerprint");
});
```

- [ ] **Step 2: Add the failing workbench status-display tests**

Extend `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`:

```tsx
it("shows task_created separately from draft_saved and published", async () => {
  renderWorkbenchWithBatchDetail({
    createdTasks: [
      { id: "task-1", title: "Style 1", designId: "design-1", status: "task_created" },
      { id: "task-2", title: "Style 2", designId: "design-2", status: "draft_saved" },
      { id: "task-3", title: "Style 3", designId: "design-3", status: "published" },
    ],
  });

  expect(screen.getByText("任务已创建")).toBeInTheDocument();
  expect(screen.getByText("草稿已保存")).toBeInTheDocument();
  expect(screen.getByText("已发布")).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the targeted frontend tests to verify they fail**

Run: `npm test -- grouped-sds-create.test.ts shein-studio-workbench.test.tsx shein-studio-batches.test.ts`

Expected: FAIL because the models do not yet expose compatibility fingerprints or explicit task status.

### Task 5: Implement frontend compatibility-aware grouped mode and explicit task state rendering

**Files:**
- Modify: `web/listingkit-ui/src/lib/types/sds-baseline.ts`
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/grouped-image-mode.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/create-review-tasks.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-grouped-selection-panel.tsx`
- Test: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Extend the frontend types for explicit backend state**

Update `web/listingkit-ui/src/lib/types/sds-baseline.ts` and `web/listingkit-ui/src/lib/types/shein-studio.ts`:

```ts
export type GroupedSDSSelectionEligibility = {
  selectionId: string;
  selection: SDSProductVariantSelection;
  baselineKey?: string;
  baselineStatus: SDSBaselineStatus;
  baselineReason: string;
  baselineReasonCode?: string;
  sheinStoreId: string;
  eligible: boolean;
  eligibilityReason?: string;
  compatibilityFingerprint?: string;
};

export type SheinStudioCreatedTaskStatus =
  | "task_created"
  | "needs_review"
  | "ready_to_submit"
  | "draft_saved"
  | "published"
  | "submit_failed"
  | "unknown";

export type SheinStudioCreatedTask = {
  id: string;
  title: string;
  designId: string;
  itemId?: string;
  selectionId?: string;
  compatibilityFingerprint?: string;
  status?: SheinStudioCreatedTaskStatus;
  submissionState?: string;
  lastSubmissionAction?: string;
  reasonCode?: string;
  message?: string;
};
```

- [ ] **Step 2: Parse the new response fields from the batch API**

Update `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`:

```ts
createdTasks: (payload.created_tasks ?? []).map((task) => ({
  id: task.id,
  title: task.title,
  designId: task.design_id,
  itemId: task.item_id,
  selectionId: task.selection_id,
  compatibilityFingerprint: task.compatibility_fingerprint,
  status: normalizeCreatedTaskStatus(task.status),
  submissionState: typeof task.submission_state === "string" ? task.submission_state : undefined,
  lastSubmissionAction:
    typeof task.last_submission_action === "string" ? task.last_submission_action : undefined,
  reasonCode: typeof task.reason_code === "string" ? task.reason_code : undefined,
  message: typeof task.message === "string" ? task.message : undefined,
})),
```

- [ ] **Step 3: Make shared mode depend on fingerprint equivalence**

Update `web/listingkit-ui/src/lib/shein-studio/grouped-image-mode.ts` and `create-review-tasks.ts`:

```ts
export function buildSharedCompatibilityGroupKey(selection: SDSProductVariantSelection) {
  return [
    selection.parentProductId ?? selection.productId ?? 0,
    selection.prototypeGroupId ?? 0,
    selection.layerId ?? "",
    selection.designType ?? "material",
    selection.printableWidth ?? 0,
    selection.printableHeight ?? 0,
    selection.templateImageUrl ?? "",
    selection.maskImageUrl ?? "",
  ].join("|");
}

if (groupedImageMode === "shared_by_size") {
  const fingerprints = new Set(group.selections.map((item) => item.compatibilityFingerprint).filter(Boolean));
  if (fingerprints.size > 1) {
    throw new Error("Only selections with the same compatibility fingerprint can share one design.");
  }
}
```

- [ ] **Step 4: Render explicit task state in the workbench**

Update `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.ts` and `shein-studio-workbench.tsx`:

```ts
export function statusGroupLabel(key: string) {
  switch (key) {
    case "task_created":
      return "任务已创建";
    case "needs_review":
      return "待审核";
    case "ready_to_submit":
      return "待提交";
    case "draft_saved":
      return "草稿已保存";
    case "published":
      return "已发布";
    case "submission_failed":
      return "提交失败";
    default:
      return key;
  }
}
```

- [ ] **Step 5: Show grouped-mode ineligibility messaging**

Update `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-grouped-selection-panel.tsx`:

```tsx
{item.eligibilityReason ? (
  <p className="text-xs text-amber-700">{item.eligibilityReason}</p>
) : item.compatibilityFingerprint == null && groupedImageMode === "shared_by_size" ? (
  <p className="text-xs text-amber-700">当前商品缺少兼容指纹，不能参与共图。</p>
) : null}
```

- [ ] **Step 6: Run the targeted frontend tests to verify they pass**

Run: `npm test -- grouped-sds-create.test.ts shein-studio-workbench.test.tsx shein-studio-batches.test.ts`

Expected: PASS.

### Task 6: Run regression checks and commit the implementation

**Files:**
- Test: `internal/listingkit/...`
- Test: `web/listingkit-ui/...`
- Modify: implementation files from Tasks 1-5

- [ ] **Step 1: Run backend regression tests**

Run: `go test ./internal/listingkit -count=1`

Expected: PASS.

- [ ] **Step 2: Run frontend targeted regression tests**

Run: `npm test -- shein-studio-workbench.test.tsx shein-studio-grouped-selection-panel.test.tsx grouped-sds-create.test.ts shein-studio-batches.test.ts`

Expected: PASS.

- [ ] **Step 3: Run frontend type checks**

Run: `npm run typecheck`

Expected: PASS.

- [ ] **Step 4: Commit the implementation**

Run:

```bash
git add internal/listingkit web/listingkit-ui/src docs/superpowers/plans/2026-06-20-listingkit-sds-batch-task-gating-and-status.md
git commit -m "feat: tighten SDS batch task gating and status semantics"
```
