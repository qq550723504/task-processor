"use client";
import Link from "next/link";
import { useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Clock, FileText, FolderOpen, Star } from "lucide-react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConsolePage, ConsoleState } from "../console/console-page";
import { ReportError, readReportIntent, reportRequest, writeReportIntent } from "@/lib/api/report-center";
import { reportResultSchema, reportSchema, reportsPageSchema, reportsSummarySchema, reportText, sourceHref, type Report, type ReportIntent, type ReportScope, type ReportSource } from "@/lib/contracts/report-center";
import { ReportSourcePicker } from "./source-picker";
import styles from "./report-page.module.css";
type View = "overview" | "recent" | "favorites" | "all";
const labels = { overview: "我的报告", recent: "最近报告", favorites: "收藏报告", all: "全部报告" };
const types = { TITLE_REVIEW: "标题审核", SHEIN_RECORD: "商品资料与离线诊断" };
const errorText = (e: unknown) => e instanceof ReportError ? ({ OUTCOME_UNKNOWN: "操作结果尚未确认。请使用原请求恢复，避免创建新的保存或收藏请求。", CONFLICT: "来源版本已变化或请求内容冲突，请重新选择来源。", FORBIDDEN: "当前权限不足。已保留原请求，可在权限恢复后重试。", NOT_FOUND: "本人当前企业范围内未找到该来源或报告。", INTENT_STORAGE_UNAVAILABLE: "无法保存操作恢复信息，已停止提交。", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化，请返回当前企业后恢复原请求。", IDENTITY_CONTEXT_CHANGED: "登录用户已变化，请重新登录后读取。" }[e.code] ?? "报告服务暂不可用，请稍后重试。") : "报告服务暂不可用，请稍后重试。";
const policy = { retry: false, gcTime: 0, staleTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false } as const;
export function ReportPage({ view = "overview", titleReviewAvailable = false }: { view?: View; titleReviewAvailable?: boolean }) {
  const context = useWorkbenchContext();
  const scope = context.user && context.effectiveOrganization && !context.selectionRequired && !context.isSwitching && !context.isLoading && !context.error && !context.blockingError ? { userId: context.user.id, organizationId: context.effectiveOrganization.id } : null;
  const authorizationKey = JSON.stringify([context.roles, context.permissions]);
  return <ConsolePage title={labels[view]} description={view === "overview" ? "AI成果沉淀与复用，保留本人当前企业的历史结果。" : "查阅已手动保存的只读报告，收藏重要结果并下载复用。"} breadcrumbs={[{ label: "AI工作台" }, { label: "我的报告", href: "/workbench/ai/reports" }, ...(view === "overview" ? [] : [{ label: labels[view] }])]}>
    {!scope ? <ConsoleState kind={context.isSwitching || context.isLoading ? "loading" : "unavailable"} title="企业上下文不可用">请选择可访问的企业并登录。</ConsoleState> : !context.permissions.includes("workbench.report.read") ? <ConsoleState kind="unavailable" title="报告尚不可用">当前企业尚未启用报告，或当前用户没有查看权限。</ConsoleState> : <ScopedReports key={JSON.stringify([scope.userId, scope.organizationId, authorizationKey])} scope={scope} authorizationKey={authorizationKey} view={view} manage={context.permissions.includes("workbench.report.manage")} tasksAvailable={context.aiWorkbenchAvailable} titleReviewAvailable={titleReviewAvailable} />}
  </ConsolePage>;
}
function ScopedReports({ scope, authorizationKey, view, manage, tasksAvailable, titleReviewAvailable }: { scope: ReportScope; authorizationKey: string; view: View; manage: boolean; tasksAvailable: boolean; titleReviewAvailable: boolean }) {
  const client = useQueryClient(), prefix = ["reports", scope.userId, scope.organizationId, authorizationKey];
  const [selected, setSelected] = useState<string>(), [showSave, setShowSave] = useState(false), [intentOverride, setIntent] = useState<ReportIntent | null | undefined>(), [feedback, setFeedback] = useState("");
  const recovery = useQuery({ ...policy, queryKey: [...prefix, "intent"], queryFn: () => readReportIntent(scope), staleTime: Infinity });
  const intent = intentOverride === undefined ? recovery.data ?? null : intentOverride, intentReady = recovery.isSuccess;
  const notice = feedback || (recovery.isError ? errorText(recovery.error) : intent ? "有尚未确认的原请求，请恢复该请求后继续操作。" : "");
  const summary = useQuery({ ...policy, queryKey: [...prefix, "summary"], queryFn: ({ signal }) => reportRequest(scope, "/summary", reportsSummarySchema, signal) });
  const detail = useQuery({ ...policy, queryKey: [...prefix, "detail", selected], enabled: !!selected, queryFn: ({ signal }) => reportRequest(scope, `/${selected}`, reportSchema, signal) });
  const mutation = useMutation({ mutationKey: [...prefix, "write"], mutationFn: (command: ReportIntent) => reportRequest(scope, command.operation === "save" ? "" : `/${command.id}/favorite`, reportResultSchema, undefined, command), onSuccess: (result) => {
    try { writeReportIntent(scope, null); setIntent(null); setSelected(result.report.id); setShowSave(false); setFeedback(result.replayed ? "已找回原请求保存的报告，收藏显示当前状态。" : "操作已保存。"); void client.invalidateQueries({ queryKey: prefix }); } catch (e) { setFeedback(errorText(e)); }
  }, onError: (e) => { setFeedback(errorText(e)); if (e instanceof ReportError && ["CONFLICT", "INVALID_REQUEST", "NOT_FOUND"].includes(e.code)) { try { writeReportIntent(scope, null); setIntent(null); } catch (storageError) { setFeedback(errorText(storageError)); } } } });
  function submit(command: ReportIntent) { if (!manage || mutation.isPending || intent || !intentReady) return; try { writeReportIntent(scope, command); setIntent(command); setFeedback(""); mutation.mutate(command); } catch (e) { setFeedback(errorText(e)); } }
  const busy = mutation.isPending || !!intent || !intentReady;
  return <>
    <div className={styles.actions}><span>归属：当前企业 · 本人</span>{manage ? <Button onClick={() => setShowSave(v => !v)} disabled={busy || !titleReviewAvailable}>{showSave ? "收起保存" : "保存报告"}</Button> : null}</div>
    {manage && !titleReviewAvailable ? <p>标题审核报告保存暂未开放；已保存的历史报告仍可查阅、收藏和下载。</p> : null}
    {notice ? <div className={styles.feedback} role="status"><p>{notice}</p>{intent && !mutation.isPending && manage ? <Button variant="outline" onClick={() => mutation.mutate(intent)}>恢复原请求</Button> : null}</div> : null}
    {showSave && manage && titleReviewAvailable ? <ReportSourcePicker scope={scope} authorizationKey={authorizationKey} tasksAvailable={tasksAvailable} disabled={busy} save={(source: ReportSource) => submit({ operation: "save", key: crypto.randomUUID(), source: source.ref })} /> : null}
    {view === "overview" ? <>
      {summary.isPending || summary.isFetching ? <ConsoleState kind="loading" title="正在读取报告汇总" /> : summary.isError ? <ConsoleState kind="error" title="汇总读取失败"><p>{errorText(summary.error)}</p><Button variant="outline" onClick={() => void summary.refetch()}>重试</Button></ConsoleState> : <div className={styles.summary}><strong>已保存 {summary.data.saved} 份报告</strong><span>近30天 {summary.data.recent} 份</span><span>收藏 {summary.data.favorites} 份</span><span>涉及 {summary.data.stores} 个店铺</span><span>项目关联暂未开放</span></div>}
      <div className={styles.cards}>{([
        { view: "recent", Icon: Clock, description: "快速查阅最近30天保存的成果", uses: ["查看最近保存的报告", "检查保存时版本与来源", "下载后继续使用"], value: summary.data?.recent, note: "按保存时间展示，保留当时状态" },
        { view: "favorites", Icon: Star, description: "沉淀常用结果，方便再次查阅", uses: ["收藏重要报告", "集中查阅个人常用结果", "取消不再需要的收藏"], value: summary.data?.favorites, note: "本人收藏，只在当前企业可见" },
        { view: "all", Icon: FolderOpen, description: "查找本人已保存的历史报告", uses: ["按类型和商品查找", "查看只读内容与原始来源", "下载JSON或文本文件"], value: summary.data?.saved, note: "保存不执行模型、Apply或平台操作" },
      ] as const).map(card => <Card className={styles.feature} data-tone={card.view} key={card.view}><card.Icon size={30} aria-hidden="true" /><h2>{labels[card.view]}</h2><p>{card.description}</p><h3>主要用途</h3><ul>{card.uses.map(use => <li key={use}>{use}</li>)}</ul><div className={styles.usage}><span>已保存</span><strong>{summary.isSuccess && !summary.isFetching ? `${card.value} 份` : "—"}</strong><p>{card.note}</p></div><Button asChild><Link href={`/workbench/ai/reports/${card.view}`} prefetch={false}>进入{labels[card.view]}</Link></Button></Card>)}</div>
    </> : <ReportList scope={scope} prefix={prefix} view={view} open={setSelected} />}
    {selected ? <Card className={styles.detail} role="region" aria-label="报告详情"><header><h2>只读报告详情</h2><Button variant="ghost" onClick={() => setSelected(undefined)}>关闭详情</Button></header>{detail.isPending || detail.isFetching ? <ConsoleState kind="loading" title="正在读取报告" /> : detail.isError ? <ConsoleState kind="error" title="报告读取失败"><p>{errorText(detail.error)}</p><Button variant="outline" onClick={() => void detail.refetch()}>重试</Button></ConsoleState> : <ReportDetail report={detail.data} manage={manage} busy={busy} favorite={() => submit({ operation: "favorite", key: crypto.randomUUID(), id: detail.data.id, favorite: !detail.data.favorite })} />}</Card> : null}
    <p className={styles.note}>当前支持标题审核、已保存的 SHEIN 商品资料及保存时离线诊断。市场分析、店铺经营分析和团队共享暂未开放。</p>
  </>;
}
function ReportList({ scope, prefix, view, open }: { scope: ReportScope; prefix: string[]; view: Exclude<View, "overview">; open: (id: string) => void }) {
  const [kind, setKind] = useState(""), [search, setSearch] = useState(""), [query, setQuery] = useState("");
  const reports = useInfiniteQuery({ ...policy, queryKey: [...prefix, "list", view, kind, query], initialPageParam: "", queryFn: ({ signal, pageParam }) => {
    const params = new URLSearchParams({ view, limit: "20" }); if (kind) params.set("kind", kind); if (query) params.set("search", query); if (pageParam) params.set("cursor", pageParam); return reportRequest(scope, `?${params}`, reportsPageSchema, signal);
  }, getNextPageParam: (last, _pages, lastParam) => last.nextCursor && last.nextCursor !== lastParam ? last.nextCursor : undefined });
  const items = reports.data?.pages.flatMap(p => p.items) ?? [];
  return <>
    <div className={styles.filters} role="group" aria-label="报告筛选">{[["", "全部"], ["TITLE_REVIEW", "标题审核"], ["SHEIN_RECORD", "资料与诊断"]].map(([value, label]) => <Button key={value} variant={value === kind ? "secondary" : "outline"} aria-pressed={kind === value} onClick={() => setKind(value)}>{label}</Button>)}<form onSubmit={e => { e.preventDefault(); if (new TextEncoder().encode(search).length <= 128) setQuery(search); }}><label className="sr-only" htmlFor="report-search">报告标题或商品关键词</label><input id="report-search" value={search} onChange={e => setSearch(e.target.value)} maxLength={128} placeholder="搜索报告或商品" /><Button type="submit" variant="outline">搜索</Button></form><Button variant="ghost" onClick={() => void reports.refetch()}>刷新</Button></div>
    {reports.isPending ? <ConsoleState kind="loading" title="正在读取报告" /> : reports.isError ? <ConsoleState kind="error" title="报告列表读取失败"><p>{errorText(reports.error)}</p><Button variant="outline" onClick={() => void reports.refetch()}>重试</Button></ConsoleState> : items.length === 0 ? <ConsoleState kind="empty" title={view === "favorites" ? "尚无收藏报告" : "当前范围暂无报告"}>使用“保存报告”保留本人已有结果，再查看或收藏。</ConsoleState> : <Card className={styles.list}><div className={styles.table}><table><thead><tr><th>名称</th><th>类型</th><th>商品 / 店铺</th><th>格式</th><th>保存时间</th></tr></thead><tbody>{items.map(item => <tr key={item.id}><td><button className={styles.rowLink} onClick={() => open(item.id)}>{item.favorite ? <Star size={18} className={styles.star} aria-label="已收藏" /> : <FileText size={18} aria-hidden="true" />}{item.title}</button></td><td>{types[item.ref.kind]}</td><td>{item.productKey}{item.storeId ? <small>店铺 {item.storeId}</small> : null}</td><td>JSON / TXT</td><td><time dateTime={item.capturedAt}>{new Date(item.capturedAt).toLocaleString("zh-CN")}</time></td></tr>)}</tbody></table></div>{reports.hasNextPage ? <Button variant="outline" disabled={reports.isFetchingNextPage} onClick={() => void reports.fetchNextPage()}>加载更多</Button> : <p>已显示当前筛选范围全部报告</p>}</Card>}
  </>;
}
function download(report: Report, format: "json" | "txt") {
  const content = format === "json" ? JSON.stringify(report, null, 2) + "\n" : reportText(report);
  const blob = new Blob([content], { type: format === "json" ? "application/json;charset=utf-8" : "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob), link = document.createElement("a"); link.href = url; link.download = `report-${report.id}.${format}`; document.body.append(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function ReportDetail({ report, manage, busy, favorite }: { report: Report; manage: boolean; busy: boolean; favorite: () => void }) {
  return <><h3>{report.title}</h3><p>历史只读快照；不代表当前批准、执行或平台发布许可。</p><dl className={styles.provenance}><dt>来源</dt><dd>{types[report.ref.kind]} · {report.ref.id}</dd><dt>来源版本</dt><dd>{report.ref.version}</dd><dt>保存时间</dt><dd>{new Date(report.capturedAt).toLocaleString("zh-CN")}</dd>{report.sourceAt ? <><dt>原结果时间</dt><dd>{new Date(report.sourceAt).toLocaleString("zh-CN")}</dd></> : null}<dt>内容 SHA-256</dt><dd>{report.digest}</dd></dl><div className={styles.actions}>{manage ? <Button variant="outline" aria-pressed={report.favorite} disabled={busy} onClick={favorite}>{report.favorite ? "取消收藏" : "收藏报告"}</Button> : null}<Button variant="outline" onClick={() => download(report, "json")}>下载 JSON</Button><Button variant="outline" onClick={() => download(report, "txt")}>下载文本</Button><Button asChild variant="outline"><Link href={sourceHref(report.ref)} prefetch={false}>打开原来源</Link></Button></div>{report.content.sections.map((section, index) => <section key={index} className={styles.section}><h4>{section.title}</h4><dl>{section.fields.map((field, n) => <div key={n}><dt>{field.label}</dt><dd>{field.value}</dd></div>)}</dl></section>)}</>;
}
