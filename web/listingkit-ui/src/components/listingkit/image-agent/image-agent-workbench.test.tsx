import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ImageAgentWorkbench } from "@/components/listingkit/image-agent/image-agent-workbench";
import type {
  ImageAgentProjection,
  ImageAgentSlot,
} from "@/lib/types/image-agent";

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  static active = 0;
  readonly url: string;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
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
}

const originalEventSource = globalThis.EventSource;

beforeEach(() => {
  FakeEventSource.instances = [];
  FakeEventSource.active = 0;
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
});

afterEach(() => {
  globalThis.EventSource = originalEventSource;
  vi.restoreAllMocks();
});

describe("ImageAgentWorkbench", () => {
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
    expect(within(sources).getByText("source-1")).toBeInTheDocument();
    expect(within(sources).queryByText("style-1")).not.toBeInTheDocument();
    expect(within(styles).getByText("style-1")).toBeInTheDocument();
    expect(within(styles).queryByText("source-1")).not.toBeInTheDocument();
    expect(within(sources).queryByAltText("scene-1 候选图 1")).not.toBeInTheDocument();
    expect(screen.getByAltText("scene-1 候选图 1")).toHaveAttribute(
      "src",
      "https://assets.example/scene-1.png",
    );
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

  it("keeps one action id when the user retries a failed slot command", async () => {
    const projection = projectionWithSlots(7, "scene-2");
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "provider unavailable" }), {
          status: 503,
          headers: { "content-type": "application/json" },
        }),
      )
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

    const button = screen.getByRole("button", { name: "仅重试 scene-2" });
    await user.click(button);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "provider unavailable",
    );
    await user.click(button);

    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(3));
    const commandCalls = fetchSpy.mock.calls.filter(
      (call) => call[1]?.method === "POST",
    );
    const first = JSON.parse(String(commandCalls[0]?.[1]?.body));
    const second = JSON.parse(String(commandCalls[1]?.[1]?.body));
    expect(commandCalls[0]?.[0]).toBe(
      "/api/listing-kits/image-agent/runs/run-1/slots/scene-2/retry",
    );
    expect(first).toMatchObject({ plan_revision: 3 });
    expect(first.action_id).toBeTruthy();
    expect(second.action_id).toBe(first.action_id);
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

  it("refetches a full snapshot on a version gap and never leaks parallel EventSources", async () => {
    const initial = projectionWithSlots(7);
    initial.run.version = 4;
    initial.last_event_id = 4;
    const recovered = structuredClone(initial);
    recovered.run.version = 7;
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
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(2));
    expect(FakeEventSource.instances[0]?.closed).toBe(true);
    expect(FakeEventSource.active).toBe(1);

    rendered.unmount();
    expect(FakeEventSource.active).toBe(0);
  });

  it("ignores duplicate or retrograde event cursors and projection versions", async () => {
    const initial = projectionWithSlots(7);
    initial.run.version = 4;
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

  it("resets an ahead browser cursor from the server snapshot after disconnect", async () => {
    const initial = projectionWithSlots(7);
    initial.run.version = 8;
    initial.last_event_id = 8;
    const recovered = structuredClone(initial);
    recovered.run.version = 4;
    recovered.last_event_id = 4;
    const advanced = structuredClone(recovered);
    advanced.run.version = 5;
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

    act(() => FakeEventSource.instances[1]?.emit(5, 5));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(3));
    expect(FakeEventSource.active).toBe(1);
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
  };
}
