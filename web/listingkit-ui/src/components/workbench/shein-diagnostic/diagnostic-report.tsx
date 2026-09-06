import { AlertTriangle, CircleHelp, ClipboardCheck } from "lucide-react";
import { useId } from "react";

import { Card } from "@/components/ui/card";
import type { SheinDiagnostic } from "@/lib/api/shein-diagnostic";

export function DiagnosticReport({ report, recordId }: { report: SheinDiagnostic; recordId: string }) {
  const blocked = report.offline_checks.status === "blocked";
  return (
    <div className="space-y-5 break-words [overflow-wrap:anywhere]">
      <Card className="p-5 sm:p-6">
        <div className="flex items-start gap-3">
          <ClipboardCheck aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-primary" />
          <div>
            <h2 className="text-lg font-semibold">
              {blocked ? "发现需要处理的问题" : report.not_evaluated.length ? "离线项通过，仍有未评估范围" : "离线检查已完成"}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">{report.action === "publish" ? "按发布规则检查" : "按本地草稿规则检查"}</p>
            <p className="mt-3 text-sm">本结果仅覆盖离线资料检查，不代表可以发布。平台在线检查、批准和提交前门禁需另行完成。</p>
            {report.action_policy.readiness_blockers_allowed ? <p className="mt-2 text-sm text-muted-foreground">本地草稿规则允许资料尚未完善；下列问题仍保留供后续处理。</p> : null}
          </div>
        </div>
      </Card>

      <div className="grid items-start gap-5 lg:grid-cols-2">
        <CheckGroup title="问题" checks={report.offline_checks.blockers} tone="problem" />
        <CheckGroup title="警告" checks={report.offline_checks.warnings} tone="warning" />
      </div>

      <Card className="p-5 sm:p-6">
        <h2 className="flex items-center gap-2 text-base font-semibold"><CircleHelp aria-hidden="true" className="size-5 shrink-0" />未评估范围</h2>
        <p className="mt-2 text-sm text-muted-foreground">以下范围未包含在本次离线结果中。读取时间不代表外部资料仍然有效。</p>
        <ul className="mt-4 grid gap-3 sm:grid-cols-2">
          {report.not_evaluated.map((item, index) => (
            <li className="rounded-lg border bg-muted/30 p-3 text-sm" key={`${item}-${index}`}>
              <p className="font-medium">{scopeLabel(item)}</p>
              <p className="mt-1 text-muted-foreground">{report.not_evaluated_reasons?.[item] === "no_authoritative_package_freshness" ? "没有权威的外部资料有效性证据。" : "本次离线检查未评估此范围。"}</p>
              <details className="mt-2 text-xs text-muted-foreground">
                <summary className="cursor-pointer rounded focus-visible:outline-2 focus-visible:outline-ring">范围与原因标识</summary>
                <p className="mt-2">{item}</p>
                {report.not_evaluated_reasons?.[item] ? <p>{report.not_evaluated_reasons[item]}</p> : <p>服务端未提供单独原因</p>}
              </details>
            </li>
          ))}
        </ul>
        {report.not_evaluated.length === 0 ? <p className="mt-3 text-sm">服务端未列出未评估项；本报告仍仅供诊断。</p> : null}
      </Card>

      <CheckGroup title="检查项目" checks={report.offline_checks.checks} tone="neutral" />

      <Card className="p-5 sm:p-6">
        <details>
          <summary className="cursor-pointer rounded font-medium focus-visible:outline-2 focus-visible:outline-ring">检查技术信息</summary>
          <dl className="mt-4 grid gap-4 text-sm sm:grid-cols-2">
            <Datum label="本地资料标识" value={recordId} />
            <Datum label="检查范围" value={report.scope} />
            <Datum label="规则版本" value={report.rule_version} />
            <Datum label="内容绑定版本" value={report.input.binding_version} />
            <Datum label="读取时间（服务端）" value={report.input.read_at} />
            <Datum label="评估时间（服务端）" value={report.input.evaluated_at} />
            <Datum label="实际内容摘要" value={report.input.actual_digest} />
            <Datum label="外部资料有效性状态" value={report.external_freshness.status} />
            <Datum label="外部有效性覆盖" value={report.external_freshness.coverage.join("、") || "未提供"} />
            {report.external_freshness.status === "valid" ? Object.entries(report.external_freshness.evidence).map(([key, value]) => <Datum key={key} label={`外部证据 · ${key}`} value={String(value)} />) : null}
          </dl>
        </details>
      </Card>
    </div>
  );
}

function CheckGroup({ title, checks, tone }: { title: string; checks: SheinDiagnostic["offline_checks"]["checks"]; tone: "problem" | "warning" | "neutral" }) {
  const id = useId();
  return (
    <section aria-labelledby={id} className="min-w-0 rounded-lg border bg-card p-5 shadow-sm sm:p-6">
      <h2 className="flex items-center gap-2 font-semibold" id={id}>
        {tone !== "neutral" ? <AlertTriangle aria-hidden="true" className={`size-4 ${tone === "problem" ? "text-destructive" : "text-amber-700 dark:text-amber-300"}`} /> : null}
        {title}（{checks.length}）
      </h2>
      {checks.length === 0 ? <p className="mt-3 text-sm text-muted-foreground">本次没有{title}项。</p> : (
        <ul className="mt-4 divide-y">
          {checks.map((check, index) => (
            <li key={`${check.rule}-${index}`} className="py-3 first:pt-0 last:pb-0">
              <details>
                <summary className="cursor-pointer rounded text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring">{check.message || check.code || check.rule}</summary>
                <dl className="mt-3 space-y-2 text-sm">
                  <Datum label="规则" value={check.rule} />
                  <Datum label="错误 / 检查码" value={check.code} />
                  <Datum label="类别" value={check.category} />
                  <Datum label="状态" value={check.status} />
                  <Datum label="字段路径" value={check.paths?.join("、") || "服务端未提供"} />
                  <Datum label="修正建议" value={check.guidance || "服务端未提供单独建议"} />
                </dl>
              </details>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function Datum({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 whitespace-pre-wrap [overflow-wrap:anywhere]">{value}</dd></div>;
}

function scopeLabel(scope: string) {
  const labels: Record<string, string> = {
    external_package_freshness: "外部资料有效性",
    online_template_freshness: "平台在线模板有效性",
    store_authorization: "店铺授权",
    cookie: "平台登录状态",
    pod: "POD 平台检查",
    human_review: "人工审核",
    approved_asset_provenance_and_consent: "图片来源、批准与授权",
    submission_gate: "提交前门禁",
  };
  return labels[scope] || scope;
}
