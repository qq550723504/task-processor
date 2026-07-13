# Paid Pilot Product Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish the approved `paid_pilot` product catalog and usage policy, then anchor it in the paid-pilot execution plan without changing runtime billing or entitlement behavior.

**Architecture:** A focused product-policy document is the canonical source for the customer-facing plan, its capabilities, metrics, and non-billable outcomes. A small document-contract test prevents later wording changes from silently reintroducing Basic/Professional ambiguity or an unsafe publication/source default. The product index and execution plan point to that canonical document rather than duplicating the policy.

**Tech Stack:** Markdown, Go standard-library document test, existing `go test ./tests` harness.

## Global Constraints

- The only launch package code is `paid_pilot`; do not retain Basic or Professional as launch offerings.
- The pilot is invite-only and manually approved; do not introduce public pricing or self-service purchase.
- `shein_publish` is a separate, tenant-scoped entitlement and defaults to disabled.
- Keep 1688 disabled until its M2 safety gate has been completed through a later policy change.
- Commit usage only for successful design, image, remote draft, and remote publish outcomes; cancellation, failure, platform rejection, and replay do not create additional usage.
- `storage_bytes_current` measures retained current occupancy; internal AI cost is recorded but not billed per call.
- PAY-040 is documentation and policy verification only. Do not change Go runtime behavior, database schema, subscriptions, billing, feature flags, or external access.

---

### Task 1: Add a policy-contract regression test

**Files:**
- Create: `tests/paid_pilot_product_catalog_policy_test.go`

**Interfaces:**
- Consumes: `docs/product/listingkit-paid-pilot-product-catalog.md` as the policy source.
- Produces: `TestPaidPilotProductCatalogFreezesApprovedPolicy`, a guard that fails when required commercial safety language is removed.

- [ ] **Step 1: Write the failing document-contract test**

Create the test before the catalog exists:

```go
package tests

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestPaidPilotProductCatalogFreezesApprovedPolicy(t *testing.T) {
    path := filepath.Join("..", "docs", "product", "listingkit-paid-pilot-product-catalog.md")
    content, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read paid-pilot product catalog: %v", err)
    }

    for _, required := range []string{
        "`paid_pilot`",
        "仅限邀请制",
        "`shein_publish`",
        "默认关闭",
        "1688",
        "保持关闭",
        "studio_design_jobs_succeeded",
        "product_image_jobs_succeeded",
        "shein_drafts_succeeded",
        "shein_publishes_succeeded",
        "storage_bytes_current",
        "失败、取消、平台拒绝和工程重放不计费",
    } {
        if !strings.Contains(string(content), required) {
            t.Errorf("paid-pilot product catalog must contain %q", required)
        }
    }
}
```

- [ ] **Step 2: Verify the test is red**

Run:

```powershell
$env:GOWORK='off'; go test ./tests -run TestPaidPilotProductCatalogFreezesApprovedPolicy -count=1
```

Expected: FAIL because `docs/product/listingkit-paid-pilot-product-catalog.md` does not yet exist.

- [ ] **Step 3: Commit the red-to-green test with its policy document in Task 2**

Do not create a standalone commit for a knowingly failing test.

### Task 2: Publish the canonical `paid_pilot` catalog and usage policy

**Files:**
- Create: `docs/product/listingkit-paid-pilot-product-catalog.md`
- Modify: `tests/paid_pilot_product_catalog_policy_test.go`

**Interfaces:**
- Consumes: the decision matrix in `docs/superpowers/specs/2026-07-13-paid-pilot-product-catalog-design.md`.
- Produces: one product-facing catalog document whose terms are protected by `TestPaidPilotProductCatalogFreezesApprovedPolicy`.

- [ ] **Step 1: Write the catalog content that makes the policy test pass**

Create the Markdown document with these exact sections and decisions:

```markdown
# ListingKit Paid Pilot Product Catalog

## Plan and enrollment

- 唯一首发套餐代码为 `paid_pilot`，仅限邀请制。
- 套餐由 platform-admin 根据已批准合同/订单人工开通、暂停和到期管理；没有自助购买或自动收费。

## Capability matrix

| 能力 | `paid_pilot` 默认状态 | 额外门禁 |
| --- | --- | --- |
| ListingKit 任务、SDS、设计生成、商品图、保存草稿 | 可用 | 租户准入、现有授权/配额与店铺 preflight |
| SHEIN 正式发布 | 默认关闭 | 独立 `shein_publish` entitlement、租户 preflight、草稿验收和业务批准 |
| 1688 来源 | 保持关闭 | M2 完成后的独立政策更新 |

## Usage policy

仅在成功结果上提交用量：`studio_design_jobs_succeeded`、`product_image_jobs_succeeded`、`shein_drafts_succeeded`、`shein_publishes_succeeded`。`listing_tasks_created` 只作为首发运营配额指标；`storage_bytes_current` 按当前保留占用计算。失败、取消、平台拒绝和工程重放不计费；工程重放复用同一业务事件。AI 内部成本记录但不按调用向客户收费。
```

Add concise sections for manual audit fields, customer-visible quota information, suspension/expiry behavior, and the explicit boundary that no charge is activated until PAY-041 through PAY-044 are complete.

- [ ] **Step 2: Verify the document contract is green**

Run:

```powershell
$env:GOWORK='off'; go test ./tests -run TestPaidPilotProductCatalogFreezesApprovedPolicy -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit the policy contract and catalog**

```powershell
git add tests/paid_pilot_product_catalog_policy_test.go docs/product/listingkit-paid-pilot-product-catalog.md
git commit -m "docs: define paid pilot product catalog and usage policy"
```

### Task 3: Anchor the policy in product documentation and the execution plan

**Files:**
- Modify: `docs/product/README.md`
- Modify: `docs/product/listingkit-paid-pilot-execution-plan.md`

**Interfaces:**
- Consumes: `docs/product/listingkit-paid-pilot-product-catalog.md` as the sole catalog and usage-policy source.
- Produces: discoverable product documentation and a completed PAY-040 decision record without duplicating the detailed matrix.

- [ ] **Step 1: Add the catalog to the product reading order**

Insert the catalog immediately after the paid-pilot execution plan in `docs/product/README.md`:

```markdown
6. [ListingKit 付费试点产品目录与用量政策](./listingkit-paid-pilot-product-catalog.md)
```

Renumber the following entries and add one sentence under “当前执行入口” stating that the catalog governs the single `paid_pilot` package, publication entitlement, and usage policy.

- [ ] **Step 2: Record PAY-040 closure in the execution plan**

Under `### PAY-040：定义商业产品目录和套餐语义`, add a “已冻结的首发政策” subsection that links to `listingkit-paid-pilot-product-catalog.md` and records only these facts:

```markdown
- 唯一首发套餐为 `paid_pilot`，仅限邀请制和人工开通。
- SDS、设计生成、商品图和保存草稿在租户准入后可用；1688 保持关闭。
- `shein_publish` 独立 entitlement，默认关闭。
- 成功才提交用量；失败、取消、平台拒绝和工程重放不计费；storage 按当前占用计量。
```

Do not mark M4 acceptance criteria complete: PAY-041 through PAY-044 still provide the ledger, enforcement, commercial ledger, and reconciliation required to charge customers.

- [ ] **Step 3: Run the policy test and document checks**

Run:

```powershell
$env:GOWORK='off'; go test ./tests -run TestPaidPilotProductCatalogFreezesApprovedPolicy -count=1
git diff --check
git diff -- docs/product/README.md docs/product/listingkit-paid-pilot-execution-plan.md
```

Expected: the policy test passes, no whitespace errors are reported, and only the product index and PAY-040 decision record changed.

- [ ] **Step 4: Commit the documentation integration**

```powershell
git add docs/product/README.md docs/product/listingkit-paid-pilot-execution-plan.md
git commit -m "docs: record paid pilot catalog policy"
```

## Final verification

- [ ] Run `$env:GOWORK='off'; go test ./tests -count=1`.
- [ ] Run `git diff origin/master...HEAD --check` and confirm the only non-spec changes are the policy document, its contract test, the product index, and the PAY-040 decision record.
- [ ] Confirm no runtime source, schema, deployment, subscription, or billing file changed.
