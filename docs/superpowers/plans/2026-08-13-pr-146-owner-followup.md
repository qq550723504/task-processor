# PR #146 Owner Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the three PR #146 P2 defects by rejecting missing mapping owners before request creation, propagating the store owner through every live SHEIN repair path, and removing ownerless dead builder APIs.

**Architecture:** Validate the canonical owner at the SHEIN and TEMU publish-input boundaries, then enforce the same invariant in `MappingBuilder` before any gateway call. Live repair strategies copy ownership from `StoreInfo`; unused convenience methods that have no ownership source are deleted instead of expanded.

**Tech Stack:** Go, standard library `strings`, existing ListingKit runtime/domain types, Go `testing`, Logrus output capture.

## Global Constraints

- Do not reopen or rewrite PR #146.
- Do not backfill database rows, deploy, reply to review comments, or resolve GitHub threads.
- Do not synthesize fallback owners; missing or whitespace-only owners fail closed.
- Introduce no new dependency.
- Preserve TEMU post-processing semantics: invalid input is logged and skipped without failing the main publish flow.
- Each behavioral change follows RED, GREEN, REFACTOR and keeps the gateway untouched for invalid input.

---

### Task 1: Reject ownerless SHEIN mapping input

**Files:**
- Create: `internal/shein/publish/publish_input_test.go`
- Modify: `internal/shein/publish/publish_input.go:3-7,114-130`

**Interfaces:**
- Consumes: `buildMappingRequestInput(*shein.TaskContext) (*MappingRequestInput, error)`.
- Produces: a mapping input only when `StoreInfo` exists and its trimmed `OwnerUserID` is non-empty.

- [ ] **Step 1: Write the failing boundary tests**

Create this test file:

```go
package publish

import (
	"strings"
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

func TestBuildMappingRequestInputRejectsMissingStoreOwner(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		store *listingruntime.StoreInfo
		want string
	}{
		{name: "missing store", want: "store info is not initialized"},
		{name: "blank owner", store: &listingruntime.StoreInfo{OwnerUserID: "  "}, want: "store owner is not initialized"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildMappingRequestInput(&shein.TaskContext{Task: &model.Task{ID: 1}, StoreInfo: tt.store})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildMappingRequestInputPreservesStoreOwner(t *testing.T) {
	t.Parallel()
	input, err := buildMappingRequestInput(&shein.TaskContext{
		Task: &model.Task{ID: 1},
		StoreInfo: &listingruntime.StoreInfo{OwnerUserID: "zitadel-sub-1"},
	})
	if err != nil {
		t.Fatalf("buildMappingRequestInput() error = %v", err)
	}
	if input.StoreInfo == nil || input.StoreInfo.OwnerUserID != "zitadel-sub-1" {
		t.Fatalf("input.StoreInfo = %+v, want owner zitadel-sub-1", input.StoreInfo)
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/shein/publish -run 'TestBuildMappingRequestInput' -count=1`

Expected: the rejection test fails because the current builder returns a nil error.

- [ ] **Step 3: Implement the minimum boundary validation**

Add `strings` to imports. In `buildMappingRequestInput`, after the task check and before the return, insert:

```go
storeInfo := publishRuntimeStoreInfo(ctx.StoreInfo)
if storeInfo == nil {
	return nil, fmt.Errorf("store info is not initialized")
}
if strings.TrimSpace(storeInfo.OwnerUserID) == "" {
	return nil, fmt.Errorf("store owner is not initialized")
}
```

Set the returned struct's existing `StoreInfo` field to `storeInfo` instead of calling `publishRuntimeStoreInfo` again.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/shein/publish -run 'TestBuildMappingRequestInput' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add -- internal/shein/publish/publish_input.go internal/shein/publish/publish_input_test.go
git diff --cached --check
git commit -m "fix: reject ownerless SHEIN mapping input"
```

---

### Task 2: Reject and expose ownerless TEMU mapping input

**Files:**
- Create: `internal/temu/product/publish_result_input_test.go`
- Modify: `internal/temu/product/publish_result_input.go:284-318`
- Modify: `internal/temu/product/save_publish_result_handler.go:48-56`

**Interfaces:**
- Consumes: `buildSavePublishResultInput(*temucontext.TemuTaskContext)` and `SavePublishResultHandler.HandleTemu`.
- Produces: an input only for a non-nil store with a non-blank owner; the handler logs the precise validation error and returns nil.

- [ ] **Step 1: Write the failing input tests**

Create the file with this helper and table:

```go
package product

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

func newPublishResultContext(store *listingadmin.StoreRespDTO) *temucontext.TemuTaskContext {
	ctx := temucontext.NewTemuTaskContext(context.Background(), &model.Task{ID: 1, TenantID: 2, StoreID: 3})
	ctx.SetSubmitResponse(&temuapi.SubmitResponse{Success: true})
	ctx.StoreInfo = store
	return ctx
}

func TestBuildSavePublishResultInputRejectsMissingStoreOwner(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		store *listingadmin.StoreRespDTO
		want string
	}{
		{name: "missing store", want: "store info is not initialized"},
		{name: "blank owner", store: &listingadmin.StoreRespDTO{OwnerUserID: "  "}, want: "store owner is not initialized"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildSavePublishResultInput(newPublishResultContext(tt.store))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/temu/product -run 'TestBuildSavePublishResultInputRejectsMissingStoreOwner' -count=1`

Expected: FAIL because both invalid stores are accepted.

- [ ] **Step 3: Implement the minimum input validation**

After the task check and before reading the submit response, add:

```go
if temuCtx.StoreInfo == nil {
	return nil, fmt.Errorf("store info is not initialized")
}
if strings.TrimSpace(temuCtx.StoreInfo.OwnerUserID) == "" {
	return nil, fmt.Errorf("store owner is not initialized")
}
```

The file already imports `strings`.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/temu/product -run 'TestBuildSavePublishResultInputRejectsMissingStoreOwner' -count=1`

Expected: PASS.

- [ ] **Step 5: Write the failing log test**

Append:

```go
func TestSavePublishResultHandlerLogsInputValidationError(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	log := logrus.New()
	log.SetOutput(&output)
	handler := NewSavePublishResultHandler(nil, nil)
	handler.logger = logrus.NewEntry(log)
	if err := handler.HandleTemu(newPublishResultContext(nil)); err != nil {
		t.Fatalf("HandleTemu() error = %v", err)
	}
	if !strings.Contains(output.String(), "store info is not initialized") {
		t.Fatalf("log output = %q, want validation error", output.String())
	}
}
```

- [ ] **Step 6: Verify RED**

Run: `go test ./internal/temu/product -run 'TestSavePublishResultHandlerLogsInputValidationError' -count=1`

Expected: FAIL because the warning omits `err`.

- [ ] **Step 7: Attach the error to the existing warning**

Replace the error branch with:

```go
if err != nil {
	h.logger.WithError(err).Warn("TEMU发布结果输入无效，跳过保存")
	return nil
}
```

- [ ] **Step 8: Verify the package and commit**

Run: `go test ./internal/temu/product -count=1`

Expected: PASS.

```powershell
git add -- internal/temu/product/publish_result_input.go internal/temu/product/publish_result_input_test.go internal/temu/product/save_publish_result_handler.go
git diff --cached --check
git commit -m "fix: reject ownerless TEMU mapping input"
```

---

### Task 3: Enforce owner propagation in the SHEIN mapping builder

**Files:**
- Create: `internal/shein/mapping/builder_owner_test.go`
- Modify: `internal/shein/mapping/builder.go:3-10,146-260`
- Modify: `internal/shein/mapping/strategies.go:205-220`

**Interfaces:**
- Consumes: `MappingBuilder.CreateMappingRelation`, `NewSmartRepairStrategy`, and `runtimeMappingGateway`.
- Produces: no gateway call for a blank owner; SmartRepair sends the store owner in its mapping upsert.

- [ ] **Step 1: Add the gateway fake and failing owner test**

Create:

```go
package mapping

import (
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingruntime"
)

type ownerCapturingMappingGateway struct {
	requests []*listingruntime.ProductImportMappingUpsert
}

func (g *ownerCapturingMappingGateway) CreateMapping(req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	g.requests = append(g.requests, req)
	return 41, nil
}

func (g *ownerCapturingMappingGateway) FindMappingByPlatformProductID(string, int64) (*listingruntime.ProductImportMapping, error) {
	return &listingruntime.ProductImportMapping{ID: 41}, nil
}

func TestMappingBuilderRejectsBlankOwnerBeforeGateway(t *testing.T) {
	t.Parallel()
	gateway := &ownerCapturingMappingGateway{}
	_, err := NewMappingBuilder(gateway).CreateMappingRelation(&MappingCreateOptions{
		TenantID: 1, StoreID: 2, OwnerUserID: "  ", SkuCode: "SKU-1", Region: "US",
	})
	if err == nil || !strings.Contains(err.Error(), "映射所有者不能为空") {
		t.Fatalf("error = %v, want owner validation error", err)
	}
	if len(gateway.requests) != 0 {
		t.Fatalf("gateway calls = %d, want 0", len(gateway.requests))
	}
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/shein/mapping -run 'TestMappingBuilderRejectsBlankOwnerBeforeGateway' -count=1`

Expected: FAIL because the gateway is called.

- [ ] **Step 3: Add the builder invariant**

Import `strings`, then add after tenant/store validation:

```go
if strings.TrimSpace(options.OwnerUserID) == "" {
	return fmt.Errorf("映射所有者不能为空")
}
```

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/shein/mapping -run 'TestMappingBuilderRejectsBlankOwnerBeforeGateway' -count=1`

Expected: PASS.

- [ ] **Step 5: Write the failing SmartRepair propagation test**

Append:

```go
func TestSmartRepairStrategyPropagatesStoreOwner(t *testing.T) {
	t.Parallel()
	gateway := &ownerCapturingMappingGateway{}
	strategy := NewSmartRepairStrategy(gateway, nil, nil, nil)
	result, err := strategy.Repair(&MappingRepairContext{
		Request: &MappingRepairRequest{TenantID: 1, StoreID: 2, SkuCode: "SKU-1", Reason: "repair"},
		StoreInfo: &listingruntime.StoreInfo{OwnerUserID: "zitadel-sub-1", Region: "US"},
		StartTime: time.Now(),
	})
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("result = %+v, want success", result)
	}
	if len(gateway.requests) != 1 || gateway.requests[0].OwnerUserID != "zitadel-sub-1" {
		t.Fatalf("requests = %+v, want owner zitadel-sub-1", gateway.requests)
	}
}
```

- [ ] **Step 6: Verify RED**

Run: `go test ./internal/shein/mapping -run 'TestSmartRepairStrategyPropagatesStoreOwner' -count=1`

Expected: FAIL because `buildEnhancedMappingOptions` omits the owner.

- [ ] **Step 7: Propagate the SmartRepair owner**

Add this field to the existing `MappingCreateOptions` literal in `buildEnhancedMappingOptions`:

```go
OwnerUserID: ownerUserIDFromStore(ctx.StoreInfo),
```

- [ ] **Step 8: Verify GREEN**

Run: `go test ./internal/shein/mapping -run 'TestSmartRepairStrategyPropagatesStoreOwner' -count=1`

Expected: PASS.

- [ ] **Step 9: Remove the dead ownerless methods**

Delete `CreateBasicMapping`, `CreateMappingWithSPU`, `CreateMappingWithPrice`, `CreateMappingWithRules`, and `CreateMappingFromTaskContext`. Then run:

`rg -n 'CreateBasicMapping|CreateMappingWithSPU|CreateMappingWithPrice|CreateMappingWithRules|CreateMappingFromTaskContext' --glob '*.go'`

Expected: no matches and exit code 1.

- [ ] **Step 10: Verify the package and commit**

Run: `go test ./internal/shein/mapping -count=1`

Expected: PASS.

```powershell
git add -- internal/shein/mapping/builder.go internal/shein/mapping/builder_owner_test.go internal/shein/mapping/strategies.go
git diff --cached --check
git commit -m "fix: require owners in SHEIN mapping builder"
```

---

### Task 4: Verify the complete follow-up

**Files:**
- Verify: all changes after design commit `570c98893`.

**Interfaces:**
- Consumes: Tasks 1-3.
- Produces: targeted, full-suite, formatting, scope, and thread-state evidence.

- [ ] **Step 1: Format all changed Go files**

```powershell
gofmt -w internal/shein/publish/publish_input.go internal/shein/publish/publish_input_test.go internal/temu/product/publish_result_input.go internal/temu/product/publish_result_input_test.go internal/temu/product/save_publish_result_handler.go internal/shein/mapping/builder.go internal/shein/mapping/builder_owner_test.go internal/shein/mapping/strategies.go
```

- [ ] **Step 2: Run targeted regression tests**

Run: `go test ./internal/shein/publish ./internal/temu/product ./internal/shein/mapping -count=1`

Expected: all three packages PASS.

- [ ] **Step 3: Run the full repository suite**

Run: `go test ./... -count=1`

Expected: PASS. Use a timeout of at least 15 minutes; the clean baseline took about 184 seconds.

- [ ] **Step 4: Verify hygiene and scope**

```powershell
git diff --check 570c98893..HEAD
git status --short
git diff --stat 570c98893..HEAD
rg -n 'CreateBasicMapping|CreateMappingWithSPU|CreateMappingWithPrice|CreateMappingWithRules|CreateMappingFromTaskContext' --glob '*.go'
```

Expected: no whitespace errors, a clean worktree, only approved files changed, and no removed-method matches.

- [ ] **Step 5: Re-read PR #146 thread state without mutation**

Use the thread-aware GitHub read and confirm the same three P2 threads are still unresolved. Report code/test evidence and request explicit authorization before replying to or resolving them.
