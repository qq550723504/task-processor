# ListingKit SDS Batch Open Queue Design

## Goal

Add safe homepage-level bulk actions to `/listing-kits/sds` without introducing a second execution engine:

- `批量继续生成`
- `批量创建任务`

Both actions should reuse the existing single-batch Shein Studio workbench. The homepage only prepares a queue of selected batches, then the workbench processes them one by one.

## Why This Approach

The current product model is already stable around:

- one independent `batch`
- one active workbench editing context
- one set of prompt / designs / store assignments per batch

Trying to execute generation or task creation directly from the homepage would create a second orchestration path and duplicate validation, error handling, and progress behavior. A batch-open queue avoids that. It keeps the homepage as a launcher and keeps the workbench as the only execution surface.

## User Experience

### Homepage

The recent-batches dashboard keeps the existing multi-select behavior. When one or more persisted batches are selected, the toolbar adds two new actions:

- `批量继续生成`
- `批量创建任务`

These actions only apply to persisted `batch` records. Recoverable local drafts are not included.

### Entering Queue Mode

When the user clicks one of the two bulk actions:

1. collect the selected batch ids in current selection order
2. enter a temporary queue mode in the workbench page
3. immediately load the first batch into the existing editor

Queue mode is not a new page. It is a thin wrapper around the current workbench.

### Queue Banner

When queue mode is active, show a compact banner above the workbench:

- current queue purpose: `批量继续生成` or `批量创建任务`
- progress: `第 1 / 3 个批次`
- current batch name
- controls:
  - `下一批次`
  - `跳过`
  - `退出批量处理`

### Continue Generate Mode

`批量继续生成` should:

- load the first selected batch
- focus the workbench on the `generate` step
- let the user review or adjust prompt, grouped selections, and design settings
- after the user finishes with the current batch, they can click `下一批次`

This mode does not auto-trigger generation in phase 1. It is a batch navigation accelerator, not a background runner.

### Create Tasks Mode

`批量创建任务` should:

- load the first selected batch
- focus the workbench on the `review` or `tasks` step depending on batch state
- preserve all existing task creation validation
- let the user create tasks from the current batch, then move to the next one

This mode also does not silently create tasks from the homepage.

## State Model

Add a lightweight queue state inside the existing workbench state:

- `batchQueueMode?: "generate" | "create_tasks"`
- `queuedBatchIds: string[]`
- `queuedBatchIndex: number`

Derived helpers:

- current queued batch id
- queue progress label
- whether queue mode is active

This queue state is UI/session state only. It does not need to be persisted to server batch records in phase 1.

## Data Flow

### Homepage to Workbench

The recent-batches dashboard emits:

- selected batch ids
- selected queue mode

The workbench then:

1. validates the selected ids still exist in `savedBatches`
2. initializes queue state
3. calls the existing `handleLoadBatch()` for the first batch
4. sets the effective step based on queue mode

### Moving Between Batches

`下一批次`:

- increments `queuedBatchIndex`
- loads the next batch with `handleLoadBatch()`
- keeps queue mode active

`跳过`:

- same navigation behavior as `下一批次`
- no extra mutation to the current batch

`退出批量处理`:

- clears queue state
- leaves the current batch loaded in the editor

When the queue reaches the end:

- clear queue state
- show a success message like `已完成这批批次的顺序处理。`

## Error Handling

### Missing Batch

If a queued batch id no longer exists:

- skip it automatically
- continue to the next batch
- append a lightweight warning message

### Invalid Batch State

If a batch is missing data needed for the target action:

- still load it
- keep existing inline validation in the workbench
- do not auto-skip

This is important because the user may want to repair the batch before continuing.

## Testing

Add focused coverage for:

- multi-select homepage actions appear when persisted batches are selected
- starting `批量继续生成` loads the first batch and enables queue banner
- clicking `下一批次` loads the next batch
- starting `批量创建任务` enters queue mode with the expected target step
- exiting queue mode clears queue banner and queue state
- deleted or missing batch ids are skipped safely

## Scope Boundaries

Included in this spec:

- homepage batch queue launcher
- queue banner in current workbench
- sequential batch opening for generate/task workflows

Not included in this spec:

- background bulk generation
- homepage-side direct task creation
- queue persistence across refresh
- per-batch queue result audit trail

## Recommendation

Implement this as a small extension of the existing recent-batches homepage and single-batch workbench, not as a new batch processor. That keeps the product mental model simple:

- homepage selects the work
- workbench does the work
- queue mode only helps the user move through many batches faster
