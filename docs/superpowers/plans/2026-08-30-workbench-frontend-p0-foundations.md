# Workbench Frontend P0 Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为硕米工作台建立统一的数据表格、长列表虚拟化、图表和浏览器可访问性测试底座，并统一使用 pnpm 管理前端依赖。

**Architecture:** 基础组件继续沿用现有 shadcn 风格的 `src/components/ui`，TanStack 只提供无样式状态与虚拟化能力，ECharts 封装在客户端组件边界内。Playwright 只测试公开页面，运行产物统一写入仓库根 `.local/playwright`，不把浏览器测试混入 Vitest。

**Tech Stack:** Next.js 16.3、React 19.2、TypeScript 6、pnpm 11.19、Vitest 4、TanStack Table 9.2.4、TanStack Virtual 3.14.10、ECharts 6.1.0、Playwright 1.62.1、axe-core Playwright 4.13.0

**Spec:** `docs/superpowers/specs/2026-08-30-internal-target-architecture-migration-design.md`

## Global Constraints

- 只修改 `web/listingkit-ui` 及本计划文档，不修改后端 `internal` 目录。
- 统一使用 `pnpm@11.19.0` 和 `pnpm-lock.yaml`；删除 npm 的 `package-lock.json`。
- 扩展现有 `Table` 原语，不引入 Ant Design、MUI 或第二套视觉体系。
- DataTable 负责排序、渲染和空状态，不拥有服务端查询、分页或业务筛选语义。
- 长列表虚拟化为独立原语，不强迫所有语义表格使用绝对定位布局。
- ECharts 只在客户端动态加载，必须在卸载时释放实例并支持容器尺寸变化。
- Playwright 输出、截图、trace 和 HTML 报告全部位于仓库根 `.local/playwright`。
- 每个生产行为先写可观察的失败测试，再写最小实现。

---

### Task 1: 统一包管理并安装前端 P0 依赖

**Files:**
- Modify: `web/listingkit-ui/package.json`
- Modify: `web/listingkit-ui/pnpm-lock.yaml`
- Delete: `web/listingkit-ui/package-lock.json`

**Interfaces:**
- Consumes: 现有 pnpm lockfile 和前端脚本。
- Produces: `pnpm@11.19.0` 的唯一锁文件，以及 DataTable、VirtualList、EChart 和 E2E 后续任务使用的依赖。

- [ ] **Step 1: 声明 pnpm 和浏览器测试脚本**

在 `package.json` 顶层增加：

```json
"packageManager": "pnpm@11.19.0"
```

在 `scripts` 中增加：

```json
"test:e2e": "playwright test",
"test:a11y": "playwright test e2e/accessibility.spec.ts"
```

- [ ] **Step 2: 使用 pnpm 添加锁定依赖**

Run:

```powershell
pnpm add @tanstack/react-table@9.2.4 @tanstack/react-virtual@3.14.10 echarts@6.1.0
pnpm add -D @playwright/test@1.62.1 @axe-core/playwright@4.13.0
```

Expected: `package.json` 与 `pnpm-lock.yaml` 更新，安装命令退出码为 0。

- [ ] **Step 3: 删除 npm lockfile**

使用 `apply_patch` 删除 `web/listingkit-ui/package-lock.json`，不运行 `npm install`。

- [ ] **Step 4: 验证锁文件可复现**

Run:

```powershell
pnpm install --frozen-lockfile
pnpm exec playwright --version
pnpm typecheck
```

Expected: 三个命令均退出 0；Playwright 输出 `Version 1.62.1`。

- [ ] **Step 5: 提交**

```powershell
git diff --check
git add web/listingkit-ui/package.json web/listingkit-ui/pnpm-lock.yaml web/listingkit-ui/package-lock.json
git commit -m "build(web): adopt workbench foundation dependencies"
```

---

### Task 2: 建立统一 DataTable 与 VirtualList 原语

**Files:**
- Create: `web/listingkit-ui/src/components/ui/data-table.tsx`
- Create: `web/listingkit-ui/src/components/ui/data-table.test.tsx`
- Create: `web/listingkit-ui/src/components/ui/virtual-list.tsx`
- Create: `web/listingkit-ui/src/components/ui/virtual-list.test.tsx`

**Interfaces:**
- Consumes: `Table`、`cn`、TanStack Table v9 feature API 和 `useVirtualizer`。
- Produces: `DataTable<TData>`、`DataTableProps<TData>`、`createDataTableColumnHelper<TData>()`、`VirtualList<TItem>`、`VirtualListProps<TItem>`。

- [ ] **Step 1: 写 DataTable 排序与空状态失败测试**

创建 `data-table.test.tsx`：

```tsx
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import {
  DataTable,
  type DataTableColumnDef,
} from "@/components/ui/data-table";

type Product = { id: string; name: string; sales: number };

const columns: DataTableColumnDef<Product>[] = [
  { accessorKey: "name", header: "商品" },
  { accessorKey: "sales", header: "销量" },
];

describe("DataTable", () => {
  it("sorts rows through an accessible column button", async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        ariaLabel="商品数据"
        columns={columns}
        data={[
          { id: "2", name: "Zulu", sales: 2 },
          { id: "1", name: "Alpha", sales: 10 },
        ]}
        getRowId={(row) => row.id}
      />,
    );

    await user.click(screen.getByRole("button", { name: "商品" }));
    const rows = screen.getAllByRole("row").slice(1);
    expect(within(rows[0]).getByText("Alpha")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Zulu")).toBeInTheDocument();
  });

  it("renders one semantic empty row", () => {
    render(
      <DataTable
        ariaLabel="商品数据"
        columns={columns}
        data={[]}
        emptyMessage="暂无商品"
      />,
    );

    expect(screen.getByRole("table", { name: "商品数据" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "暂无商品" })).toHaveAttribute("colspan", "2");
  });
});
```

- [ ] **Step 2: 运行 DataTable 测试并确认 RED**

Run:

```powershell
pnpm test -- src/components/ui/data-table.test.tsx
```

Expected: FAIL，错误为无法解析 `@/components/ui/data-table`。

- [ ] **Step 3: 实现最小 DataTable**

创建 `data-table.tsx`：

```tsx
"use client";

import * as React from "react";
import {
  createColumnHelper,
  createSortedRowModel,
  flexRender,
  type ColumnDef,
  type Row,
  type RowData,
  rowSortingFeature,
  type SortingState,
  tableFeatures,
  useTable,
} from "@tanstack/react-table";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const dataTableFeatures = tableFeatures({
  rowSortingFeature,
  sortedRowModel: createSortedRowModel(),
});

export type DataTableColumnDef<TData extends RowData> = ColumnDef<
  typeof dataTableFeatures,
  TData,
  unknown
>;

export function createDataTableColumnHelper<TData extends RowData>() {
  return createColumnHelper<typeof dataTableFeatures, TData>();
}

export type DataTableProps<TData extends RowData> = {
  ariaLabel: string;
  columns: DataTableColumnDef<TData>[];
  data: TData[];
  emptyMessage?: string;
  getRowId?: (
    originalRow: TData,
    index: number,
    parent?: Row<typeof dataTableFeatures, TData>,
  ) => string;
};

export function DataTable<TData extends RowData>({
  ariaLabel,
  columns,
  data,
  emptyMessage = "暂无数据",
  getRowId,
}: DataTableProps<TData>) {
  const [sorting, setSorting] = React.useState<SortingState>([]);
  const table = useTable({
    features: dataTableFeatures,
    columns,
    data,
    getRowId,
    onSortingChange: setSorting,
    state: { sorting },
  });

  return (
    <Table aria-label={ariaLabel}>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header) => {
              const sorted = header.column.getIsSorted();
              return (
              <TableHead
                key={header.id}
                aria-sort={
                  sorted === "asc"
                    ? "ascending"
                    : sorted === "desc"
                      ? "descending"
                      : "none"
                }
              >
                {header.isPlaceholder ? null : header.column.getCanSort() ? (
                  <button
                    className="inline-flex min-h-9 items-center gap-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={header.column.getToggleSortingHandler()}
                    type="button"
                  >
                    {flexRender(header.column.columnDef.header, header.getContext())}
                    <span aria-hidden="true">
                      {header.column.getIsSorted() === "asc"
                        ? "↑"
                        : header.column.getIsSorted() === "desc"
                          ? "↓"
                          : "↕"}
                    </span>
                  </button>
                ) : (
                  flexRender(header.column.columnDef.header, header.getContext())
                )}
              </TableHead>
              );
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.length > 0 ? (
          table.getRowModel().rows.map((row) => (
            <TableRow key={row.id}>
              {row.getAllCells().map((cell) => (
                <TableCell key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))
        ) : (
          <TableRow>
            <TableCell className="h-24 text-center text-muted-foreground" colSpan={columns.length}>
              {emptyMessage}
            </TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}
```

- [ ] **Step 4: 运行 DataTable GREEN**

Run:

```powershell
pnpm test -- src/components/ui/data-table.test.tsx
```

Expected: 2 tests PASS。

- [ ] **Step 5: 写 VirtualList 窗口化失败测试**

创建 `virtual-list.test.tsx`：

```tsx
import { render, screen } from "@testing-library/react";
import { afterEach, vi } from "vitest";

import { VirtualList } from "@/components/ui/virtual-list";

afterEach(() => vi.restoreAllMocks());

describe("VirtualList", () => {
  it("renders a bounded window instead of every item", async () => {
    vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(160);
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(320);
    const items = Array.from({ length: 100 }, (_, index) => `任务 ${index + 1}`);
    render(
      <VirtualList
        ariaLabel="任务列表"
        estimateSize={32}
        height={160}
        items={items}
      >
        {(item) => <span>{item}</span>}
      </VirtualList>,
    );

    const renderedItems = await screen.findAllByRole("listitem");
    expect(renderedItems.length).toBeGreaterThan(0);
    expect(renderedItems.length).toBeLessThan(100);
    expect(screen.getByText("任务 1")).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: 运行 VirtualList 测试并确认 RED**

Run:

```powershell
pnpm test -- src/components/ui/virtual-list.test.tsx
```

Expected: FAIL，错误为无法解析 `@/components/ui/virtual-list`。

- [ ] **Step 7: 实现最小 VirtualList**

创建 `virtual-list.tsx`：

```tsx
"use client";

import * as React from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { cn } from "@/lib/utils/cn";

export type VirtualItemKey = string | number | bigint;

export type VirtualListProps<TItem> = {
  ariaLabel: string;
  children: (item: TItem, index: number) => React.ReactNode;
  className?: string;
  estimateSize: number;
  getItemKey?: (item: TItem, index: number) => VirtualItemKey;
  height: number;
  items: TItem[];
  overscan?: number;
};

export function VirtualList<TItem>({
  ariaLabel,
  children,
  className,
  estimateSize,
  getItemKey,
  height,
  items,
  overscan = 4,
}: VirtualListProps<TItem>) {
  "use no memo";

  const parentRef = React.useRef<HTMLDivElement>(null);
  // TanStack Virtual intentionally keeps interior mutable state; this component
  // is opted out of React Compiler memoization by the directive above.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: items.length,
    estimateSize: () => estimateSize,
    getItemKey: (index) => getItemKey?.(items[index], index) ?? index,
    getScrollElement: () => parentRef.current,
    initialRect: { height, width: 0 },
    overscan,
  });

  return (
    <div
      ref={parentRef}
      aria-label={ariaLabel}
      className={cn("overflow-auto", className)}
      role="list"
      style={{ height }}
    >
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((virtualItem) => (
          <div
            key={virtualItem.key}
            className="absolute left-0 top-0 w-full"
            role="listitem"
            style={{
              height: virtualItem.size,
              transform: `translateY(${virtualItem.start}px)`,
            }}
          >
            {children(items[virtualItem.index], virtualItem.index)}
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 8: 验证两个原语**

Run:

```powershell
pnpm test -- src/components/ui/data-table.test.tsx src/components/ui/virtual-list.test.tsx
pnpm typecheck
```

Expected: 3 tests PASS；TypeScript 退出 0。

- [ ] **Step 9: 提交**

```powershell
git diff --check
git add web/listingkit-ui/src/components/ui/data-table.tsx web/listingkit-ui/src/components/ui/data-table.test.tsx web/listingkit-ui/src/components/ui/virtual-list.tsx web/listingkit-ui/src/components/ui/virtual-list.test.tsx
git commit -m "feat(web): add data table and virtual list foundations"
```

---

### Task 3: 建立可释放的 ECharts 客户端边界

**Files:**
- Create: `web/listingkit-ui/src/components/ui/chart.tsx`
- Create: `web/listingkit-ui/src/components/ui/chart.test.tsx`

**Interfaces:**
- Consumes: `echarts.init`、`EChartsOption`、`ResizeObserver`。
- Produces: `EChart` 和 `EChartProps`；业务页面只传 option，不直接管理 ECharts 实例生命周期。

- [ ] **Step 1: 写 option 更新、resize 和 dispose 失败测试**

创建 `chart.test.tsx`，在文件顶部模拟 jsdom 不支持的外部图形运行时，但保留真实 React 生命周期：

```tsx
import { render, screen, waitFor } from "@testing-library/react";
import type { EChartsOption } from "echarts";
import { afterEach, vi } from "vitest";

const chart = vi.hoisted(() => ({
  dispose: vi.fn(),
  resize: vi.fn(),
  setOption: vi.fn(),
}));

vi.mock("echarts", () => ({
  init: vi.fn(() => chart),
}));

import { EChart } from "@/components/ui/chart";

afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("EChart", () => {
  it("updates the chart and disposes the instance on unmount", async () => {
    let notifyResize: (() => void) | undefined;
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          notifyResize = callback;
        }
        observe() {}
        disconnect() {}
      },
    );

    const first: EChartsOption = {
      xAxis: { type: "category" },
      series: [{ type: "bar", data: [1] }],
    };
    const second: EChartsOption = {
      xAxis: { type: "category" },
      series: [{ type: "bar", data: [2] }],
    };
    const view = render(<EChart ariaLabel="经营趋势" option={first} />);

    expect(screen.getByRole("img", { name: "经营趋势" })).toBeInTheDocument();
    await waitFor(() =>
      expect(chart.setOption).toHaveBeenCalledWith(first, { notMerge: true }),
    );
    notifyResize?.();
    expect(chart.resize).toHaveBeenCalledTimes(1);

    view.rerender(<EChart ariaLabel="经营趋势" option={second} />);
    await waitFor(() =>
      expect(chart.setOption).toHaveBeenLastCalledWith(second, { notMerge: true }),
    );

    view.unmount();
    expect(chart.dispose).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **Step 2: 运行测试并确认 RED**

Run:

```powershell
pnpm test -- src/components/ui/chart.test.tsx
```

Expected: FAIL，错误为无法解析 `@/components/ui/chart`。

- [ ] **Step 3: 实现动态客户端图表组件**

创建 `chart.tsx`：

```tsx
"use client";

import * as React from "react";
import type { EChartsOption } from "echarts";

import { cn } from "@/lib/utils/cn";

export type EChartProps = {
  ariaLabel: string;
  className?: string;
  option: EChartsOption;
};

export function EChart({ ariaLabel, className, option }: EChartProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const chartRef = React.useRef<import("echarts").ECharts | null>(null);
  const optionRef = React.useRef(option);

  React.useEffect(() => {
    optionRef.current = option;
    chartRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  React.useEffect(() => {
    let active = true;
    let observer: ResizeObserver | undefined;

    void import("echarts").then((echarts) => {
      if (!active || !containerRef.current) {
        return;
      }
      const chart = echarts.init(containerRef.current, undefined, { renderer: "svg" });
      chartRef.current = chart;
      chart.setOption(optionRef.current, { notMerge: true });
      observer = new ResizeObserver(() => chart.resize());
      observer.observe(containerRef.current);
    });

    return () => {
      active = false;
      observer?.disconnect();
      chartRef.current?.dispose();
      chartRef.current = null;
    };
  }, []);

  return (
    <div
      ref={containerRef}
      aria-label={ariaLabel}
      className={cn("min-h-72 w-full", className)}
      role="img"
    />
  );
}
```

- [ ] **Step 4: 验证图表组件**

Run:

```powershell
pnpm test -- src/components/ui/chart.test.tsx
pnpm typecheck
```

Expected: 测试 PASS；TypeScript 退出 0。如果动态 import mock 暴露实例类型不一致，只修正测试 fake 的完整结构，不削弱生命周期断言。

- [ ] **Step 5: 提交**

```powershell
git diff --check
git add web/listingkit-ui/src/components/ui/chart.tsx web/listingkit-ui/src/components/ui/chart.test.tsx
git commit -m "feat(web): add echarts lifecycle boundary"
```

---

### Task 4: 建立公开页面 Playwright 与 axe 门禁

**Files:**
- Create: `web/listingkit-ui/playwright.config.ts`
- Create: `web/listingkit-ui/e2e/public-site.spec.ts`
- Create: `web/listingkit-ui/e2e/accessibility.spec.ts`
- Modify: `web/listingkit-ui/.gitignore`

**Interfaces:**
- Consumes: 根路径公开营销页、Playwright webServer、axe-core 浏览器注入。
- Produces: `pnpm test:e2e` 与 `pnpm test:a11y`；所有运行产物写入仓库根 `.local/playwright`。

- [ ] **Step 1: 写 Playwright 配置**

创建 `playwright.config.ts`：

```ts
import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

const runtimeRoot = path.resolve(__dirname, "../../.local/playwright");

export default defineConfig({
  testDir: "./e2e",
  outputDir: path.join(runtimeRoot, "test-results"),
  reporter: [["line"], ["html", { outputFolder: path.join(runtimeRoot, "report"), open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:3210",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "pnpm dev --hostname 127.0.0.1 --port 3210",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: "http://127.0.0.1:3210",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
```

- [ ] **Step 2: 写公开页面行为测试**

创建 `e2e/public-site.spec.ts`：

```ts
import { expect, test } from "@playwright/test";

test("public homepage exposes the primary workbench entry", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1 })).toContainText("硕米智能引擎");
  await expect(page.getByRole("link", { name: "进入硕米" })).toHaveAttribute(
    "href",
    "/login?returnTo=%2Flisting-kits%2Fhome",
  );
});
```

- [ ] **Step 3: 写 axe 可访问性测试**

创建 `e2e/accessibility.spec.ts`：

```ts
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("public homepage has no automatically detectable serious violations", async ({ page }) => {
  await page.goto("/");
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();

  expect(
    results.violations.filter((violation) =>
      violation.impact === "serious" || violation.impact === "critical",
    ),
  ).toEqual([]);
});
```

- [ ] **Step 4: 确保运行产物被忽略**

若 `.gitignore` 尚未覆盖仓库根 `.local/`，只增加：

```gitignore
.local/
```

不要忽略 `e2e` 或 `playwright.config.ts`。

- [ ] **Step 5: 安装 Chromium 并运行 RED/GREEN 门禁**

Run:

```powershell
pnpm exec playwright install chromium
pnpm test:e2e
```

Expected: 首次运行若发现真实页面行为或严重可访问性问题，测试应先 RED 并列出具体 selector/rule；修复对应生产可访问性问题后重跑至 2 tests PASS。不得通过排除整个页面、禁用规则或放宽为不检查来变绿。

- [ ] **Step 6: 验证产物位置与前端回归**

Run:

```powershell
pnpm test:a11y
pnpm typecheck
pnpm test
pnpm build
git status --short
```

Expected: axe、TypeScript、1770 个既有 Vitest 加新增测试和生产构建全部成功；Git 状态不包含 `.local/playwright`。

- [ ] **Step 7: 提交**

```powershell
git diff --check
git add web/listingkit-ui/playwright.config.ts web/listingkit-ui/e2e web/listingkit-ui/.gitignore
git commit -m "test(web): add playwright accessibility gate"
```

---

## Completion Verification

- [ ] `pnpm install --frozen-lockfile`
- [ ] `pnpm typecheck`
- [ ] `pnpm test`
- [ ] `pnpm test:e2e`
- [ ] `pnpm build`
- [ ] `git diff --check`
- [ ] `git status --short` 只显示计划允许的状态，完成提交后为空

Promptfoo 属于独立 AI 评测子项目；Goose、OpenTelemetry、OpenFeature 属于后端阶段 2 计划。本计划不把这些互不依赖的系统混入同一提交序列。
