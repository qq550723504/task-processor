"use client";

import { useState } from "react";

import { useImageAgentRun } from "@/components/listingkit/image-agent/use-image-agent-run";
import type {
  ImageAgentProjection,
  ImageAgentAuthorizedAsset,
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
  return <ImageAgentWorkbenchSession key={runId} taskId={taskId} runId={runId} initialRun={initialRun} />;
}

function ImageAgentWorkbenchSession({ taskId, runId, initialRun }: { taskId: string; runId: string; initialRun?: ImageAgentProjection }) {
  const agent = useImageAgentRun({ runId, initialRun });
  const projection = agent.projection;
  const [draftState, setDraftState] = useState(() => initialRun ? {
    revision: initialRun.plan.revision,
    draft: draftFromProjection(initialRun),
  } : undefined);

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

  if (!projection.run.business_task_id?.trim()) {
    return (
      <section className="rounded-[1.75rem] border border-destructive/40 bg-destructive/5 p-6">
        <p role="alert" className="text-sm text-destructive">
          当前图片 Agent 运行缺少业务任务归属，已停止展示
        </p>
      </section>
    );
  }

  if (projection.run.business_task_id !== taskId) {
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
  const blockedGuidance = guidanceForBlockedCode(projection.run.block?.code);
  const publicationOutcomeUnknown =
    projection.run.block?.code === "slot_publication_outcome_unknown";
  const canRetryBlockedSlot =
    !publicationOutcomeUnknown &&
    Boolean(blockedSlotID) &&
    projection.actions.includes("retry_slot");
  const canEditPlan = projection.actions.includes("edit_plan");
  const completedSlots = projection.slots.filter(
    (slot) => slot.slot.status === "accepted",
  ).length;
  const commandPending = Boolean(agent.pendingAction || projection.pending_command);
  const commandExhausted = projection.command_ingress?.exhausted === true;
  const newCommandDisabled = commandPending || commandExhausted;
  const currentDraft = draftState?.revision === projection.plan.revision
    ? draftState.draft
    : draftFromProjection(projection);
  const sourceAssets = projection.asset_catalog.filter((asset) => asset.type === "source");
  const styleAssets = projection.asset_catalog.filter((asset) => asset.type === "style");

  const togglePlanAsset = (type: "source" | "style", id: string) => {
    setDraftState((previous) => {
      const current = previous?.revision === projection.plan.revision
        ? previous.draft
        : draftFromProjection(projection);
      const field = type === "source" ? "source_asset_ids" : "style_reference_ids";
      const selected = current[field] ?? [];
      const next = selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id];
      return { revision: projection.plan.revision, draft: {
        ...current,
        [field]: next,
        slots: current.slots.map((slot) => ({
          ...slot,
          ...(type === "source" && !next.includes(id)
            ? { source_asset_ids: slot.source_asset_ids.filter((value) => value !== id) }
            : {}),
          ...(type === "style" && !next.includes(id)
            ? { style_reference_ids: slot.style_reference_ids?.filter((value) => value !== id) }
            : {}),
        })),
      }};
    });
  };

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
          <AssetSelector
            title="商品来源素材"
            description="只用于确认商品身份，不会被当作生成成功结果。"
            assets={sourceAssets}
            selectedIds={currentDraft.source_asset_ids}
            editable={canEditPlan}
            onToggle={(id) => togglePlanAsset("source", id)}
            testId="image-agent-source-materials"
          />
          <AssetSelector
            title="风格参考"
            description="只影响表现方式，与商品来源素材严格分开。"
            assets={styleAssets}
            selectedIds={currentDraft.style_reference_ids ?? []}
            editable={canEditPlan}
            onToggle={(id) => togglePlanAsset("style", id)}
            emptyLabel="当前任务没有可用风格库/风格参考不可用"
            testId="image-agent-style-references"
          />
        </aside>

        <PlanBoard
          key={projection.plan.revision}
          slots={currentDraft.slots}
          projections={slotProjectionByID}
          editable={canEditPlan}
          pending={newCommandDisabled}
          sourceAssets={sourceAssets.filter((asset) => currentDraft.source_asset_ids.includes(asset.id))}
          styleAssets={styleAssets.filter((asset) => currentDraft.style_reference_ids?.includes(asset.id))}
          onChange={(slots) => setDraftState({ revision: projection.plan.revision, draft: { ...currentDraft, slots } })}
          onSave={() => agent.replacePlan(currentDraft)}
        />

        <aside className="min-w-0 space-y-4 xl:sticky xl:top-6">
          <section className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm">
            <h3 className="font-semibold text-foreground">运行状态</h3>
            <dl className="mt-3 space-y-3 text-sm">
              <Metric label="状态" value={runStatusLabel(projection.run.status)} />
              <Metric label="当前节点" value={projection.run.current_node || "—"} />
              <Metric
                label="有效并发"
                value={projection.run.max_concurrent_slots === undefined
                  ? "未提供"
                  : `${projection.run.max_concurrent_slots} 个槽位`}
              />
              <Metric
                label="图片预算"
                value={formatBudgetUsage(projection.run.usage.images, projection.run.budget, "max_images")}
              />
              <Metric
                label="模型调用"
                value={formatBudgetUsage(projection.run.usage.model_calls, projection.run.budget, "max_model_calls")}
              />
              <Metric
                label="预估成本"
                value={formatCost(projection.run.usage.estimated_cost_micros)}
              />
            </dl>
          </section>

          {commandExhausted ? (
            <section
              role="alert"
              className="rounded-[1.5rem] border border-destructive/40 bg-destructive/5 p-4 text-destructive"
            >
              <h3 className="font-semibold">命令容量已耗尽，需要创建新运行</h3>
              <p className="mt-2 text-sm opacity-80">
                已使用 {projection.command_ingress?.used}/{projection.command_ingress?.limit} 个命令记录；已知操作仍可恢复。
              </p>
            </section>
          ) : null}

          {projection.run.status === "blocked" ? (
            <section className="rounded-[1.5rem] border border-amber-400/60 bg-amber-50 p-4 text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
              <p className="text-xs font-semibold uppercase tracking-[0.18em]">
                需要处理
              </p>
              <h3 className="mt-2 font-semibold">
                {blockedGuidance?.title ?? (blockedSlot
                  ? `${slotRoleLabel(blockedSlot.role)} ${blockedSlot.id} 生成失败`
                  : projection.run.block?.message || "图片生成流程已阻断")}
              </h3>
              {blockedGuidance ? (
                <p className="mt-2 text-sm opacity-80">{blockedGuidance.description}</p>
              ) : null}
              {projection.run.block?.message ? (
                <p className="mt-2 text-sm opacity-80">
                  {projection.run.block.message}
                </p>
              ) : null}
              {canRetryBlockedSlot && blockedSlotID ? (
                <button
                  type="button"
                  className="mt-4 inline-flex h-10 w-full items-center justify-center rounded-xl bg-amber-950 px-4 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-amber-200 dark:text-amber-950"
                  disabled={newCommandDisabled}
                  onClick={() => void agent.retrySlot(blockedSlotID)}
                >
                  {blockedGuidance ? "创建新尝试" : `仅重试 ${blockedSlotID}`}
                </button>
              ) : null}
            </section>
          ) : null}

          <section className="rounded-[1.5rem] border border-border bg-card p-4 shadow-sm">
            <h3 className="font-semibold text-foreground">可执行操作</h3>
            <div className="mt-3 space-y-2">
              {projection.pending_command ? (
                <div className="space-y-2">
                  {projection.pending_command.failure_message ? (
                    <div className="rounded-xl border border-amber-400/60 bg-amber-50 px-3 py-2 text-sm text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
                      <p className="font-medium">{projection.pending_command.failure_message}</p>
                      {projection.pending_command.failure_code ? (
                        <p className="mt-1 text-xs opacity-75">
                          {projection.pending_command.failure_category || "technical"} · {projection.pending_command.failure_code}
                          {projection.pending_command.attempt ? ` · 第 ${projection.pending_command.attempt} 次尝试` : ""}
                        </p>
                      ) : null}
                    </div>
                  ) : null}
                  <button
                    type="button"
                    className="inline-flex h-10 w-full items-center justify-center rounded-xl bg-amber-950 px-4 text-sm font-medium text-white disabled:opacity-50"
                    disabled={Boolean(agent.pendingAction)}
                    onClick={() => void agent.resumePending()}
                  >
                    恢复上次操作
                  </button>
                </div>
              ) : null}
              {projection.actions.includes("approve_results") ? (
                <button
                  type="button"
                  className="inline-flex h-10 w-full items-center justify-center rounded-xl bg-foreground px-4 text-sm font-medium text-background disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={!projection.result_digest || newCommandDisabled}
                  onClick={() => void agent.approveResults()}
                >
                  批准当前结果
                </button>
              ) : null}
              {projection.actions.includes("cancel") ? (
                <button
                  type="button"
                  className="inline-flex h-10 w-full items-center justify-center rounded-xl border border-border bg-background px-4 text-sm font-medium text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={newCommandDisabled}
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

function guidanceForBlockedCode(code: string | undefined) {
  switch (code) {
    case "slot_provider_outcome_unknown":
      return {
        title: "生成结果状态不确定",
        description: "生成请求的结果无法确认；如服务端允许，可创建新尝试。",
      };
    case "slot_staging_outcome_unknown":
      return {
        title: "持久化字节状态不完整",
        description: "生成结果的持久化字节未完成；如服务端允许，可创建新尝试。",
      };
    case "slot_publication_outcome_unknown":
      return {
        title: "发布结果需要验证",
        description: "目标位置的发布状态尚未确认，必须先验证目标位置，不能盲目重试。",
      };
    default:
      return undefined;
  }
}

function PlanBoard({
  slots,
  projections,
  editable,
  pending,
  sourceAssets,
  styleAssets,
  onChange,
  onSave,
}: {
  slots: ImageAgentSlot[];
  projections: Map<string, ImageAgentSlotProjection>;
  editable: boolean;
  pending: boolean;
  sourceAssets: ImageAgentAuthorizedAsset[];
  styleAssets: ImageAgentAuthorizedAsset[];
  onChange: (slots: ImageAgentSlot[]) => void;
  onSave: () => Promise<void>;
}) {
  const updateSlot = (index: number, update: Partial<ImageAgentSlot>) =>
    onChange(slots.map((slot, itemIndex) => itemIndex === index ? { ...slot, ...update } : slot));
  const addSlot = () => {
    let number = 1;
    while (slots.some((slot) => slot.id === `scene-${number}`)) number += 1;
    const id = `scene-${number}`;
    onChange([...slots, {
      id, role: "scene", source_asset_ids: sourceAssets[0] ? [sourceAssets[0].id] : [],
      style_reference_ids: styleAssets.map((asset) => asset.id), brief: "",
      idempotency_key: `slot-key-${id}`, status: "pending",
    }]);
  };
  return (
    <main className="min-w-0 space-y-3">
      {slots.map((slot, index) => (
        <SlotCard
          key={slot.id}
          slot={slot}
          projection={projections.get(slot.id)}
          draft={slot}
          editable={editable}
          sourceAssets={sourceAssets}
          styleAssets={styleAssets}
          onBriefChange={(brief) => updateSlot(index, { brief })}
          onSourcesChange={(source_asset_ids) => updateSlot(index, { source_asset_ids })}
          onStylesChange={(style_reference_ids) => updateSlot(index, { style_reference_ids })}
          onDelete={() => onChange(slots.filter((_, itemIndex) => itemIndex !== index))}
        />
      ))}
      {editable ? (
        <div className="flex flex-wrap gap-2">
          <button type="button" className="inline-flex h-10 items-center justify-center rounded-xl border border-border px-4 text-sm font-medium" disabled={pending} onClick={addSlot}>新增槽位</button>
          <button type="button" className="inline-flex h-10 items-center justify-center rounded-xl bg-foreground px-4 text-sm font-medium text-background disabled:cursor-not-allowed disabled:opacity-50" disabled={pending} onClick={() => void onSave()}>保存计划修改</button>
        </div>
      ) : null}
    </main>
  );
}

function AssetSelector({
  title,
  description,
  assets,
  selectedIds,
  editable,
  onToggle,
  emptyLabel,
  testId,
}: {
  title: string;
  description: string;
  assets: ImageAgentAuthorizedAsset[];
  selectedIds: string[];
  editable: boolean;
  onToggle: (id: string) => void;
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
        {assets.length > 0 ? (
          assets.map((asset) => (
            <li
              key={asset.id}
              className="break-all rounded-xl border border-border bg-background px-3 py-2 text-sm text-foreground"
            >
              <label className="flex items-start gap-2">
                <input type="checkbox" disabled={!editable} checked={selectedIds.includes(asset.id)} onChange={() => onToggle(asset.id)} />
                <span>{asset.label || asset.id}</span>
              </label>
              {safeDisplayURL(asset.display_url) ? (
                // eslint-disable-next-line @next/next/no-img-element -- authorized backend catalog URL
                <img className="mt-2 aspect-square w-full rounded-lg object-cover" src={asset.display_url} alt={asset.label || asset.id} />
              ) : null}
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
  sourceAssets,
  styleAssets,
  onBriefChange,
  onSourcesChange,
  onStylesChange,
  onDelete,
}: {
  slot: ImageAgentSlot;
  projection?: ImageAgentSlotProjection;
  draft: ImageAgentSlot;
  editable: boolean;
  sourceAssets: ImageAgentAuthorizedAsset[];
  styleAssets: ImageAgentAuthorizedAsset[];
  onBriefChange: (brief: string) => void;
  onSourcesChange: (ids: string[]) => void;
  onStylesChange: (ids: string[]) => void;
  onDelete: () => void;
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
        <fieldset disabled={!editable}>
          <legend>槽位来源</legend>
          {sourceAssets.map((asset) => <label key={asset.id} className="mt-1 flex gap-1"><input type="checkbox" checked={draft.source_asset_ids.includes(asset.id)} onChange={() => onSourcesChange(toggleID(draft.source_asset_ids, asset.id))} />{asset.label || asset.id}</label>)}
        </fieldset>
        <fieldset disabled={!editable}>
          <legend>槽位风格</legend>
          {styleAssets.length > 0 ? styleAssets.map((asset) => <label key={asset.id} className="mt-1 flex gap-1"><input type="checkbox" checked={draft.style_reference_ids?.includes(asset.id) ?? false} onChange={() => onStylesChange(toggleID(draft.style_reference_ids ?? [], asset.id))} />{asset.label || asset.id}</label>) : <p className="mt-1">当前任务没有可用风格库/风格参考不可用</p>}
        </fieldset>
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
      {editable ? <button type="button" className="mt-2 text-xs text-destructive" onClick={onDelete}>删除 {slot.id}</button> : null}
      {projection?.candidates.length ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {projection.candidates.map((candidate, index) => (
            <figure
              key={candidate.asset_id}
              className="overflow-hidden rounded-xl border border-border bg-muted"
            >
              {safeDisplayURL(candidate.url) ? (
                // eslint-disable-next-line @next/next/no-img-element -- URL is revalidated at the rendering boundary
                <img alt={`${slot.id} 候选图 ${index + 1}`} className="aspect-square w-full object-cover" src={safeDisplayURL(candidate.url)} />
              ) : null}
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

type ImageAgentBudgetLimit = NonNullable<
  ImageAgentProjection["run"]["budget"]["enabled_limits"]
>[number];

function formatBudgetUsage(
  used: number,
  budget: ImageAgentProjection["run"]["budget"],
  limit: ImageAgentBudgetLimit,
) {
  const enabled = budget.enabled_limits === undefined
    ? budget[limit] > 0
    : budget.enabled_limits.includes(limit);
  return `${used}/${enabled ? budget[limit] : "不限"}`;
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

function draftFromProjection(projection: ImageAgentProjection) {
  return {
    source_asset_ids: [...projection.plan.source_asset_ids],
    style_reference_ids: projection.plan.style_reference_ids ? [...projection.plan.style_reference_ids] : [],
    slots: projection.plan.slots.map(cloneSlot),
  };
}

function toggleID(values: string[], id: string) {
  return values.includes(id) ? values.filter((value) => value !== id) : [...values, id];
}

function safeDisplayURL(value?: string) {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" ? url.toString() : undefined;
  } catch {
    return undefined;
  }
}
