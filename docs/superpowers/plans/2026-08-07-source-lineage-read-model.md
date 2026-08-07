# Source Lineage Read Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with review checkpoints. Steps use checkbox syntax for tracking.

**Goal:** Expose normalized source identity through the existing ListingKit task-list API and workbench.

**Architecture:** Reuse listingkit.SourceReference in an optional TaskListDisplayFields.SourceReference read-model field. Populate it from Task.Request.Source with a defensive copy, preserve completed-task source_type behavior with a pending-task fallback, then extend the TypeScript/Zod contracts and existing task-card metadata UI.

**Tech Stack:** Go, GORM, Gin/JSON, Next.js, React, TypeScript, Zod, Vitest, Testing Library.

## Global Constraints

- No new table, migration, endpoint, source-specific query model, task filter, or state-machine branch.
- Expose only source key, type, platform, ID, and normalized URL; never raw crawler payloads or credentials.
- Keep the legacy public generate handler clearing caller-supplied Source.
- Keep completed-task source_type behavior unchanged; only add a pending-task fallback.
- Keep source_reference optional for legacy tasks and clients.
- Use TDD and stage only files in the current task.

---

### Task 1: Define the backend read-model contract

**Files:**

- Create: internal/listingkit/task_source_reference_projection_test.go
- Modify: internal/listingkit/model_task.go after the red test is observed

**Interfaces:**

- Consumes: Task, GenerateRequest, SourceReference, and buildTaskListItem.
- Produces: optional TaskListItem.SourceReference with no shared pointer to the persisted request.

- [ ] **Step 1: Add the failing sourced pending-task test**

Add a test in package listingkit that creates a pending task with Task.Request.Source equal to a complete crawler:1688:888 reference, calls buildTaskListItem, and asserts:

~~~go
item.SourceReference != nil
item.SourceReference != task.Request.Source
item.SourceReference.Key == "crawler:1688:888"
item.SourceReference.Type == "crawler"
item.SourceReference.Platform == "1688"
item.SourceReference.ID == "888"
item.SourceReference.URL == "https://detail.1688.com/offer/888.html"
item.SourceType == "crawler"
~~~

- [ ] **Step 2: Add the failing legacy omission and JSON test**

Create a task with Request: &GenerateRequest{Text: "legacy"}, assert item.SourceReference == nil, marshal the item with json.Marshal, and assert the JSON does not contain "source_reference".

- [ ] **Step 3: Run the red tests**

~~~powershell
go test ./internal/listingkit -run 'TestBuildTaskListItemIncludesSourceReferenceForPendingTask|TestBuildTaskListItemOmitsLegacySourceReference' -count=1
~~~

Expected: compilation failure because TaskListDisplayFields.SourceReference does not exist.

- [ ] **Step 4: Add the model field and commit the contract**

After observing the expected failure, add this field to TaskListDisplayFields with JSON tag source_reference and omitempty:

~~~go
SourceReference *SourceReference
~~~

Do not populate it yet; the projection test must still fail on its assertion. Commit:

~~~powershell
git add internal/listingkit/task_source_reference_projection_test.go internal/listingkit/model_task.go
git commit -m "test: define source lineage read model"
~~~

### Task 2: Populate the Go task-list projection

**Files:**

- Modify: internal/listingkit/task_list_item_support.go
- Test: internal/listingkit/task_source_reference_projection_test.go

**Interfaces:**

- Consumes: Task.Request.Source and TaskListDisplayFields.SourceReference.
- Produces: defensive source-reference projection and pending-task source_type fallback.

- [ ] **Step 1: Implement the minimal projection**

In applyTaskListRequestFields, add before the existing SDS/store mapping:

~~~go
if source := task.Request.Source; source != nil {
    item.SourceReference = &SourceReference{
        Key: source.Key, Type: source.Type, Platform: source.Platform,
        ID: source.ID, URL: source.URL,
    }
    if item.SourceType == "" {
        item.SourceType = strings.TrimSpace(source.Type)
    }
}
~~~

The existing result-summary assignment remains authoritative for completed tasks.

- [ ] **Step 2: Run green projection tests**

~~~powershell
gofmt -w internal/listingkit/model_task.go internal/listingkit/task_list_item_support.go internal/listingkit/task_source_reference_projection_test.go
go test ./internal/listingkit -run 'TestBuildTaskListItemIncludesSourceReferenceForPendingTask|TestBuildTaskListItemOmitsLegacySourceReference' -count=1
~~~

- [ ] **Step 3: Run regressions and commit**

~~~powershell
go test ./internal/listingkit -run 'TestBuildTaskListItem|TestListTasks|TestTaskList' -count=1
git add internal/listingkit/model_task.go internal/listingkit/task_list_item_support.go internal/listingkit/task_source_reference_projection_test.go
git commit -m "feat: expose source lineage in task list items"
~~~

### Task 3: Extend the UI contract and render source metadata

**Files:**

- Modify: web/listingkit-ui/src/lib/types/listingkit/tasks.ts
- Modify: web/listingkit-ui/src/lib/api/task-list-schema.ts
- Modify: web/listingkit-ui/src/components/listingkit/tasks/task-list-page-sections.tsx
- Modify: web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.tsx
- Modify: web/listingkit-ui/src/lib/api/task-list-schema.test.ts
- Modify: web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.test.tsx

**Interfaces:**

- Consumes: optional API object source_reference with five optional string fields.
- Produces: typed, schema-validated source metadata, an external source link in the task-list row, and plain source summary text in the home card without changing task navigation.

- [ ] **Step 1: Add the failing schema test**

Extend task-list-schema.test.ts with a complete source_reference object and assert all five fields survive parseTaskListResponse.

- [ ] **Step 2: Add the failing card test**

Extend listingkit-home-task-card.test.tsx with the same source reference. Assert the card renders 1688 and 888 as source summary text, and does not create a second source link inside the workspace link. Keep the workspace link assertion unchanged.

- [ ] **Step 3: Run the UI red tests**

From web/listingkit-ui:

~~~powershell
npm.cmd test -- --run src/lib/api/task-list-schema.test.ts src/components/listingkit/home/listingkit-home-task-card.test.tsx
~~~

Expected: the new source-link rendering assertion fails before implementation.

- [ ] **Step 4: Add the UI type and Zod shape**

Add ListingKitSourceReference with optional key, type, platform, id, and url strings to tasks.ts; add source_reference?: ListingKitSourceReference to ListingKitTaskListItem; and add the same optional nested object with passthrough to taskListItemSchema.

- [ ] **Step 5: Render source metadata**

In the existing task row, render metadata only when the reference has a non-empty platform, ID, or URL. Show platform/ID as text and render the URL as an accessible external link with target="_blank" and rel="noreferrer". In the home task card, show platform/ID as plain text only because the card root is already the workspace link; do not create nested anchors or empty anchors.

- [ ] **Step 6: Run UI green checks and commit**

~~~powershell
npm.cmd test -- --run src/lib/api/task-list-schema.test.ts src/components/listingkit/home/listingkit-home-task-card.test.tsx
npm.cmd run typecheck
npm.cmd run lint -- --file src/lib/api/task-list-schema.ts --file src/components/listingkit/home/listingkit-home-task-card.tsx --file src/components/listingkit/tasks/task-list-page-sections.tsx
git add web/listingkit-ui/src/lib/types/listingkit/tasks.ts web/listingkit-ui/src/lib/api/task-list-schema.ts web/listingkit-ui/src/lib/api/task-list-schema.test.ts web/listingkit-ui/src/components/listingkit/tasks/task-list-page-sections.tsx web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.tsx web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.test.tsx
git commit -m "feat: show source lineage in listingkit workbench"
~~~

### Task 4: Run the full regression gate

**Files:**

- Test: all files from Tasks 1–3

**Interfaces:**

- Consumes: the backend and UI read-model contracts.
- Produces: evidence that sourced tasks are visible without changing legacy task, authorization, or publishing behavior.

- [ ] **Step 1: Run focused Go tests**

~~~powershell
go test ./internal/listingkit/... ./internal/product/sourcehandoff/a1688/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
~~~

- [ ] **Step 2: Run focused UI tests**

~~~powershell
Set-Location web/listingkit-ui
npm.cmd test -- --run src/lib/api/task-list-schema.test.ts src/components/listingkit/home/listingkit-home-task-card.test.tsx src/components/listingkit/tasks/task-list-page.test.tsx
Set-Location ../..
~~~

- [ ] **Step 3: Run repository checks**

~~~powershell
go test ./... -count=1
Set-Location web/listingkit-ui
npm.cmd test
npm.cmd run typecheck
npm.cmd run lint
Set-Location ../..
~~~

Record concrete failures separately from timeouts.

- [ ] **Step 4: Review final state**

~~~powershell
git diff --check
git status --short --branch
git log --oneline -5
~~~

Expected: only the scoped source-lineage read-model spec, plan, backend, and UI changes are present.
