import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ImageAgentWorkbench } from "@/components/listingkit/image-agent/image-agent-workbench";
import { parseImageAgentProjection } from "@/lib/api/image-agent";
import type {
  ImageAgentAction,
  ImageAgentProjection,
  ImageAgentSlot,
} from "@/lib/types/image-agent";

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  static active = 0;
  readonly url: string;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onopen: ((event: Event) => void) | null = null;
  closed = false;
  private listeners = new Map<string, EventListener>();

  constructor(url: string | URL) {
    this.url = String(url);
    FakeEventSource.instances.push(this);
    FakeEventSource.active += 1;
  }

  close() {
    if (!this.closed) {
      this.closed = true;
      FakeEventSource.active -= 1;
    }
  }

  addEventListener(type: string, listener: EventListener) {
    this.listeners.set(type, listener);
  }

  removeEventListener(type: string, listener: EventListener) {
    if (this.listeners.get(type) === listener) {
      this.listeners.delete(type);
    }
  }

  emit(id: number, projectionVersion: number) {
    this.listeners.get("projection")?.({
      data: JSON.stringify({
        schema_version: "image-agent.projection.v1",
        type: "run.updated",
        projection_version: projectionVersion,
      }),
      lastEventId: String(id),
    } as MessageEvent<string>);
  }

  fail() {
    this.onerror?.(new Event("error"));
  }

  open() {
    this.onopen?.(new Event("open"));
  }
}

const originalEventSource = globalThis.EventSource;

const blockedUnknownCases: Array<{
  name: string;
  code: string;
  guidance: string;
  initialActions: ImageAgentAction[];
  refreshedActions: ImageAgentAction[];
  initialCanCreate: boolean;
  refreshedCanCreate: boolean;
}> = [
  {
    name: "provider outcome unknown",
    code: "slot_provider_outcome_unknown",
    guidance: "生成结果状态不确定",
    initialActions: ["edit_plan", "retry_slot", "cancel"],
    refreshedActions: ["edit_plan", "cancel"],
    initialCanCreate: true,
    refreshedCanCreate: false,
  },
  {
    name: "staging outcome unknown",
    code: "slot_staging_outcome_unknown",
    guidance: "持久化字节状态不完整",
    initialActions: ["edit_plan", "cancel"],
    refreshedActions: ["edit_plan", "retry_slot", "cancel"],
    initialCanCreate: false,
    refreshedCanCreate: true,
  },
  {
    name: "publication outcome unknown",
    code: "slot_publication_outcome_unknown",
    guidance: "发布结果需要验证",
    initialActions: ["edit_plan", "retry_slot", "cancel"],
    refreshedActions: ["edit_plan", "retry_slot", "cancel"],
    initialCanCreate: false,
    refreshedCanCreate: false,
  },
];

beforeEach(() => {
  FakeEventSource.instances = [];
  FakeEventSource.active = 0;
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
});

afterEach(() => {
  vi.useRealTimers();
  globalThis.EventSource = originalEventSource;
  vi.restoreAllMocks();
});

describe("ImageAgentWorkbench", () => {
  it("parses legacy concurrency absence and strict positive persisted concurrency", () => {
    const legacy = projectionWithSlots(7);
    Reflect.deleteProperty(legacy.run, "max_concurrent_slots");
    const parsedLegacy = parseImageAgentProjection(legacy);
    expect(parsedLegacy.run.max_concurrent_slots).toBeUndefined();
    expect(Object.hasOwn(parsedLegacy.run, "max_concurrent_slots")).toBe(false);

    const current = projectionWithSlots(7);
    current.run.max_concurrent_slots = 6;
    expect(parseImageAgentProjection(current).run.max_concurrent_slots).toBe(6);

    for (const malformed of [0, -1, 1.5, "6"]) {
      const projection = projectionWithSlots(7);
      projection.run.max_concurrent_slots = malformed as number;
      expect(() => parseImageAgentProjection(projection)).toThrow();
    }
  });

  it("parses optional budget presence metadata and rejects unknown limit names", () => {
    const current = projectionWithSlots(7);
    Reflect.set(current.run.budget, "enabled_limits", ["max_images", "max_elapsed"]);
    expect(Reflect.get(parseImageAgentProjection(current).run.budget, "enabled_limits")).toEqual([
      "max_images",
      "max_elapsed",
    ]);

    const invalid = projectionWithSlots(7);
    Reflect.set(invalid.run.budget, "enabled_limits", ["provider_magic_tokens"]);
    expect(() => parseImageAgentProjection(invalid)).toThrow();
  });

  it.each([
    {
      name: "current disabled limits",
      enabledLimits: [] as string[],
      maxImages: 0,
      maxModelCalls: 0,
      expectedImages: "7/不限",
      expectedModelCalls: "7/不限",
    },
    {
      name: "current explicit zero limits",
      enabledLimits: ["max_images", "max_model_calls"],
      maxImages: 0,
      maxModelCalls: 0,
      expectedImages: "7/0",
      expectedModelCalls: "7/0",
    },
    {
      name: "legacy positive limits",
      enabledLimits: undefined,
      maxImages: 20,
      maxModelCalls: 20,
      expectedImages: "7/20",
      expectedModelCalls: "7/20",
    },
  ])("renders $name without treating disabled zero values as hard caps", ({
    enabledLimits,
    maxImages,
    maxModelCalls,
    expectedImages,
    expectedModelCalls,
  }) => {
    const projection = projectionWithSlots(7);
    projection.run.budget.max_images = maxImages;
    projection.run.budget.max_model_calls = maxModelCalls;
    if (enabledLimits === undefined) {
      Reflect.deleteProperty(projection.run.budget, "enabled_limits");
    } else {
      Reflect.set(projection.run.budget, "enabled_limits", enabledLimits);
    }

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);

    expect(screen.getByText("图片预算").parentElement).toHaveTextContent(expectedImages);
    expect(screen.getByText("模型调用").parentElement).toHaveTextContent(expectedModelCalls);
  });

  it("shows the exact blocked slot and keeps every planned slot", () => {
    const projection = projectionWithSlots(11, "scene-2");

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );

    expect(screen.getAllByTestId("image-slot-card")).toHaveLength(11);
    expect(screen.getByText("场景图 scene-2 生成失败")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "仅重试 scene-2" }),
    ).toBeEnabled();
  });

  it.each(blockedUnknownCases)("renders $name guidance from server actions on initial render and SSE refresh", async ({ code, guidance, initialActions, refreshedActions, initialCanCreate, refreshedCanCreate }) => {
    const initial = projectionWithSlots(7, "scene-2");
    initial.run.max_concurrent_slots = 3;
    initial.run.block = { code, message: "server detail before refresh", slot_id: "scene-2" };
    initial.actions = [...initialActions];
    initial.projection_version = 4;
    initial.last_event_id = 4;
    const refreshed = structuredClone(initial);
    refreshed.run.max_concurrent_slots = 6;
    refreshed.actions = [...refreshedActions];
    refreshed.run.block = { code, message: "server detail after refresh", slot_id: "scene-2" };
    refreshed.projection_version = 5;
    refreshed.last_event_id = 5;
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(refreshed), { status: 200, headers: { "content-type": "application/json" } }),
    );

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);

    expect(screen.getByText(guidance)).toBeInTheDocument();
    expect(screen.getByText("server detail before refresh")).toBeInTheDocument();
    expect(screen.getByText("有效并发").parentElement).toHaveTextContent("3 个槽位");
    if (initialCanCreate) {
      expect(screen.getByRole("button", { name: "创建新尝试" })).toBeEnabled();
    } else {
      expect(screen.queryByRole("button", { name: "创建新尝试" })).not.toBeInTheDocument();
    }

    act(() => FakeEventSource.instances[0]?.emit(5, 5));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    expect(screen.getByText(guidance)).toBeInTheDocument();
    expect(screen.getByText("server detail after refresh")).toBeInTheDocument();
    expect(screen.getByText("有效并发").parentElement).toHaveTextContent("6 个槽位");
    if (refreshedCanCreate) {
      expect(screen.getByRole("button", { name: "创建新尝试" })).toBeEnabled();
    } else {
      expect(screen.queryByRole("button", { name: "创建新尝试" })).not.toBeInTheDocument();
    }
  });

  it("commits legacy absence and a later supplied concurrency across SSE refreshes", async () => {
    const initial = projectionWithSlots(7);
    initial.run.max_concurrent_slots = 3;
    initial.projection_version = 4;
    initial.last_event_id = 4;
    const legacy = structuredClone(initial);
    Reflect.deleteProperty(legacy.run, "max_concurrent_slots");
    legacy.projection_version = 5;
    legacy.last_event_id = 5;
    const current = structuredClone(legacy);
    current.run.max_concurrent_slots = 5;
    current.projection_version = 6;
    current.last_event_id = 6;
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify(legacy), { status: 200, headers: { "content-type": "application/json" } }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify(current), { status: 200, headers: { "content-type": "application/json" } }),
      );

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);
    expect(screen.getByText("有效并发").parentElement).toHaveTextContent("3 个槽位");
    const source = FakeEventSource.instances[0]!;

    act(() => source.emit(5, 5));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    expect(screen.getByText("有效并发").parentElement).toHaveTextContent("未提供");
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(source.closed).toBe(false);

    act(() => source.emit(6, 6));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(screen.getByText("有效并发").parentElement).toHaveTextContent("5 个槽位");
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(source.closed).toBe(false);
  });

  it("shows durable safe command failure details after refresh", () => {
    const projection = projectionWithSlots(7, "scene-2");
    projection.pending_command = {
      action_id: "retry-failed",
      kind: "retry_slot",
      phase: "retry.persist_result",
      status: "pending",
      plan_revision: 3,
      slot_id: "scene-2",
      failure_code: "persistence_failed",
      failure_category: "persistence",
      failure_message: "运行状态保存暂时失败",
      last_failed_at: "2026-08-27T00:00:00Z",
      attempt: 2,
    };

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);

    expect(screen.getByText("运行状态保存暂时失败")).toBeInTheDocument();
    expect(screen.getByText(/persistence_failed/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复上次操作" })).toBeEnabled();
  });

  it("shows durable command exhaustion but preserves terminal cancellation", () => {
    const projection = projectionWithSlots(7, "scene-2");
    projection.command_ingress = {
      used: 1024,
      limit: 1024,
      exhausted: true,
      reason: "command_capacity_exhausted",
    };
    projection.pending_command = {
      action_id: "retry-at-cap",
      kind: "retry_slot",
      phase: "retry.persist_result",
      status: "pending",
      plan_revision: 3,
      slot_id: "scene-2",
      failure_code: "persistence_failed",
    };

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);

    expect(screen.getByRole("alert")).toHaveTextContent("普通命令容量已耗尽");
    expect(screen.getByRole("button", { name: "仅重试 scene-2" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "保存计划修改" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "取消运行" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "恢复上次操作" })).toBeEnabled();
  });

  it("restarts a failed run through its stored immutable inputs", async () => {
    const projection = projectionWithSlots(7);
    projection.run.status = "failed";
    projection.run.current_node = "workflow_failed";
    projection.actions = ["restart"];
    const restarted = structuredClone(projection);
    restarted.run.status = "planning";
    restarted.run.current_node = "plan";
    restarted.actions = [];
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(restarted), {
        status: 200, headers: { "content-type": "application/json" },
      }));
    const user = userEvent.setup();

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);
    expect(FakeEventSource.instances).toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "重新启动失败运行" }));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(fetchSpy.mock.calls[0]?.[0]).toBe("/api/listing-kits/image-agent/runs/run-1/restart");
    expect(fetchSpy.mock.calls[0]?.[1]).toMatchObject({ method: "POST" });
    expect(screen.getAllByText("计划中")).toHaveLength(2);
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  it("keeps source materials and style references separate from generated candidates", () => {
    const projection = projectionWithSlots(7, "scene-2");

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );

    const sources = screen.getByTestId("image-agent-source-materials");
    const styles = screen.getByTestId("image-agent-style-references");
    expect(within(sources).getByRole("checkbox", { name: "来源 source-1" })).toBeInTheDocument();
    expect(within(sources).queryByRole("checkbox", { name: "风格 style-1" })).not.toBeInTheDocument();
    expect(within(styles).getByRole("checkbox", { name: "风格 style-1" })).toBeInTheDocument();
    expect(within(styles).queryByRole("checkbox", { name: "来源 source-1" })).not.toBeInTheDocument();
    expect(within(sources).queryByAltText("scene-1 候选图 1")).not.toBeInTheDocument();
    expect(screen.getByAltText("scene-1 候选图 1")).toHaveAttribute(
      "src",
      "https://assets.example/scene-1.png",
    );
  });

  it("does not render unsafe provider candidate URLs", () => {
    const projection = projectionWithSlots(1);
    projection.slots[0]!.candidates[0]!.url = "javascript:alert(document.domain)";

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);

    expect(screen.queryByAltText("main-1 候选图 1")).not.toBeInTheDocument();
    expect(screen.getByText("candidate-main-1")).toBeInTheDocument();
  });

  it("states that style references are unavailable when the run catalog has no styles", () => {
    const projection = projectionWithSlots(1);
    projection.asset_catalog = projection.asset_catalog.filter((asset) => asset.type === "source");
    projection.plan.style_reference_ids = [];
    projection.plan.slots[0]!.style_reference_ids = [];

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);

    expect(within(screen.getByTestId("image-agent-style-references")).getByText("当前任务没有可用风格库/风格参考不可用")).toBeInTheDocument();
    expect(within(screen.getAllByTestId("image-slot-card")[0]!).getByText("当前任务没有可用风格库/风格参考不可用")).toBeInTheDocument();
  });

  it("refuses to render a run that belongs to another business task", () => {
    const projection = projectionWithSlots(7);
    projection.run.business_task_id = "task-other";

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "当前图片 Agent 运行不属于任务 task-1",
    );
    expect(screen.queryByTestId("image-slot-card")).not.toBeInTheDocument();
  });

  it("fails closed when the run projection has no business task owner", () => {
    const projection = projectionWithSlots(7);
    projection.run.business_task_id = "";
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);
    expect(screen.getByRole("alert")).toHaveTextContent("缺少业务任务归属");
    expect(screen.queryByTestId("image-slot-card")).not.toBeInTheDocument();
  });

  it("recovers an ambiguous command from the server receipt after a client reload", async () => {
    const projection = projectionWithSlots(7, "scene-2");
    const pending = structuredClone(projection);
    pending.pending_command = {
      action_id: "server-action-1",
      kind: "retry_slot",
      phase: "persist_transition",
      status: "pending",
      plan_revision: 3,
      slot_id: "scene-2",
    };
    const completed = structuredClone(projection);
    completed.pending_command = undefined;
    completed.run.status = "executing";
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "provider unavailable" }), {
          status: 503,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify(pending), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify(completed), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    const user = userEvent.setup();

    const firstClient = render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );

    const button = screen.getByRole("button", { name: "仅重试 scene-2" });
    await user.click(button);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "provider unavailable",
    );
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    firstClient.unmount();

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={pending}
      />,
    );
    await user.click(screen.getByRole("button", { name: "恢复上次操作" }));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(4));
    const commandCalls = fetchSpy.mock.calls.filter(
      (call) => call[1]?.method === "POST",
    );
    expect(commandCalls[0]?.[0]).toBe(
      "/api/listing-kits/image-agent/runs/run-1/slots/scene-2/retry",
    );
    expect(commandCalls[1]?.[0]).toBe(
      "/api/listing-kits/image-agent/runs/run-1/commands/server-action-1/resume",
    );
    expect(commandCalls[1]?.[1]?.body).toBeUndefined();
  });

  it("builds a plan only from controlled catalog selections and supports eleven slots", async () => {
    const projection = projectionWithSlots(10, "scene-2");
    projection.actions = ["edit_plan", "retry_slot", "cancel"];
    const updated = structuredClone(projection);
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(updated), {
        status: 200, headers: { "content-type": "application/json" },
      }));
    const user = userEvent.setup();

    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);
    await user.click(within(screen.getByTestId("image-agent-source-materials")).getByRole("checkbox", { name: "来源 source-9" }));
    await user.click(within(screen.getByTestId("image-agent-style-references")).getByRole("checkbox", { name: "风格 style-2" }));
    const sceneOne = screen.getByRole("heading", { name: "scene-1" }).closest("article");
    expect(sceneOne).not.toBeNull();
    await user.click(within(sceneOne!).getByRole("checkbox", { name: "来源 source-2" }));
    await user.click(within(sceneOne!).getByRole("checkbox", { name: "风格 style-2" }));
    await user.click(screen.getByRole("button", { name: "删除 scene-9" }));
    await user.click(screen.getByRole("button", { name: "新增槽位" }));
    await user.click(screen.getByRole("button", { name: "新增槽位" }));
    expect(screen.getAllByTestId("image-slot-card")).toHaveLength(11);
    await user.click(screen.getByRole("button", { name: "保存计划修改" }));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    const put = fetchSpy.mock.calls.find((call) => call[1]?.method === "PUT");
    const body = JSON.parse(String(put?.[1]?.body));
    expect(body.expected_revision).toBe(3);
    expect(body.plan.source_asset_ids).not.toContain("source-9");
    expect(body.plan.style_reference_ids).toContain("style-2");
    expect(body.plan.slots).toHaveLength(11);
    expect(body.plan.slots.flatMap((slot: ImageAgentSlot) => slot.source_asset_ids)).not.toContain("source-9");
    const sceneOnePlan = body.plan.slots.find((slot: ImageAgentSlot) => slot.id === "scene-1");
    expect(sceneOnePlan.source_asset_ids).not.toContain("source-2");
    expect(sceneOnePlan.style_reference_ids).toContain("style-2");
  });

  it("never renders unsafe catalog display URLs", () => {
    const projection = projectionWithSlots(7);
    projection.asset_catalog.push({
      id: "unsafe-source", type: "source", label: "unsafe", display_url: "javascript:alert(1)",
    });
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);
    expect(screen.queryByRole("img", { name: "unsafe" })).not.toBeInTheDocument();
  });

  it("sends the current plan revision and exact projection digest for final approval", async () => {
    const projection = projectionWithSlots(7);
    projection.run.status = "awaiting_final_approval";
    projection.run.current_node = "approve_results";
    projection.actions = ["approve_results", "cancel"];
    projection.result_digest = "sha256:server-owned-digest";
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify(projection), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    const user = userEvent.setup();

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );
    await user.click(screen.getByRole("button", { name: "批准当前结果" }));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    const commandCall = fetchSpy.mock.calls.find(
      (call) => call[1]?.method === "POST",
    );
    const body = JSON.parse(String(commandCall?.[1]?.body));
    expect(body).toMatchObject({
      plan_revision: 3,
      result_digest: "sha256:server-owned-digest",
    });
  });

  it("creates a plan revision from manual brief edits using expected_revision", async () => {
    const projection = projectionWithSlots(7, "scene-2");
    projection.actions = ["edit_plan", "retry_slot", "cancel"];
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify(projection), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    const user = userEvent.setup();

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={projection}
      />,
    );
    const brief = screen.getByLabelText("scene-1 场景说明");
    await user.clear(brief);
    await user.type(brief, "放在自然光客厅中");
    await user.click(screen.getByRole("button", { name: "保存计划修改" }));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    const commandCall = fetchSpy.mock.calls.find(
      (call) => call[1]?.method === "PUT",
    );
    const body = JSON.parse(String(commandCall?.[1]?.body));
    expect(body.expected_revision).toBe(3);
    expect(body.plan).toMatchObject({
      revision: 4,
      parent_revision: 3,
    });
    expect(
      body.plan.slots.find((slot: ImageAgentSlot) => slot.id === "scene-1")
        ?.brief,
    ).toBe("放在自然光客厅中");
  });

  it("refetches a full snapshot on a version gap without replacing the healthy EventSource", async () => {
    const initial = projectionWithSlots(7);
    initial.projection_version = 4;
    initial.last_event_id = 4;
    const recovered = structuredClone(initial);
    recovered.projection_version = 7;
    recovered.last_event_id = 7;
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response(JSON.stringify(recovered), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );

    const rendered = render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={initial}
      />,
    );
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.active).toBe(1);

    act(() => FakeEventSource.instances[0]?.emit(7, 7));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.instances[0]?.closed).toBe(false);
    expect(FakeEventSource.active).toBe(1);

    rendered.unmount();
    expect(FakeEventSource.active).toBe(0);
  });

  it("does not subscribe for terminal runs and closes an active stream after a terminal snapshot", async () => {
    const completed = projectionWithSlots(7);
    completed.run.status = "completed";
    const rendered = render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={completed} />);
    expect(FakeEventSource.instances).toHaveLength(0);
    rendered.unmount();

    const executing = projectionWithSlots(7);
    executing.run.status = "executing";
    const failed = structuredClone(executing);
    failed.run.status = "failed";
    failed.projection_version += 1;
    failed.last_event_id += 1;
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(failed), { status: 200, headers: { "content-type": "application/json" } }),
    );
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={executing} />);
    expect(FakeEventSource.active).toBe(1);

    act(() => FakeEventSource.instances[0]?.emit(failed.last_event_id, failed.projection_version));

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(FakeEventSource.active).toBe(0));
  });

  it("ignores duplicate or retrograde event cursors and projection versions", async () => {
    const initial = projectionWithSlots(7);
    initial.projection_version = 4;
    initial.last_event_id = 4;
    const fetchSpy = vi.spyOn(globalThis, "fetch");

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={initial}
      />,
    );
    act(() => {
      FakeEventSource.instances[0]?.emit(4, 5);
      FakeEventSource.instances[0]?.emit(5, 4);
    });

    await new Promise((resolve) => setTimeout(resolve, 80));
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(FakeEventSource.active).toBe(1);
  });

  it("refetches every consecutive slot-result cursor even when run.version does not change", async () => {
    const initial = projectionWithSlots(7);
    const slotSix = structuredClone(initial);
    slotSix.last_event_id = 6;
    slotSix.projection_version = 6;
    const slotSeven = structuredClone(initial);
    slotSeven.last_event_id = 7;
    slotSeven.projection_version = 7;
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify(slotSix), { status: 200, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(slotSeven), { status: 200, headers: { "content-type": "application/json" } }));
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);
    const source = FakeEventSource.instances[0]!;
    act(() => source.emit(6, 6));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    act(() => source.emit(7, 7));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(slotSix.run.version).toBe(slotSeven.run.version);
    expect(FakeEventSource.instances).toHaveLength(1);
    expect(source.closed).toBe(false);
  });

  it("queues a projection refresh that arrives while snapshot recovery is in flight", async () => {
    const initial = projectionWithSlots(7);
    initial.projection_version = 5;
    initial.last_event_id = 5;
    const recoveredSix = structuredClone(initial);
    recoveredSix.projection_version = 6;
    recoveredSix.last_event_id = 6;
    const recoveredSeven = structuredClone(initial);
    recoveredSeven.projection_version = 7;
    recoveredSeven.last_event_id = 7;
    let resolveFirst!: (response: Response) => void;
    const firstSnapshot = new Promise<Response>((resolve) => { resolveFirst = resolve; });
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockReturnValueOnce(firstSnapshot)
      .mockResolvedValueOnce(new Response(JSON.stringify(recoveredSeven), { status: 200, headers: { "content-type": "application/json" } }));
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);
    const source = FakeEventSource.instances[0]!;

    act(() => source.emit(6, 6));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(1));
    act(() => source.emit(7, 7));
    expect(fetchSpy).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveFirst(new Response(JSON.stringify(recoveredSix), { status: 200, headers: { "content-type": "application/json" } }));
      await firstSnapshot;
    });
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(source.closed).toBe(false);
  });

  it("resets an ahead browser cursor from the server snapshot after disconnect", async () => {
    const initial = projectionWithSlots(7);
    initial.projection_version = 8;
    initial.last_event_id = 8;
    const recovered = structuredClone(initial);
    recovered.projection_version = 4;
    recovered.last_event_id = 4;
    const advanced = structuredClone(recovered);
    advanced.projection_version = 5;
    advanced.last_event_id = 5;
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify(recovered), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify(advanced), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );

    render(
      <ImageAgentWorkbench
        taskId="task-1"
        runId="run-1"
        initialRun={initial}
      />,
    );
    act(() => FakeEventSource.instances[0]?.fail());
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    expect(FakeEventSource.instances[1]?.url).toBe("/api/listing-kits/image-agent/runs/run-1/events?after_cursor=4");

    act(() => FakeEventSource.instances[1]?.emit(5, 5));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(FakeEventSource.instances).toHaveLength(2);
    expect(FakeEventSource.instances[1]?.closed).toBe(false);
    expect(FakeEventSource.active).toBe(1);
  });

  it("backs off exponentially after snapshot network failures and resets after success", async () => {
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const initial = projectionWithSlots(7);
    const recovered = structuredClone(initial);
    const fetchSpy = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "down" }), { status: 503, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "still down" }), { status: 503, headers: { "content-type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(recovered), { status: 200, headers: { "content-type": "application/json" } }));
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);

    await act(async () => { FakeEventSource.instances[0]?.fail(); await Promise.resolve(); });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(499); });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(fetchSpy).toHaveBeenCalledTimes(2);
    await act(async () => { await vi.advanceTimersByTimeAsync(999); });
    expect(fetchSpy).toHaveBeenCalledTimes(2);
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(fetchSpy).toHaveBeenCalledTimes(3);
	await act(async () => { await vi.advanceTimersByTimeAsync(500); });
    expect(FakeEventSource.active).toBe(1);
  });

  it("backs off EventSource failures even when every recovery snapshot succeeds", async () => {
    vi.useFakeTimers();
    vi.spyOn(Math, "random").mockReturnValue(0.5);
    const initial = projectionWithSlots(7);
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(initial), { status: 200, headers: { "content-type": "application/json" } }),
    );
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={initial} />);

    await act(async () => { FakeEventSource.instances[0]?.fail(); await Promise.resolve(); await Promise.resolve(); });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(FakeEventSource.instances).toHaveLength(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(499); });
    expect(FakeEventSource.instances).toHaveLength(1);
	await act(async () => { await vi.runOnlyPendingTimersAsync(); });
    expect(FakeEventSource.instances).toHaveLength(2);

    await act(async () => { FakeEventSource.instances[1]?.fail(); await Promise.resolve(); await Promise.resolve(); });
    expect(fetchSpy).toHaveBeenCalledTimes(2);
	await act(async () => { await vi.advanceTimersByTimeAsync(0); await Promise.resolve(); await Promise.resolve(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(999); });
    expect(FakeEventSource.instances).toHaveLength(2);
	await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(FakeEventSource.instances).toHaveLength(3);
    expect(FakeEventSource.active).toBe(1);
  });

  it("isolates delayed snapshots and stale EventSource callbacks when runId changes", async () => {
    const runOne = projectionWithSlots(1);
    const runTwo = structuredClone(runOne);
    runTwo.run.id = "run-2";
    runTwo.slots[0]!.candidates[0]!.url = "https://assets.example/run-2.png";
    let resolveRunOne!: (response: Response) => void;
    vi.spyOn(globalThis, "fetch").mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveRunOne = resolve; }));
    const rendered = render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={runOne} />);
    await act(async () => { FakeEventSource.instances[0]?.fail(); await Promise.resolve(); });

    rendered.rerender(<ImageAgentWorkbench taskId="task-1" runId="run-2" initialRun={runTwo} />);
    expect(screen.getByAltText("main-1 候选图 1")).toHaveAttribute("src", "https://assets.example/run-2.png");
    const oldSource = FakeEventSource.instances[0]!;
    const instanceCountAfterSwitch = FakeEventSource.instances.length;

    await act(async () => {
      resolveRunOne(new Response(JSON.stringify(runOne), { status: 200, headers: { "content-type": "application/json" } }));
      await Promise.resolve();
      oldSource.fail();
      await Promise.resolve();
    });

    expect(screen.getByAltText("main-1 候选图 1")).toHaveAttribute("src", "https://assets.example/run-2.png");
    expect(FakeEventSource.instances).toHaveLength(instanceCountAfterSwitch);
    expect(FakeEventSource.active).toBe(1);
  });

  it("stops reconnecting on authentication failures", async () => {
    vi.useFakeTimers();
    const projection = projectionWithSlots(7);
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ message: "denied" }), { status: 401, headers: { "content-type": "application/json" } }),
    );
    render(<ImageAgentWorkbench taskId="task-1" runId="run-1" initialRun={projection} />);
    await act(async () => {
      FakeEventSource.instances[0]?.fail();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(screen.getByRole("alert")).toHaveTextContent("需要重新认证");
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(FakeEventSource.active).toBe(0);
  });
});

function projectionWithSlots(
  count: number,
  blockedSlotId?: string,
): ImageAgentProjection {
  const slots = Array.from({ length: count }, (_, index) => {
    const id = index === 0 ? "main-1" : `scene-${index}`;
    return {
      id,
      role: index === 0 ? "main" : "scene",
      source_asset_ids: [`source-${(index % 9) + 1}`],
      style_reference_ids: ["style-1"],
      brief: `brief ${id}`,
      idempotency_key: `slot-key-${id}`,
      status: id === blockedSlotId ? "blocked" : "accepted",
    } satisfies ImageAgentSlot;
  });
  return {
    run: {
      id: "run-1",
      business_task_id: "task-1",
      tenant_id: "tenant-a",
      user_id: "user-a",
      mode: "manual",
      idempotency_key: "run-key-1",
      status: blockedSlotId ? "blocked" : "executing",
      current_node: blockedSlotId ? "retry_slot" : "execute_slots",
      active_plan_revision: 3,
      version: 5,
      max_concurrent_slots: 4,
      budget: {
        max_images: 20,
        max_agent_steps: 0,
        max_model_calls: 20,
        max_repair_attempts_per_slot: 2,
        max_cost_micros: 1_000_000,
        max_elapsed: 60_000_000_000,
      },
      usage: {
        images: count - (blockedSlotId ? 1 : 0),
        agent_steps: 0,
        model_calls: count,
        estimated_cost_micros: 70_000,
        elapsed: 4_000_000_000,
      },
      block: blockedSlotId
        ? {
            code: "slot_failed",
            message: "provider failed",
            slot_id: blockedSlotId,
          }
        : undefined,
    },
    plan: {
      revision: 3,
      parent_revision: 2,
      idempotency_key: "plan-key-3",
      source_asset_ids: Array.from({ length: 9 }, (_, index) =>
        `source-${index + 1}`,
      ),
      style_reference_ids: ["style-1"],
      slots,
      created_by: "user-a",
    },
    slots: slots.map((slot) => ({
      slot,
      attempt: 1,
      candidates:
        slot.status === "blocked"
          ? []
          : [
              {
                asset_id: `candidate-${slot.id}`,
                url: `https://assets.example/${slot.id}.png`,
                source_asset_id: slot.source_asset_ids[0],
              },
            ],
      error_code: slot.status === "blocked" ? "provider_failed" : undefined,
    })),
    result_digest: blockedSlotId ? "" : "sha256:ready",
    actions: blockedSlotId
      ? ["edit_plan", "retry_slot", "cancel"]
      : ["cancel"],
    last_event_id: 5,
    projection_version: 5,
    asset_catalog: [
      ...Array.from({ length: 9 }, (_, index) => ({
        id: `source-${index + 1}`,
        type: "source" as const,
        label: `来源 source-${index + 1}`,
        display_url: `https://source.example/source-${index + 1}.png`,
      })),
      { id: "style-1", type: "style", label: "风格 style-1", display_url: "https://style.example/1.png" },
      { id: "style-2", type: "style", label: "风格 style-2", display_url: "https://style.example/2.png" },
    ],
  };
}
