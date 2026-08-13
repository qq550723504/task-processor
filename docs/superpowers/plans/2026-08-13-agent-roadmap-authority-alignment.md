# AI Commerce Agent Roadmap Authority Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the repository authority documents and GitHub backlog so the minimum Commerce Tool foundation precedes Product Agent implementation, without changing business code or overstating runtime acceptance.

**Architecture:** Preserve the strategy document as long-term direction, the current-refactoring-status document as the active execution gate, and GitHub issue #137 as the backlog mapping. Reclassify the completed asset plan as historical execution evidence, then update existing issues only after the documentation change is merged to `master`.

**Tech Stack:** Markdown, Git, Go documentation/architecture guard tests, GitHub Issues and Draft Pull Requests.

## Global Constraints

- Do not modify Go, TypeScript, Kubernetes, workflow, provider, marketplace, persistence, or runtime behavior.
- Do not create a second Tool contract, Agent state model, submission state machine, scheduler, or workflow owner.
- Preserve issue numbers #125, #127, #128, #131, #132, #133, #134, and #137.
- Treat repository implementation, repository validation, controlled-runtime acceptance, and production acceptance as separate evidence levels.
- Do not mark a commercial or production checklist complete without its exact recorded evidence.
- Do not mutate GitHub issue titles, labels, bodies, or checkboxes until the documentation PR is merged to `master`.
- Opening a Draft PR is authorized by the approved design; merging it remains a separate explicit authorization.
- Preserve unrelated work and stage only paths named by each task.

---

## Execution Status (2026-08-13)

- Task 1 complete in `f726f4dd5`: strategy order is Phase 2A Tool Foundation, Phase 2B Product Agent, then Phase 3 Tool expansion and hardening.
- Task 2 complete in `bede51542`: the active current-state authority is calibrated to `49815aad9` and separates merged 1688/TaskEvent V2 assets from controlled and production acceptance.
- Task 3 complete in `0d68d5b31`: the completed platform-aware asset plan is explicitly historical evidence.
- Task 4 verification passed: placeholder and phase-order scans returned no errors, `go test ./tests -count=1` passed, `git diff --check origin/master...HEAD` passed, and exactly the five approved documentation paths changed.
- Task 5 complete: the verified branch was pushed and Draft PR #143 was opened against `master`; no merge or issue mutation was performed.
- Task 6 remains blocked by design until the documentation PR is explicitly authorized and confirmed merged.

### Task 1: Align the long-term strategy phase model

**Files:**
- Modify: `docs/product/ai-commerce-agent-platform-strategy.md:612-806`
- Reference: `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`

**Interfaces:**
- Consumes: the approved Phase 0, Phase 1, Phase 2A, Phase 2B, Phase 3, and Phase 4 ordering from the design.
- Produces: the canonical long-term phase model consumed by the current-state document and issue #137.

- [x] **Step 1: Capture the superseded phase order**

Run:

```powershell
rg -n '^### Phase|^### Now|^### Next|^### Later' docs/product/ai-commerce-agent-platform-strategy.md
```

Expected: Phase 2 is Product Agent PoC and Phase 3 is Commerce Tool Contract, proving the dependency is reversed before the edit.

- [x] **Step 2: Replace the Phase 2 and Phase 3 roadmap sections**

Replace the old sections with this order and meaning:

```markdown
### Phase 2A：最小 Commerce Tool Foundation

在 Product Agent 之前建立一套最小、稳定、框架无关的 Tool 合同：

- ToolDefinition / version / capability；
- input/output schema；
- read / compute / propose 风险等级；
- tenant/user permission；
- timeout / retry owner；
- deterministic error taxonomy；
- audit / trace metadata；
- AgentDefinition allowlist 绑定。

第一批只接入 Product Agent 所需的 source evidence、canonical product、
catalog/asset facts、ProductEnrich proposal、ProductImage analysis proposal、
marketplace rule lookup 和 deterministic validator。不得开放 write 或 publish，
不得让 Agent 直接访问 repository、provider SDK 或 marketplace client。

### Phase 2B：Product Agent PoC

在 Phase 1 与 Phase 2A 达到退出门禁后，实现“商品资料诊断与补全 Agent”。

门禁：

- 只通过 Phase 2A Tool Registry 获取 read / compute / propose tools；
- 最多有限修复循环；
- 所有输出进入人工审核；
- feature flag 与 tenant allowlist；
- step、model call、token、runtime 和 cost hard budget；
- 与固定流程做离线 A/B eval。

只有证明质量或人工效率有可量化提升且风险指标不恶化后，才继续扩大 Agent 使用面。

### Phase 3：Commerce Tool 扩展与生产级治理

Phase 3 不再创建第一套 Tool 合同，而是在 Product Agent PoC 证明价值后扩展
Phase 2A 的同一合同，包括更多领域工具、版本迁移策略、运行 SLO 和平台适配器。
write / publish tools 仍需独立的审批、幂等、权限与审计门禁。
```

- [x] **Step 3: Align Now, Next, and Later**

Keep Now focused on SHEIN stability, current-baseline validation, controlled 1688 closure, and AI Capability stabilization. Set Next to this exact order:

```markdown
1. 最小 Commerce Tool Foundation；
2. Product Agent 有界运行时；
3. Product Agent PoC 与 fixed pipeline 对照评测；
4. Commerce Tool 扩展与生产级治理；
5. SHEIN Listing Agent read/propose-only 版本；
6. Agent Workspace 最小入口。
```

Keep Sourcing Agent, controlled write tools, additional marketplace Agents, and multi-Agent handoff in Later.

- [x] **Step 4: Align the documented authority order**

Keep the strategy first and current-refactoring-status second. Add issue #137 immediately after them as the executable backlog mapping. Move the detailed Agent platform design after issue #137. State that completed implementation plans are historical evidence and cannot override the current-state document.

- [x] **Step 5: Verify the strategy ordering**

Run:

```powershell
$path = 'docs/product/ai-commerce-agent-platform-strategy.md'
$text = Get-Content -Raw $path
$p2a = $text.IndexOf('### Phase 2A')
$p2b = $text.IndexOf('### Phase 2B')
$p3 = $text.IndexOf('### Phase 3')
if ($p2a -lt 0 -or $p2b -le $p2a -or $p3 -le $p2b) { throw 'Phase order must be 2A, 2B, 3' }
rg -n 'Product Agent PoC|Commerce Tool|current-refactoring-status|issue #137' $path
```

Expected: Phase 2A appears before Phase 2B, Phase 2B before Phase 3, and issue #137 appears in the authority section.

- [x] **Step 6: Commit the strategy slice**

```powershell
git add -- docs/product/ai-commerce-agent-platform-strategy.md
git diff --cached --check
git commit -m "docs: order tool foundation before product agent"
```

### Task 2: Recalibrate the active current-state authority

**Files:**
- Modify: `docs/refactoring/current-refactoring-status.md:1-300`
- Reference: `docs/product/ai-commerce-agent-platform-strategy.md`
- Reference: `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`

**Interfaces:**
- Consumes: repository baseline `49815aad922d98bc4c7d12b90dcabf86b541df2a` and the Phase 2A/2B order from Task 1.
- Produces: the active Now/Next/Later execution gate consumed by issue #137.

- [x] **Step 1: Reproduce the stale calibration**

Run:

```powershell
Select-String -Path docs/refactoring/current-refactoring-status.md -Pattern 'Last reviewed|Calibrated against|Select one next product source|Source of truth summary'
```

Expected: the file names 2026-07-13, commit `5c72f406`, and next-source selection as the next growth action.

- [x] **Step 2: Update metadata and current posture**

Set:

```markdown
> Last reviewed: 2026-08-13.
>
> Calibrated against: `master` at `49815aad922d98bc4c7d12b90dcabf86b541df2a`.
```

Replace the opening posture with:

```text
Stabilize and validate the current SHEIN production path first;
complete controlled 1688 source-to-ListingKit acceptance second;
stabilize the AI Capability Platform and trusted identity/evaluation gates third;
establish the minimum Commerce Tool Foundation before Product Agent code;
defer new product sources and target workbenches until their explicit gates are met.
```

- [x] **Step 3: Record current merged-versus-accepted reality**

Update the Product Sourcing section to say the guarded operator tool from PR #141 is merged, while a live tenant-owned 1688 account run, durable lineage evidence, and the complete preview/readiness result remain open.

Add a TaskEvent V2 operational paragraph stating that repository observability and canary tooling from PR #142 are merged, while monitoring installation, production canary/full rollout, and the 14-day zero-legacy observation gate remain open. Do not call the legacy decoder removable.

- [x] **Step 4: Replace the Next section**

Replace next-source selection with these subsections:

```markdown
### 4.1 Complete Phase 0 evidence
### 4.2 Stabilize the AI Capability Platform
### 4.3 Establish Phase 2A Commerce Tool Foundation
### 4.4 Start Phase 2B Product Agent only after its gates
### 4.5 Improve operational evidence
```

Move the next warehouse/catalog source, with 大建云仓 as a candidate rather than a commitment, to Later under a Sourcing Agent/product-source heading.

- [x] **Step 5: Replace the source-of-truth summary**

Use this order:

```markdown
1. `docs/product/ai-commerce-agent-platform-strategy.md` for long-term product direction and phase definitions.
2. `current-refactoring-status.md` for current repository reality and Now / Next / Later gates.
3. GitHub issue #137 for executable backlog order.
4. Domain-specific plans and dated validation notes for bounded acceptance evidence.
5. `next-phase-plan.md` and other completed implementation plans as historical execution evidence only.
```

- [x] **Step 6: Verify evidence language and ordering**

Run:

```powershell
$path = 'docs/refactoring/current-refactoring-status.md'
rg -n '49815aad|PR #141|PR #142|controlled|production|14-day|Phase 2A|Phase 2B|大建云仓|issue #137|historical' $path
if (Select-String -Path $path -Pattern 'preferred next growth direction is one warehouse') { throw 'Next-source priority is still active' }
```

Expected: merged tooling and runtime acceptance remain separate, Phase 2A precedes Phase 2B, and the old preferred-next-source statement is absent.

- [x] **Step 7: Commit the current-state slice**

```powershell
git add -- docs/refactoring/current-refactoring-status.md
git diff --cached --check
git commit -m "docs: recalibrate current roadmap authority"
```

### Task 3: Reclassify the completed asset plan as historical evidence

**Files:**
- Modify: `docs/refactoring/next-phase-plan.md:1-30`
- Reference: `docs/refactoring/current-refactoring-status.md`

**Interfaces:**
- Consumes: the authority order from Task 2.
- Produces: an immutable completed-plan record that no longer claims to be the live queue.

- [x] **Step 1: Capture the misleading active-plan wording**

Run:

```powershell
Get-Content docs/refactoring/next-phase-plan.md -TotalCount 32
```

Expected: the title and agentic-worker instruction still present the file as an implementation plan even though its status says implementation complete.

- [x] **Step 2: Add an explicit historical-authority banner**

Immediately below the title, add:

```markdown
> **Authority:** Historical implementation record. The platform-aware asset refactor is complete. This file no longer defines the active execution queue; use `docs/refactoring/current-refactoring-status.md` and GitHub issue #137 for current order.
```

Change the agentic-worker instruction to state that the checklist is retained as execution evidence and must not be re-executed as a current plan. Preserve all task content and completion evidence below the banner.

- [x] **Step 3: Verify no implementation evidence was removed**

Run:

```powershell
rg -n 'Historical implementation record|PR #138|TaskEventV2|Definition of done|Explicitly deferred' docs/refactoring/next-phase-plan.md
git diff -- docs/refactoring/next-phase-plan.md
```

Expected: the authority banner is present and the completed tasks, commits, gates, and deferred scope remain in the file.

- [x] **Step 4: Commit the historical-plan slice**

```powershell
git add -- docs/refactoring/next-phase-plan.md
git diff --cached --check
git commit -m "docs: retire completed next-phase plan authority"
```

### Task 4: Validate the complete documentation change

**Files:**
- Modify: `docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md` only to mark completed steps and record actual evidence.
- Test: `docs/product/ai-commerce-agent-platform-strategy.md`
- Test: `docs/refactoring/current-refactoring-status.md`
- Test: `docs/refactoring/next-phase-plan.md`
- Test: `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`

**Interfaces:**
- Consumes: Tasks 1-3.
- Produces: one internally consistent documentation-only branch ready for review.

- [x] **Step 1: Run the placeholder and ambiguity scan**

Run:

```powershell
$paths = @(
  'docs/product/ai-commerce-agent-platform-strategy.md',
  'docs/refactoring/current-refactoring-status.md',
  'docs/refactoring/next-phase-plan.md',
  'docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md',
  'docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md'
)
$hits = rg -n 'T[B]D|T[O]DO|implement l[a]ter|fill in d[e]tails|handle appropriatel[y]' $paths
if ($LASTEXITCODE -eq 0) { $hits; throw 'Documentation placeholder scan found matches' }
```

Expected: no matches.

- [x] **Step 2: Verify phase order across active authority documents**

Run:

```powershell
foreach ($path in @('docs/product/ai-commerce-agent-platform-strategy.md','docs/refactoring/current-refactoring-status.md')) {
  $text = Get-Content -Raw $path
  $p2a = $text.IndexOf('Phase 2A')
  $p2b = $text.IndexOf('Phase 2B')
  if ($p2a -lt 0 -or $p2b -le $p2a) { throw "$path does not order Phase 2A before Phase 2B" }
}
```

Expected: both active documents order Phase 2A before Phase 2B.

- [x] **Step 3: Run repository documentation and architecture guards**

Run:

```powershell
go test ./tests -count=1
git diff --check origin/master...HEAD
```

Expected: tests pass and no whitespace errors are reported.

- [x] **Step 4: Review scope and commit the plan evidence**

Run:

```powershell
git diff --stat origin/master...HEAD
git diff --name-only origin/master...HEAD
```

Expected: only the five approved documentation paths appear. Mark executed plan steps complete and add the exact test result under an `Execution Status` section, then commit only the plan:

```powershell
git add -- docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md
git diff --cached --check
git commit -m "docs: record roadmap alignment execution"
```

### Task 5: Publish a Draft PR without merging

**Files:**
- Read: all branch changes from Tasks 1-4
- External write: branch `codex/agent-roadmap-authority-alignment`
- External write: one Draft PR targeting `master`

**Interfaces:**
- Consumes: the verified documentation-only commits.
- Produces: a reviewable Draft PR; it does not change GitHub issue authority yet.

- [x] **Step 1: Recheck branch and remote state**

Run:

```powershell
git fetch origin --prune
git status --short --branch
git rev-list --left-right --count origin/master...HEAD
git log --oneline origin/master..HEAD
```

Expected: clean branch, known divergence, and only the roadmap-alignment commits.

- [x] **Step 2: Re-run final verification against the fetched base**

Run:

```powershell
go test ./tests -count=1
git diff --check origin/master...HEAD
```

Expected: tests pass on the exact branch proposed for publication.

- [x] **Step 3: Push the dedicated branch**

```powershell
git push -u origin codex/agent-roadmap-authority-alignment
```

- [x] **Step 4: Open the Draft PR**

Use this title:

```text
docs: align Agent roadmap and backlog authority
```

The body must state:

```markdown
## What

- Move the minimum Commerce Tool foundation before Product Agent implementation.
- Split Product Agent delivery into Phase 2A Tool Foundation and Phase 2B bounded PoC.
- Recalibrate the active current-state document to the latest verified baseline.
- Reclassify the completed platform-aware asset plan as historical execution evidence.

## Why

The previous Phase 2/3 order required Product Agent to use tools before the shared Tool Registry and adapters existed, risking a disposable second contract. The active current-state authority was also calibrated to an older baseline and still prioritized a next source ahead of the approved Agent direction.

## Boundaries

- Documentation only; no runtime or business behavior changes.
- Does not claim SHEIN, 1688, TaskEvent V2, or Phase 0 production acceptance.
- GitHub Issues remain unchanged until this PR is merged.

## Verification

- `go test ./tests -count=1`
- `git diff --check origin/master...HEAD`
- Phase-order and placeholder scans recorded in the implementation plan.
```

- [x] **Step 5: Stop at the merge authorization boundary**

Report the Draft PR URL, checks, and changed paths. Do not mark it ready, merge it, or update issues until the user explicitly authorizes the next action.

### Task 6: Synchronize GitHub Issues after the documentation merge

**Files:**
- External write after merge only: issues #125, #127, #128, #131, #132, #133, #134, and #137

**Interfaces:**
- Consumes: the merged documentation commit on `master` and fresh issue snapshots.
- Produces: issue titles, labels, dependencies, and roadmap order matching the merged authority.

- [ ] **Step 1: Verify the merge and capture rollback snapshots**

Fetch `master`, confirm the merged authority text is present, then fetch all eight issues and retain their exact pre-change title, labels, and body in the execution log. If the documentation is not on `master`, stop without changing issues.

- [ ] **Step 2: Apply exact title and priority changes**

Use this mapping:

| Issue | New title | Priority label |
| --- | --- | --- |
| #128 | `[EPIC][Phase 2A][P0] Commerce Tool Foundation：Product Agent 的最小受控工具边界` | `P0` |
| #133 | `[Phase 2A][P0][Tools] 定义 Commerce Tool Registry、权限与风险等级` | `P0` |
| #134 | `[Phase 2A][P0][Tools] 接入首批 Product / Source / Validator 只读与提案工具` | `P0` |
| #127 | `[EPIC][Phase 2B][P0] Product Agent：商品资料诊断与补全 PoC` | keep `P0` |
| #131 | `[Phase 2B][P0][Agent Runtime] 定义 Product Agent 有界运行时、状态与工具调用合同` | keep `P0` |
| #132 | `[Phase 2B][P0][Product Agent] 实现商品诊断与补全 PoC，并与固定流程做对照评测` | keep `P0` |

Remove `P1` from #128/#133/#134 when adding `P0`. Preserve all non-priority labels.

- [ ] **Step 3: Apply exact dependency changes**

Set the dependency headers to:

```text
#127
Depends on: #126, #130, #128, #36, #47

#131
Parent: #127
Depends on: #130, #133

#132
Parent: #127
Depends on: #126, #130, #128, #131, #133, #134, #36, #47

#134
Parent: #128
Depends on: #30, #34, #133
```

Add to #128 that its exit gate is the minimum read/compute/propose foundation required by #127/#132 and that Phase 3 expands this same contract rather than replacing it.

- [ ] **Step 4: Replace the issue #137 phase ordering**

Keep Phase 0 and Phase 1 unchanged. Replace the current Phase 2 and Phase 3 sections with:

```markdown
## 2A. Minimum Commerce Tool Foundation

- [ ] #128 Phase 2A Epic：Product Agent 的最小受控工具边界
- [ ] #133 Tool Registry / permission / risk / side-effect declaration
- [ ] #134 首批 Product / Source / Validator read/compute/propose tools

**Exit Gate：** Product Agent 只能通过同一套受控 Tool Registry 和窄领域适配器完成取证、提案和确定性校验；不直接访问数据库、provider SDK 或 marketplace client。

---

## 2B. Product Agent PoC

- [ ] #127 Phase 2B Epic：商品资料诊断与补全 Agent
- [ ] #131 Agent Runtime 有界运行时、状态与工具调用合同
- [ ] #132 Product Agent PoC + fixed pipeline 对照评测

**前置：** Phase 1 和 Phase 2A Exit Gate 均有证据。

**Exit Gate：** 有证据证明 Agent 在至少一个核心质量指标上优于固定流程，同时风险指标不恶化；否则保持固定流程为主。

---

## 3. Commerce Tool Expansion and Production Hardening

Phase 3 扩展 Phase 2A 的同一 Tool Contract，不创建第二套接口。只有 Product Agent PoC 证明价值后才拉入更多工具、版本迁移、运行 SLO 或受控写能力；write/publish 仍需独立审批、幂等、权限与审计门禁。
```

Update #125 so its authority condition links the merged PR/commit, but keep Phase 0 open and leave every commercial acceptance checkbox unchanged.

- [ ] **Step 5: Verify every issue after mutation**

Fetch #125, #127, #128, #131, #132, #133, #134, and #137 again. Verify:

- Phase 2A appears before Phase 2B in #137;
- #132 depends on #128/#133/#134;
- #131 depends on #133;
- #128/#133/#134 carry `P0` and no longer carry `P1`;
- no Phase 0 commercial checkbox changed;
- no issue was closed, deleted, or recreated.

If any verification fails, restore that issue from the captured pre-change snapshot before continuing.

## Self-Review

- **Spec coverage:** Tasks 1-3 implement the three repository authority changes. Task 4 verifies phase order, evidence vocabulary, scope, and placeholders. Task 5 publishes without crossing the merge boundary. Task 6 applies every approved issue mutation only after merge.
- **Placeholder scan:** Every file, title, dependency, phase section, command, expected result, publication gate, and rollback action is explicit.
- **Interface consistency:** Phase 2A is produced by #128/#133/#134 and consumed by #127/#131/#132 in the strategy, current-state document, Draft PR, and post-merge issue mapping.

## Execution Handoff

This plan is saved at `docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md`. Execute Tasks 1-5 inline in the existing isolated worktree. Stop after the Draft PR is opened. Execute Task 6 only after the documentation PR is explicitly authorized and confirmed merged.
