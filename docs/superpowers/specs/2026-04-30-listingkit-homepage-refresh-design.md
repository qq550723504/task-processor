# ListingKit Homepage Refresh Design

## Summary

Refresh the ListingKit homepage from a single centered launcher into an efficiency-first product entry screen. The page should still feel like a product surface, but its main job is to get operators into the SHEIN workflow quickly, expose secondary tools without clutter, and surface recent work so users can resume in one click.

The primary action is **进入 SHEIN 工作台**. Secondary actions remain available for **开始新的 ListingKit 任务**, **SDS 选品**, and **任务列表**. The homepage should also show a dedicated **继续最近任务** shortcut plus up to three recent task cards.

## Goals

- Make the homepage feel intentional instead of placeholder-like.
- Prioritize the SHEIN workflow as the main operational entry.
- Preserve direct access to general ListingKit tools.
- Reduce time-to-resume for users returning to active work.
- Keep the page lightweight and fast; no heavy dashboard sprawl.

## Non-Goals

- This change does not redesign downstream task pages or the SHEIN workspace itself.
- This change does not introduce new backend APIs unless the current task list query is insufficient.
- This change does not attempt a full analytics or KPI dashboard.

## User Experience

### Primary Structure

The homepage is split into three layers:

1. **Hero / entry band**
   - Product title and short description of what ListingKit is for.
   - Main CTA: `进入 SHEIN 工作台`
   - Secondary CTA: `开始新的 ListingKit 任务`
   - Visual treatment should feel polished and operational, not marketing-heavy.

2. **Quick tools band**
   - Compact cards or large buttons for:
     - `SHEIN 工作台`
     - `SDS 选品`
     - `任务列表`
   - These are shortcuts, not explanations-heavy content.

3. **Recent work band**
   - A prominent `继续最近任务` action for the top resumable task.
   - Up to 3 recent task cards with:
     - title or derived task name
     - platform(s)
     - status
     - updated time
     - direct continue link

### Content Priority

- SHEIN should be visually first and largest.
- ListingKit generic entry should remain visible but secondary.
- Recent work should be visible above the fold on desktop if space permits.
- On mobile, the order remains:
  - hero
  - quick tools
  - continue recent task
  - recent task list

### Visual Direction

- Keep a clean workbench look with stronger hierarchy and atmosphere than the current single-button layout.
- Use layered surfaces, subtle background texture or gradient, and clearer section framing.
- Avoid looking like a BI dashboard or a template landing page.
- Preserve the existing design language where possible, but elevate typography and spacing.

## Data and Behavior

### Data Sources

- Reuse existing task list/query capability where possible.
- The homepage needs enough task data to determine:
  - most recent resumable task
  - up to three recent tasks

If the existing frontend API layer already exposes suitable task list data, use it. If not, extend only the client-side fetch path needed for the homepage.

### Resume Heuristic

`继续最近任务` should target the highest-value resumable task using this order:

1. newest incomplete or actionable SHEIN-related task
2. newest incomplete task of any platform
3. if all are completed, newest task overall

If no tasks exist, hide the continue shortcut and let quick tools dominate.

### Loading / Empty States

- While loading recent tasks:
  - show lightweight skeletons, not spinners-only
- If there are no tasks:
  - show a concise empty state under recent work
  - keep primary and quick-entry actions fully available

### Error Handling

- If recent tasks fail to load:
  - do not block the homepage
  - show a small inline failure hint in the recent work area
  - keep entry actions active

## Component Design

### Page Composition

Likely structure:

- `app/page.tsx`
  - becomes a composed homepage instead of just rendering `TaskLauncher`
- new or adapted homepage sections:
  - hero section
  - quick tool cards
  - recent task summary
  - recent task list cards

### Reuse Strategy

- Reuse existing button, card, badge, and task-link patterns where they already exist.
- Do not embed the old `TaskLauncher` as the entire page. It can be repurposed as one secondary action if useful.
- Prefer small focused homepage components over growing one large file.

## Navigation

- `进入 SHEIN 工作台` should go directly to the SHEIN workflow entry page.
- `开始新的 ListingKit 任务` should keep the current generic task creation path.
- `SDS 选品` should go to the SDS page.
- `任务列表` should go to the ListingKit index/task listing page.
- Recent task cards should deep-link to the most appropriate continuation page for that task.

## Testing

### Functional Checks

- Homepage renders without tasks.
- Homepage renders with recent tasks.
- Continue shortcut chooses the expected task.
- Quick action links navigate to the intended routes.
- Recent task load failure does not break the page.

### UI Checks

- Desktop and mobile layouts preserve priority and readability.
- CTA hierarchy is visually obvious.
- Recent task cards remain scannable with long titles and mixed statuses.

## Risks

- Pulling too much task detail onto the homepage could make it feel crowded.
- If resume routing is too clever, users may distrust where `继续最近任务` lands.
- Over-styling the page could reduce efficiency rather than improve it.

## Recommendation

Implement the homepage as an **efficiency-first branded entry screen**:

- one dominant SHEIN CTA
- one secondary generic ListingKit CTA
- one compact quick-tools row
- one recent-work section with a resume shortcut and up to three tasks

This gives the product a real front door without sacrificing operator speed.
