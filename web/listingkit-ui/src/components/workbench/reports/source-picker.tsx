"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsoleState } from "../console/console-page";
import { fetchProductTitleProposals } from "@/lib/api/product-title-review-client";
import { fetchSheinRecords } from "@/lib/api/shein-records-client";
import { requestAIWorkbench } from "@/lib/api/ai-workbench";
import { ReportError, reportRequest } from "@/lib/api/report-center";
import { reportSourceSchema, type ReportScope, type ReportSource } from "@/lib/contracts/report-center";
import styles from "./report-page.module.css";
type SourceMode = "review" | "task" | "record";
export function ReportSourcePicker({ scope, authorizationKey, disabled, tasksAvailable, save }: { scope: ReportScope; authorizationKey: string; disabled: boolean; tasksAvailable: boolean; save: (source: ReportSource) => void }) {
  const [mode, setMode] = useState<SourceMode>("review"), [cursor, setCursor] = useState("");
  const query = useQuery({ queryKey: ["report-sources", scope.userId, scope.organizationId, authorizationKey, mode, cursor], queryFn: async ({ signal }) => {
    let refs: { kind: "TITLE_REVIEW" | "SHEIN_RECORD"; id: string }[], next = "";
    if (mode === "review") { const page = await fetchProductTitleProposals({ ...scope, limit: 20, ...(cursor ? { cursor } : {}), signal }); refs = page.items.map(i => ({ kind: "TITLE_REVIEW", id: i.proposal_id })); next = page.next_cursor ?? ""; }
    else if (mode === "record") { const page = await fetchSheinRecords({ organizationId: scope.organizationId, limit: 20, ...(cursor ? { cursor } : {}), signal }); refs = page.items.map(i => ({ kind: "SHEIN_RECORD", id: i.record_id })); next = page.next_cursor ?? ""; }
    else { const page = await requestAIWorkbench({ scope, signal, method: "GET", route: "task-list", path: `tasks?limit=20${cursor ? `&after=${cursor}` : ""}` }); refs = [...new Set(page.tasks.flatMap(i => i.reviewId ? [i.reviewId] : []))].map(id => ({ kind: "TITLE_REVIEW", id })); next = page.next; }
    const sources: ReportSource[] = [];
    // Original admin lists can contain others' records. Only the private source
    // adapter's successful metadata response is admitted into this selector.
    for (let start = 0; start < refs.length; start += 4) { const batch = await Promise.allSettled(refs.slice(start, start + 4).map(ref => reportRequest(scope, `/sources/${ref.kind}/${ref.id}`, reportSourceSchema, signal))); for (const result of batch) { if (result.status === "fulfilled") sources.push(result.value); else if (!(result.reason instanceof ReportError && result.reason.code === "NOT_FOUND")) throw result.reason; } }
    signal.throwIfAborted(); return { sources, next };
  }, retry: false, gcTime: 0, staleTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false });
  return <Card className={styles.picker}><h2>保存本人已有结果</h2><p>选择来源并手动保存当前展示版本。若来源在保存前变化，会拒绝本次保存。</p><div className={styles.filters}>{([["review", "待处理标题审核"], ["task", "业务任务中的标题结果"], ["record", "SHEIN资料与保存时诊断"]] as const).filter(([value]) => value !== "task" || tasksAvailable).map(([value, label]) => <Button key={value} aria-pressed={mode === value} variant={mode === value ? "secondary" : "outline"} disabled={disabled} onClick={() => { setMode(value); setCursor(""); }}>{label}</Button>)}</div>{query.isPending || query.isFetching ? <ConsoleState kind="loading" title="正在核实本人可保存的来源" /> : query.isError ? <ConsoleState kind="error" title="来源读取失败">原来源权限或服务暂不可用。<Button variant="outline" onClick={() => void query.refetch()}>重试</Button></ConsoleState> : <><ul className={styles.sources}>{query.data.sources.map(source => <li key={source.ref.id}><div><strong>{source.title}</strong><small>版本 {source.ref.version}{source.storeId ? ` · 店铺 ${source.storeId}` : ""}</small></div><Button disabled={disabled} onClick={() => save(source)}>保存此版本</Button></li>)}</ul>{query.data.sources.length === 0 ? <p>本页暂无本人可保存的结果。</p> : null}{query.data.next && query.data.next !== cursor ? <Button variant="outline" disabled={disabled} onClick={() => setCursor(query.data.next)}>查看下一页来源</Button> : null}{cursor ? <Button variant="ghost" disabled={disabled} onClick={() => setCursor("")}>返回首批来源</Button> : null}</>}</Card>;
}
