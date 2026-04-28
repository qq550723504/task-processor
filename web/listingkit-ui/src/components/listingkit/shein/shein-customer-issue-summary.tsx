import { Button } from "@/components/shared/button";
import type { CustomerIssue } from "@/lib/shein-studio/shein-customer-issues";

function severityLabel(severity: CustomerIssue["severity"]) {
  switch (severity) {
    case "warning":
      return "提醒";
    case "error":
      return "错误";
    default:
      return "阻断";
  }
}

function severityClass(severity: CustomerIssue["severity"]) {
  switch (severity) {
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-rose-200 bg-rose-50 text-rose-700";
  }
}

export function SheinCustomerIssueSummary({
  issues,
  compact = false,
  canSelectIssue,
  onSelectIssue,
}: {
  issues: CustomerIssue[];
  compact?: boolean;
  canSelectIssue?: ((issue: CustomerIssue) => boolean) | null;
  onSelectIssue?: ((issue: CustomerIssue) => void) | null;
}) {
  const blockingCount = issues.filter((issue) => issue.severity !== "warning").length;
  const warningCount = issues.length - blockingCount;
  const visibleIssues = issues.slice(0, 5);
  const extraIssues = issues.slice(5);
  const rawTexts = Array.from(
    new Set(issues.map((issue) => issue.rawText).filter(Boolean)),
  );

  if (!issues.length) {
    return (
      <div className="rounded-2xl border border-emerald-200 bg-emerald-50 p-4">
        <p className="text-sm font-semibold text-emerald-800">
          当前资料已通过提交前检查
        </p>
        <p className="mt-1 text-sm leading-6 text-emerald-700">
          没有发现阻断项。提交前仍建议核对最终图片、价格和 SKU。
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
            待处理问题
          </p>
          <p className="mt-1 text-sm font-semibold text-zinc-950">
            {blockingCount} 个阻断项 · {warningCount} 个提醒
          </p>
          {!compact ? (
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              下面是转换后的客户可读问题。原始 SHEIN 返回默认收起，必要时再查看。
            </p>
          ) : null}
        </div>
      </div>

      <div className="mt-3 space-y-2">
        {visibleIssues.map((issue, index) => {
          const canAct =
            Boolean(issue.actionKey) &&
            Boolean(onSelectIssue) &&
            (canSelectIssue ? canSelectIssue(issue) : true);
          return (
            <div
              className="rounded-xl border border-white/70 bg-white/75 p-3"
              key={`${issue.severity}-${issue.category}-${issue.title}-${index}`}
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 flex-1 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span
                      className={`rounded-full border px-2 py-1 text-[10px] font-semibold ${severityClass(
                        issue.severity,
                      )}`}
                    >
                      {severityLabel(issue.severity)}
                    </span>
                    <span className="text-[11px] font-semibold text-zinc-500">
                      {issue.category}
                    </span>
                  </div>
                  <p className="text-sm font-semibold text-zinc-950">
                    {issue.title}
                  </p>
                  <p className="text-sm leading-6 text-zinc-700">
                    {issue.message}
                  </p>
                </div>
                {canAct ? (
                  <Button
                    className="h-8 shrink-0 px-3 text-xs"
                    tone="secondary"
                    onClick={() => onSelectIssue?.(issue)}
                  >
                    {issue.actionLabel}
                  </Button>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>

      {extraIssues.length ? (
        <details className="mt-3 rounded-xl border border-white/70 bg-white/60 p-3">
          <summary className="cursor-pointer text-sm font-semibold text-zinc-800">
            查看剩余 {extraIssues.length} 条问题
          </summary>
          <div className="mt-2 space-y-2">
            {extraIssues.map((issue, index) => (
              <p
                className="text-sm leading-6 text-zinc-700"
                key={`${issue.title}-${index}`}
              >
                {issue.category}：{issue.title}
              </p>
            ))}
          </div>
        </details>
      ) : null}

      {rawTexts.length ? (
        <details className="mt-3 rounded-xl border border-zinc-200/70 bg-white/70 p-3">
          <summary className="cursor-pointer text-sm font-semibold text-zinc-800">
            查看原始 SHEIN 返回
          </summary>
          <ul className="mt-2 space-y-2">
            {rawTexts.map((rawText, index) => (
              <li
                className="break-words text-xs leading-5 text-zinc-600"
                key={`${index}-${rawText}`}
              >
                {rawText}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </div>
  );
}
