import type { SheinSubmissionEvent } from "@/lib/types/listingkit";
import {
  sheinSubmissionActionLabel,
  sheinSubmissionStatusLabel,
  sheinSubmitPhaseLabel,
} from "@/lib/shein-studio/shein-submission-display";
import { Button } from "@/components/shared/button";

function formatTime(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function tone(status?: string) {
  if (status === "success") return "border-emerald-200 bg-emerald-50 text-emerald-700";
  if (status === "confirmed") return "border-emerald-200 bg-emerald-50 text-emerald-700";
  if (status === "pending") return "border-amber-200 bg-amber-50 text-amber-700";
  if (status === "failed") return "border-rose-200 bg-rose-50 text-rose-700";
  return "border-zinc-200 bg-zinc-50 text-zinc-600";
}

function isPrimaryEvent(action?: string) {
  return action === "submit_phase" || action === "image_upload" || action === "save_draft" || action === "publish";
}

function eventTitle(event: SheinSubmissionEvent) {
  if (event.action === "submit_phase") {
    return sheinSubmitPhaseLabel(event.phase) ?? "提交阶段";
  }
  return sheinSubmissionActionLabel(event.action);
}

function TimelineEventCard({ event, index }: { event: SheinSubmissionEvent; index: number }) {
  return (
    <article
      className="rounded-2xl border border-zinc-100 bg-zinc-50 p-3"
      key={event.id ?? `${event.action}-${event.started_at}-${index}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm font-semibold text-zinc-950">
          {eventTitle(event)}
        </div>
        <span className={`rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] ${tone(event.status)}`}>
          {sheinSubmissionStatusLabel(event.status)}
        </span>
      </div>
      <div className="mt-1 text-xs text-zinc-500">
        {formatTime(event.started_at)}
        {event.finished_at ? ` - ${formatTime(event.finished_at)}` : ""}
      </div>
      {event.request_id || event.remote_record_id ? (
        <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-zinc-500">
          {event.request_id ? <span>Request {event.request_id}</span> : null}
          {event.remote_record_id ? <span>Record {event.remote_record_id}</span> : null}
        </div>
      ) : null}
      {event.detail ? (
        <p className="mt-2 text-xs leading-5 text-zinc-600">{event.detail}</p>
      ) : null}
      {event.error_message ? (
        <details className="mt-2 rounded-xl border border-rose-100 bg-rose-50/70 p-2">
          <summary className="cursor-pointer text-xs font-semibold text-rose-700">
            查看失败详情
          </summary>
          <p className="mt-1 break-words text-xs leading-5 text-rose-700">
            {event.error_message}
          </p>
        </details>
      ) : null}
      {event.validation_notes?.length ? (
        <details className="mt-2 rounded-xl border border-amber-100 bg-amber-50/70 p-2">
          <summary className="cursor-pointer text-xs font-semibold text-amber-800">
            查看 SHEIN 校验提示
          </summary>
          <ul className="mt-1 space-y-1 text-xs leading-5 text-amber-800">
            {event.validation_notes.slice(0, 4).map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </details>
      ) : null}
      {event.response?.spu_name || event.response?.message ? (
        <p className="mt-2 text-xs leading-5 text-zinc-600">
          {event.response.spu_name ? `SPU ${event.response.spu_name}. ` : ""}
          {event.response.message}
        </p>
      ) : null}
    </article>
  );
}

export function SheinSubmissionTimeline({
  events,
  canRefresh,
  isRefreshing,
  onRefresh,
}: {
  events?: SheinSubmissionEvent[];
  canRefresh?: boolean;
  isRefreshing?: boolean;
  onRefresh?: (() => void) | null;
}) {
  if (!events?.length) {
    return null;
  }
  const primaryEvents = events.filter((event) => isPrimaryEvent(event.action));
  const advancedEvents = events.filter((event) => !isPrimaryEvent(event.action));

  return (
    <section className="space-y-3 rounded-[1.5rem] border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            提交记录
          </p>
          <h3 className="mt-1 text-sm font-semibold text-zinc-950">
            SHEIN 提交时间线
          </h3>
        </div>
        {canRefresh && onRefresh ? (
          <Button
            className="h-8 px-3 text-xs"
            disabled={isRefreshing}
            tone="secondary"
            onClick={onRefresh}
          >
            {isRefreshing ? "刷新中..." : "刷新状态"}
          </Button>
        ) : null}
      </div>
      <div className="space-y-2">
        {(primaryEvents.length ? primaryEvents : events).slice(0, 8).map((event, index) => (
          <TimelineEventCard event={event} index={index} key={event.id ?? `${event.action}-${event.started_at}-${index}`} />
        ))}
      </div>
      {advancedEvents.length ? (
        <details className="rounded-2xl border border-zinc-200 bg-zinc-50 p-3">
          <summary className="cursor-pointer text-sm font-semibold text-zinc-800">
            高级日志（{advancedEvents.length}）
          </summary>
          <div className="mt-2 space-y-2">
            {advancedEvents.slice(0, 8).map((event, index) => (
              <TimelineEventCard event={event} index={index} key={event.id ?? `${event.action}-${event.started_at}-${index}`} />
            ))}
          </div>
        </details>
      ) : null}
    </section>
  );
}
