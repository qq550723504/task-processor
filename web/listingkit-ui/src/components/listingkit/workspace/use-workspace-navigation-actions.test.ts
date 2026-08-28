import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  runSheinFreshnessAction,
  useWorkspaceNavigationActions,
} from "@/components/listingkit/workspace/use-workspace-navigation-actions";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  dispatchMutate: vi.fn(),
  actionMutate: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: mocks.push }),
}));

vi.mock("@/lib/query/use-dispatch", () => ({
  useDispatchNavigation: () => ({ mutate: mocks.dispatchMutate }),
}));

vi.mock("@/lib/query/use-action", () => ({
  useExecuteAction: () => ({ mutate: mocks.actionMutate }),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("runSheinFreshnessAction", () => {
  it("runs the matching freshness handler when available", () => {
    const handleRefreshCategory = vi.fn();

    const handled = runSheinFreshnessAction(
      "shein_category_template_freshness",
      {
        shein_category_template_freshness: handleRefreshCategory,
      },
    );

    expect(handled).toBe(true);
    expect(handleRefreshCategory).toHaveBeenCalledTimes(1);
  });

  it("falls back when the matching freshness handler is missing", () => {
    const handled = runSheinFreshnessAction(
      "shein_attribute_template_freshness",
      {},
    );

    expect(handled).toBe(false);
  });

  it("does not treat online auth as a local refresh action", () => {
    const handled = runSheinFreshnessAction("shein_online_auth", {
      shein_category_template_freshness: vi.fn(),
    });

    expect(handled).toBe(false);
  });
});

describe("useWorkspaceNavigationActions", () => {
  it("routes a canonical pricing issue to the mounted SHEIN final-review target", () => {
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleRunSheinPrimaryAction("pricing");
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&section_key=final_review#shein-final-review-pricing",
    );
  });

  it("persists history as a dedicated workspace route", () => {
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("platform=shein&section_key=general_review"),
      }),
    );

    act(() => {
      result.current.handleHistorySelect();
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?workspace_view=history",
    );
  });
});
