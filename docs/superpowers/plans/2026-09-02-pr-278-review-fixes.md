# PR #278 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all four P2 review comments on PR #278's marketing hero without changing the existing product copy, routing, or motion contract.

**Architecture:** Keep the hero visual as a client component, but expose its capability cards through a semantic named group and list. Render the visual in its active, visible state during SSR and use an effect-driven client-only boot/reveal cycle so the animation never controls content availability. Keep responsive layout and contrast fixes in the existing CSS module.

**Tech Stack:** React 19, Next.js 16, Motion 13, CSS Modules, Vitest, Testing Library.

**Spec:** PR #278 inline review comments 3912322793, 3912322798, 3912322802, and 3912322806.

## Global Constraints

- Preserve the six capability labels and their existing Chinese copy.
- Preserve `data-motion-sequence="boot-reveal-active-pulse"` and the existing workbench links.
- Keep the reduced-motion path visible and non-animated.
- Do not modify backend, authentication, database, deployment, or unrelated marketing sections.
- Run Vitest with one worker in this environment (`--pool=forks --maxWorkers=1`) because the default worker configuration does not terminate reliably here.

---

### Task 1: Lock the accessibility and SSR contracts with regression tests

**Files:**
- Modify: `web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx`

**Interfaces:**
- Consumes: `MarketingHomepage` and `HeroSystemVisual` rendered through the existing test setup.
- Produces: Assertions that the visual is a named group containing a semantic list, its labels are exposed to assistive technology, and SSR markup contains no hidden boot state.

- [ ] **Step 1: Write the failing tests**

Update the visual assertion to query `getByRole("group", { name: "硕米 AI 电商能力架构" })`, assert a nested `list` with six `listitem` elements, and keep checking all six labels. Add:

```tsx
import { renderToString } from "react-dom/server";

import { HeroSystemVisual } from "@/components/marketing/hero-system-visual";

it("keeps the architecture visible in server-rendered markup", () => {
  const markup = renderToString(<HeroSystemVisual />);

  expect(markup).toContain("模型与调用治理");
  expect(markup).not.toContain("opacity:0");
});
```

Also assert the status message uses the new readable color and the narrow-phone capability label has `white-space: normal` through `getComputedStyle`.

- [ ] **Step 2: Run the focused tests to verify they fail for the review findings**

Run:

```powershell
pnpm exec vitest run src/components/marketing/marketing-homepage-hero.test.tsx --pool=forks --maxWorkers=1
```

Expected: the existing `role="img"` query cannot find the named group, SSR output contains `opacity:0`, and the current CSS values remain `#596e91` / `nowrap`.

- [ ] **Step 3: Commit**

```powershell
git add web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx
git commit -m "test(marketing): capture PR 278 review regressions"
```

### Task 2: Make the visual accessible and hydration-safe

**Files:**
- Modify: `web/listingkit-ui/src/components/marketing/hero-system-visual.tsx`
- Modify: `web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx`

**Interfaces:**
- Consumes: The existing `CAPABILITIES`, Motion variants, and `useReducedMotion` hook.
- Produces: A named `role="group"` containing a semantic `ul`/`li` capability structure, with server-visible markup and a client-only boot/reveal transition.

- [ ] **Step 1: Implement the semantic structure and client-only sequence**

Import `useEffect` and `useState`. Replace the atomic `role="img"` with `role="group"` while retaining the same `aria-label`. Wrap capability cards in a `motion.ul` with `aria-label="电商能力节点"` and render each card as a `motion.li`. Add `capabilityList` styles as needed for the desktop absolute layout and mobile grid.

Initialize a state such as `"active" | "boot"` to `"active"`, keep `initial={false}`, and in `useEffect` schedule `"boot"` followed by `"active"` on the next animation frame when reduced motion is not requested. Cancel the scheduled frame during cleanup. This keeps SSR/no-JS markup visible and starts the staged animation only after the client effect runs.

- [ ] **Step 2: Run the focused tests to verify the semantic and SSR fixes pass**

Run:

```powershell
pnpm exec vitest run src/components/marketing/marketing-homepage-hero.test.tsx --pool=forks --maxWorkers=1
```

Expected: accessibility and server-rendering assertions pass; only the CSS contrast/wrapping assertions remain red until Task 3.

- [ ] **Step 3: Commit**

```powershell
git add web/listingkit-ui/src/components/marketing/hero-system-visual.tsx web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx
git commit -m "fix(marketing): preserve hero semantics through hydration"
```

### Task 3: Fix status contrast and narrow-phone label wrapping

**Files:**
- Modify: `web/listingkit-ui/src/components/marketing/marketing-hero.module.css`
- Modify: `web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx`

**Interfaces:**
- Consumes: The existing `.systemStatus`, `.capabilityList`, `.capabilityCard`, and `.capabilityCopy > span` selectors.
- Produces: Readable normal-size status text and labels that wrap within the 320px two-column cards.

- [ ] **Step 1: Implement the CSS fixes**

Set `.systemStatus` to a lighter foreground such as `#9db5d8`, remove `white-space: nowrap` from `.capabilityCopy > span`, and add `overflow-wrap: anywhere` plus a normal line height so Chinese labels can wrap in the available mobile card width. Preserve `white-space: nowrap` on the eyebrow text, which is a separate short status label.

- [ ] **Step 2: Run the focused tests to verify all review regressions pass**

Run:

```powershell
pnpm exec vitest run src/components/marketing/marketing-homepage-hero.test.tsx --pool=forks --maxWorkers=1
```

Expected: the focused suite passes with all tests green.

- [ ] **Step 3: Run frontend validation**

Run:

```powershell
pnpm lint
pnpm typecheck
pnpm test -- --pool=forks --maxWorkers=1
```

Expected: lint, typecheck, and the complete Vitest suite exit with code 0 and report no failures.

- [ ] **Step 4: Commit**

```powershell
git add web/listingkit-ui/src/components/marketing/marketing-hero.module.css web/listingkit-ui/src/components/marketing/marketing-homepage-hero.test.tsx
git commit -m "fix(marketing): address PR 278 hero accessibility review"
```

## Self-Review Checklist

- [ ] Each review comment maps to a regression assertion and implementation change.
- [ ] No new role hides the capability labels from the accessibility tree.
- [ ] SSR markup remains visible before hydration and with JavaScript disabled.
- [ ] Reduced-motion users see the active visual without the boot sequence.
- [ ] CSS changes stay within the hero module and do not affect unrelated sections.
