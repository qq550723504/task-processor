# SHEIN Sale Attribute Card UI Design

## Goal

Optimize the `SHEIN 销售属性确认` card for action-first review flow.

The card should help users answer three questions quickly:

1. Can I confirm the current result now?
2. Do I need to fill a secondary sale attribute?
3. If not, what is the one next action?

This design intentionally deprioritizes diagnostic detail in the default view and moves it behind collapsible sections.

## Problems In The Current UI

1. Too many simultaneous explanation blocks compete for attention.
   The current card shows `下一步`, `怎么操作`, `推荐操作`, detailed result summaries, manual editing, and diagnostics at once.

2. Status and action do not read as a single decision.
   Users can see `待补齐` together with `直接确认当前结果`, which feels contradictory even when the secondary attribute is optional.

3. Manual edit controls appear too early in the reading flow.
   The 3-step correction UI is useful, but it should not dominate the initial state when the user may only need to confirm.

4. Missing secondary-template scenarios are visually confusing.
   When the category has no usable secondary template, the UI should explain that directly instead of behaving like a partially broken field-selection flow.

## Design Direction

Use an action-oriented card with four layers:

1. Status bar
2. Result summary
3. Primary action area
4. Advanced details

The default experience should feel like a lightweight task decision panel, not a troubleshooting console.

## Proposed Structure

### 1. Status Bar

Place a compact status header at the top of the card.

It should include:

- title: `SHEIN 销售属性确认`
- a short status badge such as:
  - `可直接确认`
  - `需要补其他规格`
  - `建议重新生成`
- one sentence summary describing why

Rules:

- Optional-secondary scenarios with a valid primary result should show `可直接确认`
- Required-secondary scenarios should show `需要补其他规格`
- Missing value-id or missing template scenarios that need retry should show `建议重新生成`

### 2. Result Summary

Replace the current three-way summary (`主规格 / 其他规格 / 下一步`) with only two compact summary cards:

- `主规格`
- `其他规格`

Each card should answer:

- what was recognized
- whether it is usable
- whether the current category supports it

Special handling:

- if no secondary template is available but secondary is optional, the secondary summary should say the category does not provide a usable secondary template and that this can be skipped

### 3. Primary Action Area

Immediately under the summary, show one clear main action and at most one secondary action.

Priority rules:

1. If the current result can be confirmed directly:
   - primary button: `直接确认当前结果`
   - secondary button: none, unless regeneration is specifically recommended

2. If regeneration is recommended:
   - primary button: `重新生成属性`
   - secondary button: none

3. If manual correction is required because secondary is required:
   - primary button: `展开手工修正`
   - secondary button: `重新生成属性` if available

The action area should not compete with multiple explanatory boxes.

## 4. Advanced Details

Move the following sections behind collapsible containers:

- manual correction editor
- matching explanation
- processing notes

The default collapsed labels should be:

- `手工修正规格`
- `查看匹配原因`
- `查看处理说明`

Behavior:

- when direct confirm is available, these sections remain collapsed by default
- when manual correction is required, the manual correction section may auto-expand

## Interaction Rules

### Optional Secondary

When secondary is optional:

- the card should not visually present the state as incomplete
- the user should be able to confirm without opening manual correction
- if no secondary template exists, show an explanation block instead of a misleading select field

### Required Secondary

When secondary is required:

- do not show `直接确认当前结果`
- emphasize the missing secondary field in the status summary
- make the correction section easy to enter from the main action area

### Regeneration Required

When a missing `value_id` or unavailable template makes current data unusable:

- push `重新生成属性` to the primary action position
- keep manual correction secondary and diagnostic

## Implementation Scope

This change is limited to the sale attribute review card component:

- `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx`
- its tests in:
  - `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

No structural change is planned for the broader workspace page in this iteration.

## Testing

Add or update tests for:

1. optional secondary without matching template
   - status shows `可直接确认`
   - direct confirm is visible
   - manual correction is collapsed by default
   - no misleading secondary select is shown in the default view

2. required secondary
   - status shows blocking guidance
   - direct confirm is hidden
   - main action leads to correction flow

3. regeneration-required state
   - regeneration becomes the primary action

## Out Of Scope

- redesigning the entire SHEIN review page
- changing backend sale attribute policy
- changing timeline, readiness, or final review layout
