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

  it("routes action recovery out of History even when the platform is already selected", () => {
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("workspace_view=history"),
      }),
    );

    act(() => {
      result.current.handlePlatformRecovery(
        {
          recovery_target: {
            dispatch_kind: "action",
            action_target: { action_key: "retry_dispatch" },
          },
        },
        "shein",
      );
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein",
    );
  });

  it("routes action recovery out of a product section when the platform is already selected", () => {
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=images"),
      }),
    );

    act(() => {
      result.current.handlePlatformRecovery(
        {
          recovery_target: {
            dispatch_kind: "action",
            action_target: { action_key: "retry_dispatch" },
          },
        },
        "shein",
      );
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein",
    );
  });

  it("routes an overview review action to its returned focused target", () => {
    mocks.actionMutate.mockImplementation((_request, options) => {
      options?.onSuccess?.({
        review_session: {
          focused_target: {
            platform: "shein",
            slot: "main",
            capability: "detail_preview",
            section_key: "detail_preview-main",
          },
        },
      });
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleAction({ action_key: "review_detail_previews" });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview&section_key=detail_preview-main",
    );
  });

  it("routes a protected action destination from its resolved navigation target", () => {
    mocks.actionMutate.mockImplementation((_request, options) => {
      options?.onSuccess?.({
        resolved_target: {
          queue_query: {
            platform: "shein",
            slot: "main",
            preview_capability: "detail_preview",
          },
        },
      });
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleAction({ action_key: "review_detail_previews" });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview",
    );
  });

  it("routes a product recovery dispatch to its returned review target", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({
        review_session: {
          session: {
            focused_target: {
              platform: "shein",
              slot: "main",
              capability: "detail_preview",
              section_key: "detail_preview-main",
            },
          },
        },
      });
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleRecovery({
        recovery_target: {
          dispatch_kind: "review_session",
          session_query: {
            platform: "shein",
            slot: "main",
            preview_capability: "detail_preview",
          },
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview&section_key=detail_preview-main",
    );
  });

  it("uses the recovery descriptor when a product dispatch returns no focused target", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({});
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleRecovery({
        recovery_target: {
          dispatch_kind: "review_session",
          session_query: {
            platform: "shein",
            slot: "main",
            preview_capability: "detail_preview",
          },
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview",
    );
  });

  it("derives a product recovery route from an action navigation target", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({});
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleRecovery({
        recovery_target: {
          dispatch_kind: "action",
          action_target: {
            navigation_target: {
              dispatch_kind: "review_session",
              session_query: {
                platform: "shein",
                slot: "main",
                preview_capability: "detail_preview",
              },
            },
          },
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview",
    );
  });

  it("derives a product recovery route from an action queue query", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({});
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams("product_section=overview"),
      }),
    );

    act(() => {
      result.current.handleRecovery({
        recovery_target: {
          dispatch_kind: "action",
          action_target: {
            queue_query: {
              platform: "shein",
              slot: "main",
              preview_capability: "detail_preview",
            },
          },
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&slot=main&preview_capability=detail_preview",
    );
  });

  it("routes an explicit dispatch from general review to its focused target", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({
        panel_update: {
          focused_target: {
            platform: "shein",
            slot: "gallery",
            capability: "detail_preview",
            section_key: "detail_preview-gallery",
          },
        },
      });
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams(
          "platform=shein&section_key=general_review",
        ),
      }),
    );

    act(() => {
      result.current.dispatchTarget({
        dispatch_kind: "review_session",
        session_query: {
          platform: "shein",
          slot: "gallery",
          preview_capability: "detail_preview",
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&section_key=detail_preview-gallery&slot=gallery&preview_capability=detail_preview",
    );
  });

  it("routes an explicit dispatch from final review to its focused target", () => {
    mocks.dispatchMutate.mockImplementation((_target, options) => {
      options?.onSuccess?.({
        panel_update: {
          focused_target: {
            platform: "shein",
            slot: "gallery",
            capability: "detail_preview",
            section_key: "detail_preview-gallery",
          },
        },
      });
    });
    const { result } = renderHook(() =>
      useWorkspaceNavigationActions({
        taskId: "task-1",
        baseQuery: {},
        searchParams: new URLSearchParams(
          "platform=shein&section_key=final_review",
        ),
      }),
    );

    act(() => {
      result.current.dispatchTarget({
        dispatch_kind: "review_session",
        session_query: {
          platform: "shein",
          slot: "gallery",
          preview_capability: "detail_preview",
        },
      });
    });

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-1/workspace?platform=shein&section_key=detail_preview-gallery&slot=gallery&preview_capability=detail_preview",
    );
  });
});
