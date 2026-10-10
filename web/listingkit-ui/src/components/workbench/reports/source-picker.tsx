"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsoleState } from "../console/console-page";
import { fetchProductTitleProposals } from "@/lib/api/product-title-review-client";
import { requestAIWorkbench } from "@/lib/api/ai-workbench";
import { ReportError, reportRequest } from "@/lib/api/report-center";
import { reportSourceSchema, reviewLocator, type ReportScope, type ReportSource } from "@/lib/contracts/report-center";
import styles from "./report-page.module.css";
type SourceMode = "review" | "task" | "detail";
export function ReportSourcePicker({ scope, authorizationKey, disabled, tasksAvailable, save }: { scope: ReportScope; authorizationKey: string; disabled: boolean; tasksAvailable: boolean; save: (source: ReportSource) => void }) {
  const [mode, setMode] = useState<SourceMode>("review"), [cursor, setCursor] = useState("");
  const [locator, setLocator] = useState(""), [detailID, setDetailID] = useState(""), [locatorError, setLocatorError] = useState("");
  const query = useQuery({ queryKey: ["report-sources", scope.userId, scope.organizationId, authorizationKey, mode, cursor, detailID], enabled: mode !== "detail" || !!detailID, queryFn: async ({ signal }) => {
    if (mode === "detail") return { sources: [await reportRequest(scope, `/sources/TITLE_REVIEW/${detailID}`, reportSourceSchema, signal)], next: "" };
    let refs: { kind: "TITLE_REVIEW" | "SHEIN_RECORD"; id: string }[], next = "";
    if (mode === "review") { const page = await fetchProductTitleProposals({ ...scope, limit: 20, ...(cursor ? { cursor } : {}), signal }); refs = page.items.map(i => ({ kind: "TITLE_REVIEW", id: i.proposal_id })); next = page.next_cursor ?? ""; }
    else { const page = await requestAIWorkbench({ scope, signal, method: "GET", route: "task-list", path: `tasks?limit=20${cursor ? `&after=${cursor}` : ""}` }); refs = [...new Set(page.tasks.flatMap(i => i.reviewId ? [i.reviewId] : []))].map(id => ({ kind: "TITLE_REVIEW", id })); next = page.next; }
    const sources: ReportSource[] = [];
    // Original admin lists can contain others' records. Only the private source
    // adapter's successful metadata response is admitted into this selector.
    for (let start = 0; start < refs.length; start += 4) { const batch = await Promise.allSettled(refs.slice(start, start + 4).map(ref => reportRequest(scope, `/sources/${ref.kind}/${ref.id}`, reportSourceSchema, signal))); for (const result of batch) { if (result.status === "fulfilled") sources.push(result.value); else if (!(result.reason instanceof ReportError && result.reason.code === "NOT_FOUND")) throw result.reason; } }
    signal.throwIfAborted(); return { sources, next };
  }, retry: false, gcTime: 0, staleTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false });
  return <Card className={styles.picker}>
    <h2>保存本人已有结果</h2>
    <p>选择来源并手动保存当前展示版本。若来源在保存前变化，会拒绝本次保存。</p>
    <div className={styles.filters}>{([["review", "待处理标题审核"], ["detail", "已有审核详情"], ["task", "业务任务中的标题结果"]] as const).filter(([value]) => value !== "task" || tasksAvailable).map(([value, label]) => <Button key={value} aria-pressed={mode === value} variant={mode === value ? "secondary" : "outline"} disabled={disabled} onClick={() => { setMode(value); setCursor(""); }}>{label}</Button>)}<Button variant="outline" disabled title="报告保存暂未开放">SHEIN资料与保存时诊断</Button></div>
    <p>SHEIN资料报告保存暂未开放；已保存的历史报告仍可查阅和下载。</p>
    {mode === "detail" ? <form className={styles.filters} onSubmit={e => {
      e.preventDefault(); const id = reviewLocator(locator, window.location.origin);
      if (!id) { setDetailID(""); setLocatorError("请输入当前应用的标题审核详情链接或有效 ID。"); return; }
      setLocatorError(""); if (id === detailID) void query.refetch(); else setDetailID(id);
    }}>
      <label htmlFor="report-review-locator">标题审核详情链接或 ID</label>
      <input id="report-review-locator" value={locator} maxLength={2048} disabled={disabled} onChange={e => { setLocator(e.target.value); setDetailID(""); setLocatorError(""); }} />
      <Button type="submit" variant="outline" disabled={disabled || query.isFetching}>读取当前详情</Button>
      {locatorError ? <p role="alert">{locatorError}</p> : null}
    </form> : null}
    {mode === "detail" && !detailID ? <p>从已有标题审核详情复制链接，可保存已 Apply 或已拒绝的本人结果。</p> : query.isPending || query.isFetching ? <ConsoleState kind="loading" title="正在核实本人可保存的来源" /> : query.isError ? <ConsoleState kind="error" title="来源读取失败">未找到本人来源，或原来源权限、服务暂不可用。<Button variant="outline" onClick={() => void query.refetch()}>重试</Button></ConsoleState> : <>
      <ul className={styles.sources}>{query.data.sources.map(source => <li key={source.ref.id}><div><strong>{source.title}</strong><small>版本 {source.ref.version}{source.storeId ? ` · 店铺 ${source.storeId}` : ""}</small></div><Button disabled={disabled} onClick={() => save(source)}>保存此版本</Button></li>)}</ul>
      {query.data.sources.length === 0 ? <p>本页暂无本人可保存的结果。</p> : null}
      {query.data.next && query.data.next !== cursor ? <Button variant="outline" disabled={disabled} onClick={() => setCursor(query.data.next)}>查看下一页来源</Button> : null}
      {cursor ? <Button variant="ghost" disabled={disabled} onClick={() => setCursor("")}>返回首批来源</Button> : null}
    </>}
  </Card>;
}
