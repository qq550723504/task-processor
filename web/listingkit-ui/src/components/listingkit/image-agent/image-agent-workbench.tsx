"use client";

import { useState } from "react";

import { useImageAgentRun } from "@/components/listingkit/image-agent/use-image-agent-run";
import type {
  ImageAgentProjection,
  ImageAgentSlot,
  ImageAgentSlotProjection,
  ImageAgentSlotRole,
} from "@/lib/types/image-agent";

export function ImageAgentWorkbench({
  taskId,
  runId,
  initialRun,
}: {
  taskId: string;
  runId: string;
  initialRun?: ImageAgentProjection;
}) {
  const agent = useImageAgentRun({ runId, initialRun });
  const projection = agent.projection;

  if (agent.isLoading && !projection) {
    return (
      <section className="rounded-[1.75rem] border border-border bg-card p-6 shadow-sm">
        <p className="text-sm text-muted-foreground">正在加载图片 Agent 运行状态…</p>
      </section>
    );
  }

  if (!projection) {
    return (
      <section className="rounded-[1.75rem] border border-destructive/40 bg-destructive/5 p-6">
        <p role="alert" className="text-sm text-destructive">
          {agent.error ?? "图片 Agent 运行不存在或不可访问"}
        </p>
      </section>
    );
  }

  if (
    projection.run.business_task_id &&
    projection.run.business_task_id !== taskId
  ) {
    return (
      <section className="rounded-[1.75rem] border border-destructive/40 bg-destructive/5 p-6">
        <p role="alert" className="text-sm text-destructive">
          当前图片 Agent 运行不属于任务 {taskId}
        </p>
      </section>
    );
  }

  const slotProjectionByID = new Map(
    projection.slots.map((slot) => [slot.slot.id, slot]),
  );
  const blockedSlotID = projection.run.block?.slot_id;
  const blockedSlot = projection.plan.slots.find(
    (slot) => slot.id === blockedSlotID,
  );
  const canRetryBlockedSlot =
    Boolean(blockedSlotID) && projection.actions.includes("retry_slot");
  const canEditPlan = projection.actions.includes("edit_plan");
  const completedSlots = projection.slots.filter(
    (slot) => slot.slot.status === "accepted",
  ).length;

  return (
    <section className="space-y-4" aria-label="图片 Agent 工作台">
      <header className="rounded-[1.75rem] border border-border bg-card p-5 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-muted-foreground">
              手动图片 Agent
            </p>
            <h2 className="mt-1 text-2xl font-semibold tracking-tight text-foreground">
              按计划逐槽生成并恢复
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              已完成 {completedSlots}/{projection.plan.slots.length} 个槽位 · 计划修订 {projection.plan.revision}
            </p>
          </div>
          <span className={statusBadgeClass(projection.run.status)}>
            {runStatusLabel(projection.run.status)}
          </span>
        </div>
        {agent.error ? (
          <p
            role="alert"
            className="mt-4 rounded-xl border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive"
          >
            {agent.error}
          </p>
        ) : null}
      </header>

      <div className="grid min-w-0 items-start gap-4 xl:grid-cols-[17rem_minmax(0,1fr)_20rem]">
        <aside className="min-w-0 space-y-4">
          <AssetList
            title="商品来源素材"
            description="只用于确认商品身份，不会被当作生成成功结果。"
            ids={projection.plan.source_asset_ids}
            testId="image-agent-source-materials"
          />
          <AssetList
            title="风格参考"
            description="只影响表现方式，与商品来源素材严格分开。"
            ids={projection.plan.style_reference_ids ?? []}
            emptyLabel="当前计划未选择风格参考"
            testId="image-agent-style-references"
          />
        </aside>

        <PlanBoard
          key={projection.plan.revision}
          slots={projection.plan.slots}
          projections={slotProjectionByID}
          editable={canEditPlan}
          pending={Boolean(agent.pendingAction)}
          onSave={agent.replacePlan}
        />

        <aside className="min-w-0 space-y-4 xl:sticky xl:top-6">
          <section className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm">
            <h3 className="font-semibold text-foreground">运行状态</h3>
            <dl className="mt-3 space-y-3 text-sm">
              <Metric label="状态" value={runStatusLabel(projection.run.status)} />
              <Metric label="当前节点" value={projection.run.current_node || "—"} />
              <Metric
                label="图片预算"
                value={`${projection.run.usage.images}/${projection.run.budget.max_images}`}
              />
              <Metric
                label="模型调用"
                value={`${projection.run.usage.model_calls}/${projection.run.budget.max_model_calls}`}
              />
              <Metric
                label="预估成本"
                value={formatCost(projection.run.usage.estimated_cost_micros)}
              />
            </dl>
          </section>

          {projection.run.status === "blocked" ? (
            <section className="rounded-[1.5rem] border border-amber-400/60 bg-amber-50 p-4 text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
              <p className="text-xs font-semibold uppercase tracking-[0.18em]">
                需要处理
              </p>
              <h3 className="mt-2 font-semibold">
                {blockedSlot
                  ? `${slotRoleLabel(blockedSlot.role)} ${blockedSlot.id} 生成失败`
                  : projection.run.block?.message || "图片生成流程已阻断"}
              </h3>
              {projection.run.block?.message ? (
                <p className="mt-2 text-sm opacity-80">
                  {projection.run.block.message}
                </p>
              ) : null}
              {canRetryBlockedSlot && blockedSlotID ? (
                <button
                  type="button"
                  className="mt-4 inline-flex h-10 w-full items-center justify-center rounded-xl bg-amber-950 px-4 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-amber-200 dark:text-amber-950"
                  disabled={Boolean(agent.pendingAction)}
                  onClick={() => void agent.retrySlot(blockedSlotID)}
                >
                  仅重试 {blockedSlotID}
                </button>
              ) : null}
            </section>
          ) : null}

          <section className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm">
            <h3 className="font-semibold text-foreground">可执行操作</h3>
            <div className="mt-3 space-y-2">
              {projection.actions.includes("approve_results") ? (
                <button
                  type="button"
                  className="inline-flex h-10 w-full items-center justify-center rounded-xl bg-foreground px-4 text-sm font-medium text-background disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={!projection.result_digest || Boolean(agent.pendingAction)}
                  onClick={() => void agent.approveResults()}
                >
                  批准当前结果
                </button>
              ) : null}
              {projection.actions.includes("cancel") ? (
                <button
                  type="button"
                  className="inline-flex h-10 w-full items-center justify-center rounded-xl border border-border bg-background px-4 text-sm font-medium text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={Boolean(agent.pendingAction)}
                  onClick={() => void agent.cancel()}
                >
                  取消运行
                </button>
              ) : null}
              {projection.actions.length === 0 ? (
                <p className="text-sm text-muted-foreground">当前没有可执行操作。</p>
              ) : null}
            </div>
          </section>
        </aside>
      </div>
    </section>
  );
}

function PlanBoard({
  slots,
  projections,
  editable,
  pending,
  onSave,
}: {
  slots: ImageAgentSlot[];
  projections: Map<string, ImageAgentSlotProjection>;
  editable: boolean;
  pending: boolean;
  onSave: (slots: ImageAgentSlot[]) => Promise<void>;
}) {
  const [draftSlots, setDraftSlots] = useState(() => slots.map(cloneSlot));
  return (
    <main className="min-w-0 space-y-3">
      {slots.map((slot, index) => (
        <SlotCard
          key={slot.id}
          slot={slot}
          projection={projections.get(slot.id)}
          draft={draftSlots[index] ?? slot}
          editable={editable}
          onBriefChange={(brief) =>
            setDraftSlots((current) =>
              current.map((item, itemIndex) =>
                itemIndex === index ? { ...item, brief } : item,
              ),
            )
          }
        />
      ))}
      {editable ? (
        <button
          type="button"
          className="inline-flex h-10 items-center justify-center rounded-xl bg-foreground px-4 text-sm font-medium text-background disabled:cursor-not-allowed disabled:opacity-50"
          disabled={pending}
          onClick={() => void onSave(draftSlots)}
        >
          保存计划修改
        </button>
      ) : null}
    </main>
  );
}

function AssetList({
  title,
  description,
  ids,
  emptyLabel,
  testId,
}: {
  title: string;
  description: string;
  ids: string[];
  emptyLabel?: string;
  testId: string;
}) {
  return (
    <section
      data-testid={testId}
      className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm"
    >
      <h3 className="font-semibold text-foreground">{title}</h3>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      <ul className="mt-3 space-y-2">
        {ids.length > 0 ? (
          ids.map((id) => (
            <li
              key={id}
              className="break-all rounded-xl border border-border bg-background px-3 py-2 text-sm text-foreground"
            >
              {id}
            </li>
          ))
        ) : (
          <li className="text-sm text-muted-foreground">{emptyLabel}</li>
        )}
      </ul>
    </section>
  );
}

function SlotCard({
  slot,
  projection,
  draft,
  editable,
  onBriefChange,
}: {
  slot: ImageAgentSlot;
  projection?: ImageAgentSlotProjection;
  draft: ImageAgentSlot;
  editable: boolean;
  onBriefChange: (brief: string) => void;
}) {
  return (
    <article
      data-testid="image-slot-card"
      className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
            {slotRoleLabel(slot.role)}
          </p>
          <h3 className="mt-1 font-semibold text-foreground">{slot.id}</h3>
        </div>
        <span className="rounded-full border border-border bg-muted px-2.5 py-1 text-xs text-muted-foreground">
          {slotStatusLabel(projection?.slot.status ?? slot.status)}
        </span>
      </div>
      <div className="mt-3 grid gap-3 text-xs text-muted-foreground sm:grid-cols-2">
        <p>来源：{slot.source_asset_ids.join("、") || "未选择"}</p>
        <p>风格：{slot.style_reference_ids?.join("、") || "未选择"}</p>
      </div>
      <label className="mt-3 block text-sm text-foreground">
        <span className="mb-1 block text-xs font-medium text-muted-foreground">
          {slot.id} 场景说明
        </span>
        <textarea
          aria-label={`${slot.id} 场景说明`}
          className="min-h-20 w-full rounded-xl border border-border bg-background px-3 py-2 text-sm text-foreground disabled:opacity-70"
          disabled={!editable}
          value={draft.brief ?? ""}
          onChange={(event) => onBriefChange(event.target.value)}
        />
      </label>
      {projection?.candidates.length ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {projection.candidates.map((candidate, index) => (
            <figure
              key={candidate.asset_id}
              className="overflow-hidden rounded-xl border border-border bg-muted"
            >
              {/* eslint-disable-next-line @next/next/no-img-element -- provider asset hosts are tenant-configured at runtime */}
              <img
                alt={`${slot.id} 候选图 ${index + 1}`}
                className="aspect-square w-full object-cover"
                src={candidate.url}
              />
              <figcaption className="break-all px-3 py-2 text-xs text-muted-foreground">
                {candidate.asset_id}
              </figcaption>
            </figure>
          ))}
        </div>
      ) : projection?.error_code ? (
        <p className="mt-3 rounded-xl bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {projection.error_code}
        </p>
      ) : null}
    </article>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="break-all text-right font-medium text-foreground">{value}</dd>
    </div>
  );
}

function slotRoleLabel(role: ImageAgentSlotRole) {
  return {
    main: "主图",
    scene: "场景图",
    detail: "细节图",
    selling_point: "卖点图",
    size: "尺寸图",
  }[role];
}

function slotStatusLabel(status: ImageAgentSlot["status"]) {
  return {
    pending: "待执行",
    executing: "生成中",
    evaluating: "评估中",
    accepted: "已完成",
    rejected: "已拒绝",
    blocked: "已阻断",
  }[status];
}

function runStatusLabel(status: ImageAgentProjection["run"]["status"]) {
  return {
    planning: "计划中",
    awaiting_plan_approval: "等待计划确认",
    executing: "生成中",
    evaluating: "评估中",
    repairing: "修复中",
    awaiting_final_approval: "等待最终确认",
    blocked: "已阻断",
    completed: "已完成",
    failed: "失败",
    cancelled: "已取消",
  }[status];
}

function statusBadgeClass(status: ImageAgentProjection["run"]["status"]) {
  const base = "rounded-full border px-3 py-1 text-xs font-medium";
  if (status === "blocked" || status === "failed") {
    return `${base} border-amber-400/60 bg-amber-50 text-amber-900 dark:bg-amber-950/30 dark:text-amber-100`;
  }
  if (status === "completed") {
    return `${base} border-emerald-400/60 bg-emerald-50 text-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-100`;
  }
  return `${base} border-border bg-muted text-muted-foreground`;
}

function formatCost(micros: number) {
  return `${(micros / 1_000_000).toFixed(4)} 计费单位`;
}

function cloneSlot(slot: ImageAgentSlot): ImageAgentSlot {
  return {
    ...slot,
    source_asset_ids: [...slot.source_asset_ids],
    style_reference_ids: slot.style_reference_ids
      ? [...slot.style_reference_ids]
      : undefined,
  };
}
