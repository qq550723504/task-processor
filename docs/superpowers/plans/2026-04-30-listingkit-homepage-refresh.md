# ListingKit Homepage Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder ListingKit homepage with an efficiency-first entry screen that prioritizes the SHEIN workflow, preserves general ListingKit entry points, and surfaces recent work for one-click resume.

**Architecture:** Keep the homepage fully client-driven inside the existing Next.js app. Reuse the current task list API/query layer for recent work, add a small homepage-specific presentation layer for task ranking and entry cards, and compose the new sections from focused React components rather than growing `app/page.tsx`.

**Tech Stack:** Next.js App Router, React 19, TypeScript, TanStack Query, existing shared `Button` / `Card` primitives, Vitest + Testing Library.

---

## File Structure

### Existing files to modify

- `D:\code\task-processor\web\listingkit-ui\src\app\page.tsx`
  - Replace the current single `TaskLauncher` wrapper with the new homepage composition.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\tasks\task-launcher.tsx`
  - Keep as reusable legacy task-id launcher, but do not use it as the whole homepage anymore.
- `D:\code\task-processor\web\listingkit-ui\src\lib\query\use-task-list.ts`
  - Reuse as-is if sufficient; otherwise adjust only if homepage needs a lighter refresh cadence or options override.

### New files to create

- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-homepage.tsx`
  - Top-level homepage composition and section orchestration.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-hero.tsx`
  - Hero band with primary and secondary CTAs.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-quick-tools.tsx`
  - Quick-entry cards for SHEIN, SDS, and task list.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-recent-work.tsx`
  - Continue shortcut, recent task cards, empty/loading/error states.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-task-card.tsx`
  - Focused task card renderer for recent work.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-task-card.test.tsx`
  - Card rendering and status display tests.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-recent-work.test.tsx`
  - Resume shortcut and empty/error/loading behavior tests.
- `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-homepage.test.tsx`
  - End-to-end homepage composition tests.
- `D:\code\task-processor\web\listingkit-ui\src\lib\listingkit\home-recent-tasks.ts`
  - Pure helpers to rank resumable tasks and choose the top “continue” target.
- `D:\code\task-processor\web\listingkit-ui\src\lib\listingkit\home-recent-tasks.test.ts`
  - Unit tests for the ranking heuristic.

## Task 1: Add homepage recent-task ranking helpers

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\lib\listingkit\home-recent-tasks.ts`
- Create: `D:\code\task-processor\web\listingkit-ui\src\lib\listingkit\home-recent-tasks.test.ts`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\lib\types\listingkit.ts`

- [ ] **Step 1: Write the failing heuristic tests**

```ts
import { describe, expect, it } from "vitest";

import {
  pickContinueTask,
  sortRecentTasksForHomepage,
} from "@/lib/listingkit/home-recent-tasks";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

function makeTask(overrides: Partial<ListingKitTaskListItem>): ListingKitTaskListItem {
  return {
    task_id: "task-1",
    status: "completed",
    platforms: ["shein"],
    title: "Task",
    image_count: 0,
    created_at: "2026-04-30T10:00:00+08:00",
    updated_at: "2026-04-30T10:00:00+08:00",
    ...overrides,
  };
}

describe("home-recent-tasks", () => {
  it("prefers the newest actionable SHEIN task for continue", () => {
    const tasks = [
      makeTask({
        task_id: "done-amazon",
        platforms: ["amazon"],
        status: "completed",
        updated_at: "2026-04-30T09:00:00+08:00",
      }),
      makeTask({
        task_id: "needs-review-shein",
        platforms: ["shein"],
        status: "needs_review",
        updated_at: "2026-04-30T11:00:00+08:00",
      }),
      makeTask({
        task_id: "processing-temu",
        platforms: ["temu"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("needs-review-shein");
  });

  it("falls back to newest incomplete task when no actionable SHEIN task exists", () => {
    const tasks = [
      makeTask({
        task_id: "processing-amazon",
        platforms: ["amazon"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
      makeTask({
        task_id: "done-shein",
        platforms: ["shein"],
        status: "completed",
        updated_at: "2026-04-30T13:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("processing-amazon");
  });

  it("returns newest overall task when all tasks are completed", () => {
    const tasks = [
      makeTask({ task_id: "older", updated_at: "2026-04-30T09:00:00+08:00" }),
      makeTask({ task_id: "newer", updated_at: "2026-04-30T10:00:00+08:00" }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("newer");
  });

  it("caps recent tasks to the newest three after sorting", () => {
    const tasks = ["1", "2", "3", "4"].map((id, index) =>
      makeTask({
        task_id: id,
        updated_at: `2026-04-30T1${index}:00:00+08:00`,
      }),
    );

    expect(sortRecentTasksForHomepage(tasks).map((task) => task.task_id)).toEqual([
      "4",
      "3",
      "2",
    ]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
npm run test -- src/lib/listingkit/home-recent-tasks.test.ts
```

Expected: FAIL because the helper module does not exist yet.

- [ ] **Step 3: Write the minimal ranking helpers**

```ts
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

const ACTIONABLE_STATUSES = new Set(["pending", "processing", "needs_review", "failed"]);

function updatedAtValue(task: ListingKitTaskListItem) {
  const value = Date.parse(task.updated_at ?? task.created_at ?? "");
  return Number.isNaN(value) ? 0 : value;
}

function isActionable(task: ListingKitTaskListItem) {
  return ACTIONABLE_STATUSES.has(task.status ?? "");
}

function isSheinTask(task: ListingKitTaskListItem) {
  return (task.platforms ?? []).includes("shein");
}

export function sortRecentTasksForHomepage(tasks: ListingKitTaskListItem[]) {
  return [...tasks]
    .sort((left, right) => updatedAtValue(right) - updatedAtValue(left))
    .slice(0, 3);
}

export function pickContinueTask(tasks: ListingKitTaskListItem[]) {
  const sorted = [...tasks].sort((left, right) => updatedAtValue(right) - updatedAtValue(left));
  return (
    sorted.find((task) => isSheinTask(task) && isActionable(task)) ??
    sorted.find((task) => isActionable(task)) ??
    sorted[0] ??
    null
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
npm run test -- src/lib/listingkit/home-recent-tasks.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/lib/listingkit/home-recent-tasks.ts web/listingkit-ui/src/lib/listingkit/home-recent-tasks.test.ts
git commit -m "feat: add homepage recent task ranking helpers"
```

## Task 2: Build homepage recent-work and task-card components

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-task-card.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-task-card.test.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-recent-work.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-recent-work.test.tsx`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\tasks\task-list-page.tsx`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\components\shared\card.tsx`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\components\shared\button.tsx`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\components\shared\empty-state.tsx`

- [ ] **Step 1: Write the failing recent-work component tests**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

function makeTask(overrides: Partial<ListingKitTaskListItem>): ListingKitTaskListItem {
  return {
    task_id: "task-1",
    status: "completed",
    platforms: ["shein"],
    title: "Task",
    image_count: 0,
    created_at: "2026-04-30T10:00:00+08:00",
    updated_at: "2026-04-30T10:00:00+08:00",
    ...overrides,
  };
}

describe("ListingKitHomeRecentWork", () => {
  it("renders continue shortcut and up to three recent tasks", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({ task_id: "resume", status: "needs_review", title: "Resume me" }),
          makeTask({ task_id: "two", title: "Two", updated_at: "2026-04-30T09:00:00+08:00" }),
          makeTask({ task_id: "three", title: "Three", updated_at: "2026-04-30T08:00:00+08:00" }),
          makeTask({ task_id: "four", title: "Four", updated_at: "2026-04-30T07:00:00+08:00" }),
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: /继续最近任务/i })).toBeInTheDocument();
    expect(screen.getByText("Resume me")).toBeInTheDocument();
    expect(screen.getByText("Two")).toBeInTheDocument();
    expect(screen.getByText("Three")).toBeInTheDocument();
    expect(screen.queryByText("Four")).not.toBeInTheDocument();
  });

  it("renders empty state when no tasks exist", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError={false} tasks={[]} />);
    expect(screen.getByText("还没有最近任务")).toBeInTheDocument();
  });

  it("renders inline error state without blocking the page", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError tasks={[]} />);
    expect(screen.getByText("最近任务暂时加载失败")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
npm run test -- src/components/listingkit/home/listingkit-home-recent-work.test.tsx src/components/listingkit/home/listingkit-home-task-card.test.tsx
```

Expected: FAIL because the components do not exist yet.

- [ ] **Step 3: Implement the recent task card and recent-work section**

```tsx
// listingkit-home-task-card.tsx
import Link from "next/link";

import { Card } from "@/components/shared/card";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

type Props = {
  task: ListingKitTaskListItem;
};

function taskHref(task: ListingKitTaskListItem) {
  const platform = task.platforms?.[0] ?? "shein";
  return `/listing-kits/${task.task_id}/workspace?platform=${platform}`;
}

export function ListingKitHomeTaskCard({ task }: Props) {
  return (
    <Card className="border-white/70 bg-white/88 p-4 shadow-[0_14px_36px_rgba(39,39,42,0.07)]">
      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-zinc-500">
          {(task.platforms ?? []).join(" / ") || "ListingKit"}
        </p>
        <h3 className="text-base font-semibold text-zinc-950">
          {task.product_name || task.title || task.task_id.slice(0, 8)}
        </h3>
        <p className="text-sm text-zinc-600">状态：{task.status}</p>
        <Link href={taskHref(task)} className="text-sm font-medium text-teal-700 hover:text-teal-800">
          继续处理
        </Link>
      </div>
    </Card>
  );
}
```

```tsx
// listingkit-home-recent-work.tsx
import Link from "next/link";

import { EmptyState } from "@/components/shared/empty-state";
import { ListingKitHomeTaskCard } from "@/components/listingkit/home/listingkit-home-task-card";
import { pickContinueTask, sortRecentTasksForHomepage } from "@/lib/listingkit/home-recent-tasks";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

type Props = {
  tasks: ListingKitTaskListItem[];
  isLoading: boolean;
  isError: boolean;
};

function taskHref(task: ListingKitTaskListItem) {
  const platform = task.platforms?.[0] ?? "shein";
  return `/listing-kits/${task.task_id}/workspace?platform=${platform}`;
}

export function ListingKitHomeRecentWork({ tasks, isLoading, isError }: Props) {
  if (isLoading) {
    return <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]"><div className="h-44 animate-pulse rounded-[2rem] bg-white/60" /><div className="grid gap-4 md:grid-cols-3 lg:grid-cols-1"><div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" /><div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" /><div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" /></div></div>;
  }

  if (isError) {
    return (
      <div className="rounded-[2rem] border border-amber-200 bg-amber-50/80 p-5 text-sm text-amber-800">
        最近任务暂时加载失败，但你仍然可以直接进入 SHEIN 工作台或新建任务。
      </div>
    );
  }

  if (!tasks.length) {
    return (
      <EmptyState
        title="还没有最近任务"
        description="从 SHEIN 工作台或通用任务入口开始，新的联调和正式任务会显示在这里。"
      />
    );
  }

  const continueTask = pickContinueTask(tasks);
  const recentTasks = sortRecentTasksForHomepage(tasks);

  return (
    <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      {continueTask ? (
        <div className="rounded-[2rem] border border-white/70 bg-zinc-950 p-6 text-white shadow-[0_24px_80px_rgba(24,24,27,0.22)]">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-teal-300">Continue</p>
          <h2 className="mt-3 text-2xl font-semibold">继续最近任务</h2>
          <p className="mt-2 text-sm leading-6 text-zinc-300">
            {continueTask.product_name || continueTask.title || continueTask.task_id}
          </p>
          <Link href={taskHref(continueTask)} className="mt-6 inline-flex h-11 items-center justify-center rounded-xl bg-white px-5 text-sm font-medium text-zinc-950 transition hover:bg-zinc-100">
            打开当前工作区
          </Link>
        </div>
      ) : null}

      <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-1">
        {recentTasks.map((task) => (
          <ListingKitHomeTaskCard key={task.task_id} task={task} />
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
npm run test -- src/lib/listingkit/home-recent-tasks.test.ts src/components/listingkit/home/listingkit-home-recent-work.test.tsx src/components/listingkit/home/listingkit-home-task-card.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.tsx web/listingkit-ui/src/components/listingkit/home/listingkit-home-task-card.test.tsx web/listingkit-ui/src/components/listingkit/home/listingkit-home-recent-work.tsx web/listingkit-ui/src/components/listingkit/home/listingkit-home-recent-work.test.tsx web/listingkit-ui/src/lib/listingkit/home-recent-tasks.ts web/listingkit-ui/src/lib/listingkit/home-recent-tasks.test.ts
git commit -m "feat: add listingkit homepage recent work section"
```

## Task 3: Build hero and quick-tools homepage sections

**Files:**
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-hero.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-home-quick-tools.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-homepage.tsx`
- Create: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\listingkit-homepage.test.tsx`
- Modify: `D:\code\task-processor\web\listingkit-ui\src\app\page.tsx`
- Reference: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\tasks\task-launcher.tsx`

- [ ] **Step 1: Write the failing homepage composition test**

```tsx
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import Home from "@/app/page";

vi.mock("@/lib/query/use-task-list", () => ({
  useListingKitTasks: () => ({
    isLoading: false,
    isError: false,
    data: {
      total: 1,
      items: [
        {
          task_id: "task-1",
          status: "needs_review",
          platforms: ["shein"],
          title: "Botanical clock",
          image_count: 0,
          created_at: "2026-04-30T10:00:00+08:00",
          updated_at: "2026-04-30T10:00:00+08:00",
        },
      ],
    },
  }),
}));

describe("Home", () => {
  it("renders the SHEIN-first homepage layout", () => {
    const client = new QueryClient();
    render(
      <QueryClientProvider client={client}>
        <Home />
      </QueryClientProvider>,
    );

    expect(screen.getByText("ListingKit")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "进入 SHEIN 工作台" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "开始新的 ListingKit 任务" })).toBeInTheDocument();
    expect(screen.getByText("继续最近任务")).toBeInTheDocument();
    expect(screen.getByText("Botanical clock")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
npm run test -- src/components/listingkit/home/listingkit-homepage.test.tsx
```

Expected: FAIL because the homepage still renders only `TaskLauncher`.

- [ ] **Step 3: Implement the composed homepage**

```tsx
// listingkit-home-hero.tsx
import Link from "next/link";

export function ListingKitHomeHero() {
  return (
    <section className="grid gap-6 rounded-[2rem] border border-white/70 bg-white/78 p-8 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur lg:grid-cols-[1.2fr_0.8fr]">
      <div className="space-y-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">ListingKit</p>
        <h1 className="max-w-3xl text-5xl font-semibold tracking-[-0.05em] text-zinc-950">
          把 SHEIN 上架工作流放回首页正中央。
        </h1>
        <p className="max-w-2xl text-sm leading-7 text-zinc-600">
          直接进入 SHEIN 工作台继续做图、补资料和提交，同时保留通用 ListingKit 入口与最近任务恢复能力。
        </p>
        <div className="flex flex-wrap gap-3">
          <Link href="/listing-kits/shein" className="inline-flex h-11 items-center justify-center rounded-xl bg-zinc-950 px-5 text-sm font-medium text-white transition hover:bg-zinc-800">
            进入 SHEIN 工作台
          </Link>
          <Link href="/listing-kits/new" className="inline-flex h-11 items-center justify-center rounded-xl bg-white px-5 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100">
            开始新的 ListingKit 任务
          </Link>
        </div>
      </div>
      <div className="rounded-[1.75rem] border border-zinc-200/70 bg-[linear-gradient(160deg,rgba(24,24,27,0.96),rgba(39,39,42,0.92))] p-6 text-white">
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-teal-300">Workflow Focus</p>
        <div className="mt-5 space-y-4 text-sm text-zinc-300">
          <div><span className="block text-base font-semibold text-white">SHEIN 工作台优先</span><span>从选品、款式图、商品图到资料提交，减少跳页和找入口。</span></div>
          <div><span className="block text-base font-semibold text-white">最近任务可续跑</span><span>首页保留最近任务入口，避免回忆 task id。</span></div>
        </div>
      </div>
    </section>
  );
}
```

```tsx
// listingkit-home-quick-tools.tsx
import Link from "next/link";

const TOOLS = [
  { title: "SHEIN 工作台", description: "进入 SHEIN 批次工作流，继续款式图与资料处理。", href: "/listing-kits/shein" },
  { title: "SDS 选品", description: "直接浏览 SDS 商品、变体和印刷区。", href: "/listing-kits/sds" },
  { title: "任务列表", description: "查看最近任务、状态与恢复入口。", href: "/listing-kits" },
];

export function ListingKitHomeQuickTools() {
  return (
    <section className="grid gap-4 md:grid-cols-3">
      {TOOLS.map((tool) => (
        <Link
          key={tool.href}
          href={tool.href}
          className="rounded-[1.5rem] border border-white/70 bg-white/82 p-5 shadow-[0_18px_44px_rgba(39,39,42,0.07)] transition hover:-translate-y-0.5 hover:shadow-[0_24px_54px_rgba(39,39,42,0.10)]"
        >
          <p className="text-sm font-semibold text-zinc-950">{tool.title}</p>
          <p className="mt-2 text-sm leading-6 text-zinc-600">{tool.description}</p>
        </Link>
      ))}
    </section>
  );
}
```

```tsx
// listingkit-homepage.tsx
"use client";

import { ListingKitHomeHero } from "@/components/listingkit/home/listingkit-home-hero";
import { ListingKitHomeQuickTools } from "@/components/listingkit/home/listingkit-home-quick-tools";
import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import { useListingKitTasks } from "@/lib/query/use-task-list";

export function ListingKitHomepage() {
  const tasks = useListingKitTasks({ page: 1, page_size: 6 });
  const items = tasks.data?.items ?? [];

  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-[radial-gradient(circle_at_12%_10%,rgba(20,184,166,0.18),transparent_30%),radial-gradient(circle_at_86%_4%,rgba(251,146,60,0.16),transparent_26%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)] px-6 py-10">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6">
        <ListingKitHomeHero />
        <ListingKitHomeQuickTools />
        <ListingKitHomeRecentWork tasks={items} isLoading={tasks.isLoading} isError={tasks.isError} />
      </div>
    </div>
  );
}
```

```tsx
// app/page.tsx
import { ListingKitHomepage } from "@/components/listingkit/home/listingkit-homepage";

export default function Home() {
  return <ListingKitHomepage />;
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
npm run test -- src/components/listingkit/home/listingkit-homepage.test.tsx src/components/listingkit/home/listingkit-home-recent-work.test.tsx src/components/listingkit/home/listingkit-home-task-card.test.tsx src/lib/listingkit/home-recent-tasks.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/app/page.tsx web/listingkit-ui/src/components/listingkit/home
git commit -m "feat: add SHEIN-first listingkit homepage"
```

## Task 4: Verify styling, routing, and production build

**Files:**
- Verify only: `D:\code\task-processor\web\listingkit-ui\src\app\page.tsx`
- Verify only: `D:\code\task-processor\web\listingkit-ui\src\components\listingkit\home\*`
- Optional cleanup: homepage class names in the files above if layout issues appear

- [ ] **Step 1: Run the focused homepage test suite**

Run:

```bash
npm run test -- src/components/listingkit/home/listingkit-homepage.test.tsx src/components/listingkit/home/listingkit-home-recent-work.test.tsx src/components/listingkit/home/listingkit-home-task-card.test.tsx src/lib/listingkit/home-recent-tasks.test.ts
```

Expected: PASS with no snapshot-free regressions.

- [ ] **Step 2: Run the full frontend build**

Run:

```bash
npm run build
```

Expected: PASS and `/` remains a valid homepage route.

- [ ] **Step 3: Manually verify the homepage in the local browser**

Run / open:

```bash
http://127.0.0.1:3001/
```

Expected:
- primary CTA is `进入 SHEIN 工作台`
- secondary CTA is `开始新的 ListingKit 任务`
- quick tools show `SHEIN 工作台` / `SDS 选品` / `任务列表`
- recent work section shows either resume content or a clean empty state
- layout reads clearly on desktop width and a narrow mobile-width viewport

- [ ] **Step 4: Commit**

```bash
git add web/listingkit-ui/src/app/page.tsx web/listingkit-ui/src/components/listingkit/home web/listingkit-ui/src/lib/listingkit/home-recent-tasks.ts web/listingkit-ui/src/lib/listingkit/home-recent-tasks.test.ts
git commit -m "test: verify homepage refresh flow"
```

## Self-Review

- **Spec coverage:** The plan covers the primary SHEIN CTA, secondary generic ListingKit CTA, quick tools, resume shortcut, three-item recent list, loading/empty/error states, and homepage styling direction.
- **Placeholder scan:** No `TODO` / `TBD` placeholders remain. Each code-bearing step includes concrete code.
- **Type consistency:** The plan consistently uses `ListingKitTaskListItem`, `pickContinueTask`, and `sortRecentTasksForHomepage` across helper and component tasks.
