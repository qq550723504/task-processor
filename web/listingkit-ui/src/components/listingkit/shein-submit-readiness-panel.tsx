import { Card } from "@/components/shared/card";
import { Button } from "@/components/shared/button";
import type {
  SheinChecklistGroupItem,
  SheinReadinessItem,
  SheinSubmitChecklist,
  SheinSubmitReadiness,
  SheinWorkspaceOverview,
} from "@/lib/types/listingkit";

function statusLabel(status?: string) {
  switch (status) {
    case "blocked":
      return "Blocked";
    case "ready_with_warnings":
      return "Ready with warnings";
    case "ready":
      return "Ready";
    default:
      return "Unknown";
  }
}

function statusTone(status?: string) {
  switch (status) {
    case "blocked":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "ready_with_warnings":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "ready":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-700";
  }
}

function checklistLabel(items?: SheinChecklistGroupItem[] | null) {
  if (!items?.length) {
    return null;
  }
  return items;
}

function fieldPathsLabel(paths?: string[] | null) {
  if (!paths?.length) {
    return null;
  }
  return paths.join(" · ");
}

function ReadinessItems({
  title,
  items,
  actionLabel = "Open fix path",
  canSelectItem,
  onSelectItem,
}: {
  title: string;
  items?: SheinReadinessItem[] | null;
  actionLabel?: string;
  canSelectItem?: ((item: SheinReadinessItem) => boolean) | null;
  onSelectItem?: ((item: SheinReadinessItem) => void) | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-3">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-3">
        {items.map((item) => {
          const canAct = canSelectItem ? canSelectItem(item) : false;
          return (
            <div
              className="space-y-2 rounded-2xl border border-zinc-200 bg-white/80 p-4"
              key={`${title}-${item.key}-${item.label}`}
            >
              <div className="space-y-1">
                <p className="text-sm font-semibold text-zinc-950">
                  {item.label ?? item.key ?? "Unnamed item"}
                </p>
                {item.message ? (
                  <p className="text-sm leading-6 text-zinc-700">{item.message}</p>
                ) : null}
              </div>
              {item.reason?.summary ? (
                <p className="text-xs leading-5 text-zinc-600">
                  {item.reason.summary}
                </p>
              ) : null}
              {fieldPathsLabel(item.field_paths) ? (
                <p className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                  {fieldPathsLabel(item.field_paths)}
                </p>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                {item.suggested_action ? (
                  <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
                    {item.suggested_action}
                  </span>
                ) : null}
                {canAct && onSelectItem ? (
                  <Button
                    className="h-8 px-3 text-xs"
                    tone="secondary"
                    onClick={() => onSelectItem(item)}
                  >
                    {actionLabel}
                  </Button>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ChecklistSection({
  title,
  items,
}: {
  title: string;
  items?: SheinChecklistGroupItem[] | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-2">
        {items.map((item) => (
          <div
            className="flex items-start justify-between gap-3 rounded-xl border border-zinc-200/80 bg-white/70 px-3 py-2"
            key={`${title}-${item.key}-${item.label}`}
          >
            <div className="space-y-1">
              <p className="text-sm font-medium text-zinc-900">
                {item.label ?? item.key ?? "Unnamed check"}
              </p>
              {item.message ? (
                <p className="text-xs leading-5 text-zinc-600">{item.message}</p>
              ) : null}
            </div>
            <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
              {item.status ?? "unknown"}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function SheinSubmitReadinessPanel({
  readiness,
  checklist,
  workspaceOverview,
  canSelectBlockingItem,
  onSelectBlockingItem,
  canRunPrimaryAction,
  onRunPrimaryAction,
}: {
  readiness?: SheinSubmitReadiness | null;
  checklist?: SheinSubmitChecklist | null;
  workspaceOverview?: SheinWorkspaceOverview | null;
  canSelectBlockingItem?: ((item: SheinReadinessItem) => boolean) | null;
  onSelectBlockingItem?: ((item: SheinReadinessItem) => void) | null;
  canRunPrimaryAction?: ((key?: string | null) => boolean) | null;
  onRunPrimaryAction?: ((key?: string | null) => void) | null;
}) {
  if (!readiness && !checklist && !workspaceOverview) {
    return null;
  }

  const required = checklistLabel(checklist?.required);
  const recommended = checklistLabel(checklist?.recommended);
  const primaryActionKey = workspaceOverview?.primary_action_key;
  const canRunPrimary =
    readiness?.status !== "ready" &&
    Boolean(primaryActionKey) &&
    (canRunPrimaryAction ? canRunPrimaryAction(primaryActionKey) : false);

  return (
    <Card className="border-zinc-300 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-600">
              SHEIN publish readiness
            </p>
            <span
              className={`rounded-full border px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${statusTone(
                readiness?.status ?? workspaceOverview?.submit_state?.status,
              )}`}
            >
              {statusLabel(readiness?.status ?? workspaceOverview?.submit_state?.status)}
            </span>
          </div>
          {workspaceOverview?.headline ? (
            <p className="text-sm font-semibold text-zinc-950">
              {workspaceOverview.headline}
            </p>
          ) : null}
          {workspaceOverview?.subheadline ? (
            <p className="text-sm leading-6 text-zinc-700">
              {workspaceOverview.subheadline}
            </p>
          ) : null}
          <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
            <span>
              Blocking {readiness?.blocking_items?.length ?? workspaceOverview?.submit_state?.blocking_count ?? 0}
            </span>
            <span>
              Warnings {readiness?.warning_items?.length ?? workspaceOverview?.submit_state?.warning_count ?? 0}
            </span>
          </div>
        </div>

        {workspaceOverview?.primary_action ? (
          <div className="rounded-2xl border border-zinc-200 bg-white/80 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="space-y-1">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  Primary next step
                </p>
                <p className="text-sm font-semibold text-zinc-950">
                  {workspaceOverview.primary_action}
                </p>
              </div>
              {canRunPrimary && onRunPrimaryAction ? (
                <Button
                  className="h-8 px-3 text-xs"
                  tone="secondary"
                  onClick={() => onRunPrimaryAction(primaryActionKey)}
                >
                  Open fix path
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}

        {readiness?.summary?.length ? (
          <div className="space-y-2">
            {readiness.summary.map((line) => (
              <p className="text-sm leading-6 text-zinc-700" key={line}>
                {line}
              </p>
            ))}
          </div>
        ) : null}

        <ReadinessItems
          title="Blocking items"
          items={readiness?.blocking_items}
          canSelectItem={canSelectBlockingItem}
          onSelectItem={onSelectBlockingItem}
        />

        <ReadinessItems
          title="Warnings"
          items={readiness?.warning_items}
        />

        {workspaceOverview?.highlights?.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              Highlights
            </p>
            <div className="space-y-2">
              {workspaceOverview.highlights.map((line) => (
                <p className="text-sm leading-6 text-zinc-700" key={line}>
                  {line}
                </p>
              ))}
            </div>
          </div>
        ) : null}

        {workspaceOverview?.next_actions?.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              Next actions
            </p>
            <div className="flex flex-wrap gap-2">
              {workspaceOverview.next_actions.map((line) => (
                <span
                  className="rounded-full border border-zinc-200 bg-white/80 px-3 py-1 text-[11px] font-medium tracking-[0.02em] text-zinc-700"
                  key={line}
                >
                  {line}
                </span>
              ))}
            </div>
          </div>
        ) : null}

        {required || recommended ? (
          <div className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              Checklist
            </p>
            <ChecklistSection title="Required" items={required} />
            <ChecklistSection title="Recommended" items={recommended} />
          </div>
        ) : null}
      </div>
    </Card>
  );
}
