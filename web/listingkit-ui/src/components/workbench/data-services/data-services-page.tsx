"use client";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import Image from "next/image";
import { z } from "zod";
import { Download } from "lucide-react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { CollectionDialog } from "@/components/workbench/collections/collection-page";
import { dataRequest, DataAPIError, type DataScope } from "@/lib/api/data-services";
import { CUSTOM_INPUT_BYTE_LIMITS, KEY_NAME_MAX_BYTES, dataTextBytes, optionsSchema, customSummarySchema, overviewSchema, jobSchema, keySchema, keyCreatedSchema, keyHistorySchema, resultPageSchema, customSchema, querySchema, keyLimitsSchema, type DataOptions, type DataQuery, type DataKey, type DataJob, type CustomRequest, type Overview } from "@/lib/contracts/data-services";
import { useDataCommand } from "./use-data-command";
import styles from "./data-services.module.css";
export const stateNames: Record<string, string> = { ADMITTED: "已提交", RUNNING: "抓取中", SUCCEEDED: "已完成", PARTIAL: "部分完成", FAILED: "失败", CANCELED: "已取消", ACTIVE: "启用", DISABLED: "禁用", REVOKED: "已撤销", SUBMITTED: "已提交", EVALUATING: "需求评估", SPEC_CONFIRMED: "规格已确认", PREPARING: "数据制作中", DELIVERED: "已交付", CLOSED: "已关闭", SAVED: "已保存", PREPARED: "等待抓取", FETCHING: "抓取中", PREPARED_EVIDENCE: "等待保存" };
const failureNames: Record<string, string> = { DATA_UNKNOWN: "结果待核实，请继续核实原请求。", DATA_UNAVAILABLE: "数据服务暂不可用，请稍后刷新。", FORBIDDEN: "当前身份或权限已失效，请确认登录及企业。", DATA_CONFLICT: "资料或额度已发生变化，请刷新后确认。", DATA_NOT_FOUND: "未找到当前身份下的记录。", INVALID_DATA_REQUEST: "请检查输入、格式、条数和额度。", ORGANIZATION_CONTEXT_CHANGED: "当前企业已变化，请重新进入页面。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化，请重新进入页面。", PENDING_COMMAND: "请先核实上一次请求。", INTENT_STORAGE_UNAVAILABLE: "无法保存原请求，请恢复浏览器存储后再提交。", AUTHENTICATION_REQUIRED: "请先登录。", access_revoked: "权限已撤销", deadline: "任务已到截止时间", resource_unavailable: "数据条数资源不足", provider_failed: "来源页面未能取得有效数据", provider_rejected: "来源页面未能取得有效数据", provider_challenged: "Amazon 页面要求验证，本次获取已停止。", provider_unsupported: "Amazon 页面结构暂不支持，本次获取已停止。", canceled: "已取消", discovery_failed: "未能完成来源搜索" };
const money = (fen: number) => `¥${(fen / 100).toFixed(2)}`;
export const moment = (value: string) => new Date(value).toLocaleString("zh-CN");
export function Field({ title, children }: {
    title: string;
    children: ReactNode;
}) { return <label className="grid gap-2 text-sm font-medium">{title}{children}</label>; }
export function ByteLimitHint({ id, value, maximum }: { id: string; value: string; maximum: number }) {
    const length = dataTextBytes(value);
    return <span id={id} className="text-xs font-normal text-muted-foreground">当前 {length} / {maximum} 字节{length > maximum ? <span className="block text-amber-700">超出字节上限，请缩短内容。</span> : null}</span>;
}
export function DataNotice({ error }: {
    error: string;
}) { return error ? <p role="alert" className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">{failureNames[error] ?? "请求未完成，请刷新或核实原请求。"}</p> : null; }
function download(value: unknown, name: string) { const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: "application/json" })); const a = document.createElement("a"); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url); }
export function DataServicesPage({ mode }: {
    mode: "market" | "api";
}) {
    const context = useWorkbenchContext();
    const userId = context.user?.id, organizationId = context.effectiveOrganization?.id;
    const scope = useMemo(() => userId && organizationId ? { userId, organizationId } : null, [userId, organizationId]);
    if (context.isLoading || context.isSwitching)
        return <p className="p-8" role="status">正在确认当前企业…</p>;
    if (!scope || context.selectionRequired || context.error || context.blockingError)
        return <p className="p-8" role="alert">请先确认登录身份与当前企业。</p>;
    return mode === "market" ? <Market key={`${userId}:${organizationId}`} scope={scope}/> : <APIManagement key={`${userId}:${organizationId}`} scope={scope}/>;
}
function Frame({ title, subtitle, actions, children }: {
    title: string;
    subtitle: string;
    actions?: ReactNode;
    children: ReactNode;
}) { return <main className={styles.page}><p className={styles.breadcrumb}>数据服务 <span className="mx-2">/</span> {title}</p><header className={styles.header}><div><h1>{title}</h1><p>{subtitle}</p></div><div className={styles.actions}>{actions}</div></header><div className={styles.content}>{children}</div></main>; }
function Recovery({ command, onRecovered }: {
    command: ReturnType<typeof useDataCommand>;
    onRecovered: (value: unknown, path: string) => void;
}) {
    return <><DataNotice error={command.error}/>{command.pending ? <div role="status" className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900"><span>原请求 {command.pending.key} 尚待核实。核实会保留原条件及请求编号。</span><Button disabled={command.busy} variant="outline" onClick={async () => {
                const path = command.pending?.path ?? "";
                const value = await command.recover();
                if (value !== undefined)
                    onRecovered(value, path);
            }}>核实／继续原请求</Button></div> : null}</>;
}
function Market({ scope }: {
    scope: DataScope;
}) {
    const [options, setOptions] = useState<DataOptions | null>(null), [jobs, setJobs] = useState<DataJob[]>([]), [customs, setCustoms] = useState<z.infer<typeof customSummarySchema>[]>([]), [error, setError] = useState("");
    const [dialog, setDialog] = useState<"realtime" | "custom" | null>(null), [job, setJob] = useState<DataJob | null>(null), [custom, setCustom] = useState<CustomRequest | null>(null), [refresh, setRefresh] = useState(0);
    const command = useDataCommand(scope);
    const { registerOrganizationSwitchGuard } = useWorkbenchContext();
    useEffect(() => registerOrganizationSwitchGuard(() => !command.pending && !command.busy), [registerOrganizationSwitchGuard, command.pending, command.busy]);
    useEffect(() => {
        const controller = new AbortController();
        Promise.all([dataRequest(scope, "options", optionsSchema, undefined, controller.signal), dataRequest(scope, "amazon/jobs", z.array(jobSchema).max(100), undefined, controller.signal), dataRequest(scope, "custom", z.array(customSummarySchema).max(100), undefined, controller.signal)]).then(([o, j, c]) => { setOptions(o); setJobs(j); setCustoms(c); }).catch(e => {
            if (!controller.signal.aborted)
                setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
        });
        return () => controller.abort();
    }, [scope, refresh]);
    const received = (value: unknown, path: string) => {
        if (path === "custom") {
            const r = customSchema.safeParse(value);
            if (r.success)
                setCustom(r.data);
        }
        else {
            const r = jobSchema.safeParse(value);
            if (r.success)
                setJob(r.data);
        }
        setDialog(null);
        setRefresh(n => n + 1);
    };
    const disabled = command.busy || !!command.pending;
    return <Frame title="数据市场" subtitle="汇集电商平台产品数据，当前提供 Amazon 实时抓取与数据集定制。" actions={<><Button variant="outline" asChild><Link href="/workbench/data/api">API管理</Link></Button><Button variant="outline" asChild><Link href="/workbench/data/mine">我的数据</Link></Button></>}>
  <DataNotice error={error}/><Recovery command={command} onRecovered={received}/>
  <Card className={`${styles.card} ${styles.platform}`}>
    <div><span className={`${styles.badge} ${!options?.acquisitionReady ? styles.neutralBadge : ""}`}>{options ? options.acquisitionReady ? "已配置 1 个来源" : "实时抓取待配置" : "正在读取配置"}</span><h2>选择数据来源</h2><p>当前支持 Amazon 产品数据，按业务场景选择获取方式。</p><div className={styles.platformTags}><span>数据平台</span><span className={styles.badge}>Amazon · 当前选择</span><span className={`${styles.badge} ${styles.neutralBadge}`}>其他平台暂未开放</span></div></div>
    <dl className={styles.platformFacts}>{[["当前选择", "Amazon"], ["数据类型", "产品数据"], ["获取方式", "实时抓取 / 数据集定制"], ["交付格式", "Excel / CSV / JSON"]].map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
  </Card>
  <section className={styles.serviceArea}><div className={styles.sectionTitle}><h2>Amazon 产品数据获取方式</h2><p>按需求选择，获取结果统一保存到“我的数据”</p></div><div className={styles.services}>
    <Card className={`${styles.card} ${styles.service}`}><div className={styles.serviceBadges}><span className={styles.badge}>获取方式 01 · 实时抓取</span><span className={styles.badge}>按成功数据计费</span></div><h3>实时数据抓取</h3><p>适合即时选品、竞品分析与商品信息补全。</p><ServiceBullets items={["选择站点、关键词、类目或 ASIN", "设置抓取数量和所需数据字段", "确认最大条数与资源消耗", "成功结果自动保存到“我的数据”"]}/><div className={styles.serviceFooter}><p>¥0.05 / 成功保存一条 · 使用预付 DATA_ROW</p><Button disabled={disabled || !options?.acquisitionReady} onClick={() => setDialog("realtime")}>开始抓取</Button></div>{options && !options.acquisitionReady ? <p role="status" className="mt-3 text-xs text-muted-foreground">{options.unavailableReason}</p> : null}</Card>
    <Card className={`${styles.card} ${styles.service} ${styles.purple}`}><div className={styles.serviceBadges}><span className={`${styles.badge} ${styles.purpleBadge}`}>获取方式 02 · 数据集定制</span><span className={`${styles.badge} ${styles.purpleBadge}`}>根据需求单独报价</span></div><h3>数据集定制</h3><p>适合指定范围、指定字段与批量数据交付。</p><ServiceBullets purple items={["描述用途、数据范围与交付要求", "平台专员线下确认规格与报价", "在系统中查看制作与交付进度", "交付数据保存到申请者“我的数据”"]}/><div className={styles.serviceFooter}><p>报价与规格线下确认 · 系统记录进度与交付</p><Button disabled={disabled || !options} onClick={() => setDialog("custom")}>提交定制需求</Button></div></Card>
  </div></section>
  <Card className={`${styles.card} ${styles.process}`}><div className={styles.cardHeading}><h2>购买与交付流程</h2><p>两种方式都交付到“我的数据”，方便后续查看与使用</p></div><ProcessRow label="实时抓取" steps={["配置抓取条件", "确认预估费用", "确认并开始抓取", "保存到我的数据"]}/><ProcessRow purple label="数据集定制" steps={["提交定制需求", "确认规格与报价", "数据制作与交付", "保存到我的数据"]}/></Card>
  <Card className="gap-4 p-6"><div className="flex justify-between"><h2 className="font-semibold">我的抓取与定制记录</h2><Button variant="ghost" size="sm" onClick={() => setRefresh(n => n + 1)}>刷新</Button></div><p className="text-xs text-muted-foreground">展示最近 100 条抓取任务和定制申请。</p>{jobs.length === 0 && customs.length === 0 ? <p className="py-5 text-center text-sm text-muted-foreground">暂无记录，提交后会在这里显示真实进度。</p> : <div className="grid gap-3 md:grid-cols-2">{jobs.map(j => <button key={j.id} className="rounded-lg border p-4 text-left hover:bg-muted/50" onClick={() => setJob(j)}><p className="font-medium">Amazon {j.query.site.toUpperCase()} · {j.query.mode}</p><p className="mt-1 text-sm text-muted-foreground">{stateNames[j.state]} · 已保存 {j.saved} 条 · {moment(j.createdAt)}</p></button>)}{customs.map(c => <button key={c.id} className="rounded-lg border p-4 text-left hover:bg-muted/50" onClick={async () => {
                    try {
                        setCustom(await dataRequest(scope, `custom/${c.id}`, customSchema));
                    }
                    catch (e) {
                        setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
                    }
                }}><p className="font-medium">{c.name}</p><p className="mt-1 text-sm text-muted-foreground">{stateNames[c.state]} · {moment(c.createdAt)}</p></button>)}</div>}</Card>
  {dialog && options ? <CollectionDialog title={dialog === "realtime" ? "Amazon 实时抓取" : "提交数据集定制需求"} onClose={() => setDialog(null)}><QueryForm options={options} custom={dialog === "custom"} disabled={disabled} onSubmit={async (q, extra) => {
                const path = dialog === "custom" ? "custom" : "amazon/jobs";
                const value = await command.run(path, dialog === "custom" ? { ...extra, query: q } : { query: q, maximumRows: q.limit, maximumCostFen: q.limit * 5 });
                if (value !== undefined)
                    received(value, path);
            }}/></CollectionDialog> : null}
  {job ? <JobDialog scope={scope} job={job} onClose={() => setJob(null)} onCancel={async (j) => {
                const value = await command.run(`amazon/jobs/${j.id}/cancel`, {});
                if (value !== undefined) {
                    setJob(jobSchema.parse(value));
                    setRefresh(n => n + 1);
                }
            }} disabled={disabled}/> : null}
  {custom ? <CollectionDialog title={custom.input.name} onClose={() => setCustom(null)}><CustomDetails request={custom}/><Button variant="outline" onClick={async () => {
                try {
                    setCustom(await dataRequest(scope, `custom/${custom.id}`, customSchema));
                }
                catch (e) {
                    setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
                }
            }}>刷新进度</Button></CollectionDialog> : null}
 </Frame>;
}
function ServiceBullets({ items, purple = false }: {
    items: string[];
    purple?: boolean;
}) {
    return <ul className={styles.bullets}>{items.map(item => <li key={item}><Image src={`/data-services/${purple ? "custom" : "realtime"}-dot.svg`} width={6} height={6} alt=""/>{item}</li>)}</ul>;
}
function ProcessRow({ label, steps, purple = false }: {
    label: string;
    steps: string[];
    purple?: boolean;
}) {
    return <div className={styles.processRow}><span className={`${styles.badge} ${purple ? styles.purpleBadge : ""}`}>{label}</span><ol className={`${styles.steps} ${purple ? styles.purpleSteps : ""}`}>{steps.map((step, i) => <li key={step} className="contents">{i ? <span aria-hidden="true">→</span> : null}<span className={styles.step}>{step}</span></li>)}</ol></div>;
}
function QueryForm({ options, custom = false, disabled, onSubmit }: {
    options: DataOptions;
    custom?: boolean;
    disabled: boolean;
    onSubmit: (query: DataQuery, extra: Record<string, string>) => void;
}) {
    const sites = custom ? options.customSites : options.sites;
    const [site, setSite] = useState(sites[0]?.code ?? "us"), [mode, setMode] = useState<DataQuery["mode"]>("asin"), [asins, setASINs] = useState(""), [keyword, setKeyword] = useState(""), [node, setNode] = useState(""), [limit, setLimit] = useState("10"), [fields, setFields] = useState<string[]>(options.fields), [error, setError] = useState("");
    const [name, setName] = useState(""), [purpose, setPurpose] = useState(""), [format, setFormat] = useState("csv"), [timeRange, setTimeRange] = useState(""), [notes, setNotes] = useState("");
    const customBytes = { name, purpose, timeRange, notes };
    const customLength = (field: keyof typeof customBytes) => dataTextBytes(customBytes[field]);
    const exceedsLimit = (field: keyof typeof customBytes) => customLength(field) > CUSTOM_INPUT_BYTE_LIMITS[field];
    const customTooLong = custom && (Object.keys(customBytes) as (keyof typeof customBytes)[]).some(exceedsLimit);
    const lengthHint = (field: keyof typeof customBytes) => <ByteLimitHint id={`custom-${field}-limit`} value={customBytes[field]} maximum={CUSTOM_INPUT_BYTE_LIMITS[field]}/>;
    return <form className="space-y-4" onSubmit={e => {
            e.preventDefault();
            const input = { site, mode, ...(mode === "asin" ? { asins: asins.split(/[\s,，]+/).filter(Boolean) } : { ...(mode === "keyword" && keyword.trim() ? { keyword: keyword.trim() } : {}), ...(node ? { categoryNode: node } : {}) }), limit: Number(limit), fields };
            const parsed = querySchema.safeParse(input);
            if (customTooLong || !parsed.success || (mode === "keyword" && !keyword.trim()) || (mode === "category" && !node) || !fields.length) {
                setError("INVALID_DATA_REQUEST");
                return;
            }
            ;
            setError("");
            onSubmit(parsed.data, { name, purpose, format, timeRange, notes });
        }}>
  {custom ? <Field title="需求名称"><Input required aria-label="需求名称" aria-describedby="custom-name-limit" aria-invalid={exceedsLimit("name")} maxLength={CUSTOM_INPUT_BYTE_LIMITS.name} value={name} onChange={e => setName(e.target.value)}/>{lengthHint("name")}</Field> : null}
  <div className="grid gap-4 sm:grid-cols-2"><Field title="Amazon 站点"><Select value={site} onChange={e => setSite(e.target.value as DataQuery["site"])}>{sites.map(s => <option key={s.code} value={s.code}>{s.name} · {s.domain}</option>)}</Select></Field><Field title="输入方式"><Select value={mode} onChange={e => setMode(e.target.value as DataQuery["mode"])}><option value="asin">ASIN／商品链接</option><option value="keyword">关键词</option><option value="category">类目</option></Select></Field></div>
  {mode === "asin" ? <Field title="ASIN 或当前站点 HTTPS 商品链接（每行一个）"><Textarea required rows={4} value={asins} onChange={e => setASINs(e.target.value)} placeholder="B0… 或 https://www.amazon…/dp/…"/></Field> : <div className="grid gap-4 sm:grid-cols-2">{mode === "keyword" ? <Field title="关键词"><Input required maxLength={200} value={keyword} onChange={e => setKeyword(e.target.value)}/></Field> : null}<Field title={mode === "category" ? "Amazon 原生类目节点 ID" : "类目节点 ID（可选）"}><Input required={mode === "category"} pattern="[0-9]{1,20}" maxLength={20} value={node} onChange={e => setNode(e.target.value)} placeholder="当前站点数字 browse node"/></Field></div>}
  <Field title="最大数据条数"><Input required type="number" min={1} max={200} step={1} value={limit} onChange={e => setLimit(e.target.value)}/></Field><fieldset className="rounded-lg border p-4"><legend className="px-2 text-sm font-medium">所需字段</legend><div className="grid gap-2 sm:grid-cols-3">{options.fields.map(f => <label key={f} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={fields.includes(f)} onChange={e => setFields(v => e.target.checked ? [...v, f] : v.filter(x => x !== f))}/>{f}</label>)}</div></fieldset>
  {custom ? <><Field title="用途与业务场景"><Textarea required aria-label="用途与业务场景" aria-describedby="custom-purpose-limit" aria-invalid={exceedsLimit("purpose")} maxLength={CUSTOM_INPUT_BYTE_LIMITS.purpose} value={purpose} onChange={e => setPurpose(e.target.value)}/>{lengthHint("purpose")}</Field><div className="grid gap-4 sm:grid-cols-2"><Field title="期望交付格式"><Select value={format} onChange={e => setFormat(e.target.value)}>{options.formats.map(f => <option key={f}>{f}</option>)}</Select></Field><Field title="时间范围／更新需求"><Input aria-label="时间范围／更新需求" aria-describedby="custom-timeRange-limit" aria-invalid={exceedsLimit("timeRange")} maxLength={CUSTOM_INPUT_BYTE_LIMITS.timeRange} value={timeRange} onChange={e => setTimeRange(e.target.value)}/>{lengthHint("timeRange")}</Field></div><Field title="其他说明"><Textarea aria-label="其他说明" aria-describedby="custom-notes-limit" aria-invalid={exceedsLimit("notes")} maxLength={CUSTOM_INPUT_BYTE_LIMITS.notes} value={notes} onChange={e => setNotes(e.target.value)}/>{lengthHint("notes")}</Field><p className="text-sm text-muted-foreground">规格与报价由专员线下确认。定制交付最多 200 条/批，不消费实时抓取的 DATA_ROW。</p></> : <p className="rounded-lg bg-muted p-4 text-sm">确认最多获取 {limit || "—"} 条，最多消费 {limit || "—"} 个 DATA_ROW（{money((Number(limit) || 0) * 5)}）。失败且未保存的条目不计费，已保存数据会保留。来源缺失字段会明确标记。</p>}
  <DataNotice error={error}/><Button type="submit" disabled={disabled || customTooLong || (!custom && !options.acquisitionReady)}>{disabled ? "正在提交…" : custom ? "提交定制需求" : "确认并开始抓取"}</Button>
 </form>;
}
export function CustomDetails({ request: r }: {
    request: CustomRequest;
}) { return <div className="space-y-4"><p className="text-sm">状态：<strong>{stateNames[r.state]}</strong> · 原申请 {r.id}</p><p className="text-sm text-muted-foreground">Amazon {r.input.query.site.toUpperCase()} · {r.input.query.mode} · 最多 {r.input.query.limit} 条 · {r.input.format}</p><dl className="grid gap-3 text-sm"><div><dt className="text-muted-foreground">用途与业务场景</dt><dd className="whitespace-pre-wrap break-words">{r.input.purpose}</dd></div>{r.input.query.keyword ? <div><dt className="text-muted-foreground">关键词</dt><dd className="whitespace-pre-wrap break-words">{r.input.query.keyword}</dd></div> : null}{r.input.query.categoryNode ? <div><dt className="text-muted-foreground">类目节点</dt><dd>{r.input.query.categoryNode}</dd></div> : null}{r.input.query.asins?.length ? <div><dt className="text-muted-foreground">ASIN</dt><dd className="whitespace-pre-wrap break-all">{r.input.query.asins.join("、")}</dd></div> : null}<div><dt className="text-muted-foreground">所需字段</dt><dd>{r.input.query.fields?.length ? r.input.query.fields.join("、") : "默认商品字段"}</dd></div>{r.input.timeRange ? <div><dt className="text-muted-foreground">时间范围</dt><dd className="whitespace-pre-wrap break-words">{r.input.timeRange}</dd></div> : null}{r.input.notes ? <div><dt className="text-muted-foreground">补充说明</dt><dd className="whitespace-pre-wrap break-words">{r.input.notes}</dd></div> : null}</dl>{r.spec ? <div className="space-y-2 rounded-lg bg-muted p-4 text-sm"><p>确认规格：{r.spec.description}</p><p>报价记录：{r.spec.quoteNote}</p><p>确认记录：{r.spec.confirmationNote}</p><p>规格版本 {r.specRevision} · {r.spec.format} · 最多 {r.spec.maximumRows} 条</p></div> : null}{r.batchId ? <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">已交付 {r.deliveredRows} 条 · 批次 {r.batchId}<Button className="ml-3" size="sm" asChild><Link href={`/workbench/data/mine?batchId=${r.batchId}`}>在我的数据中查看</Link></Button></div> : null}<h3 className="font-semibold">进度记录</h3><ol className="space-y-3 border-l pl-4">{r.events.map(event => <li className="text-sm" key={event.revision}><p className="font-medium">{stateNames[event.state]} · {moment(event.at)}</p><p className="whitespace-pre-wrap text-muted-foreground">{event.note}</p></li>)}</ol>{r.nextEventBefore ? <p className="text-xs text-muted-foreground">当前展示最近 100 条进度记录。</p> : null}</div>; }
function JobDialog({ scope, job: initial, onClose, onCancel, disabled = false }: {
    scope: DataScope;
    job: DataJob;
    onClose: () => void;
    onCancel?: (job: DataJob) => void;
    disabled?: boolean;
}) {
    const [job, setJob] = useState(initial), [page, setPage] = useState<z.infer<typeof resultPageSchema> | null>(null), [cursor, setCursor] = useState(""), [error, setError] = useState(""), [reload, setReload] = useState(0), [loading, setLoading] = useState(false);
    useEffect(() => {
        const controller = new AbortController();
        Promise.all([dataRequest(scope, `amazon/jobs/${initial.id}`, jobSchema, undefined, controller.signal), dataRequest(scope, `amazon/jobs/${initial.id}/results${cursor ? `?cursor=${cursor}` : ""}`, resultPageSchema, undefined, controller.signal)]).then(([j, p]) => { setJob(j); setPage(p); }).catch(e => {
            if (!controller.signal.aborted)
                setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
        }).finally(() => {
            if (!controller.signal.aborted)
                setLoading(false);
        });
        return () => controller.abort();
    }, [scope, initial.id, initial.state, cursor, reload]);
    return <CollectionDialog title="抓取任务与结果" onClose={onClose}><DataNotice error={error}/><div className="grid gap-3 text-sm sm:grid-cols-3"><p>状态：{stateNames[job.state]}</p><p>已保存 {job.saved} · 失败 {job.failed} · 等待 {job.pending}</p><p>已确认 {money(job.confirmedFen)} · 待确认 {money(job.pendingFen)}</p></div>{job.reason ? <p className="text-sm text-amber-700">{failureNames[job.reason] ?? "本次任务未全部完成"}</p> : null}<p className="text-xs text-muted-foreground">任务 {job.id} · 截止 {moment(job.deadline)}</p><div className="flex flex-wrap gap-3"><Button variant="outline" disabled={loading} onClick={() => setReload(n => n + 1)}>刷新进度</Button>{onCancel && ["ADMITTED", "RUNNING"].includes(job.state) ? <Button variant="outline" disabled={disabled} onClick={() => onCancel(job)}>取消未保存项</Button> : null}{job.batchId ? <Button asChild><Link href={`/workbench/data/mine?batchId=${job.batchId}`}>查看我的数据批次</Link></Button> : null}{page?.items.some(i => i.state === "SAVED") ? <Button variant="outline" onClick={() => download(page.items.filter(i => i.state === "SAVED"), `amazon-${job.id}.json`)}><Download size={16}/>下载本页 JSON</Button> : null}</div>{loading ? <p role="status">正在读取结果…</p> : null}{page ? <div className="space-y-3">{page.items.map(item => <article className="rounded-lg border p-4" key={item.id}><p className="text-sm font-semibold">{stateNames[item.state]}{item.reason ? ` · ${failureNames[item.reason] ?? "来源未完成"}` : ""}</p>{item.data ? <pre className="mt-2 max-h-60 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 text-xs">{JSON.stringify(item.data, null, 2)}</pre> : null}{item.missing?.length ? <p className="mt-2 text-xs text-amber-700">本次未取得：{item.missing.join("、")}</p> : null}</article>)}{!page.items.length ? <p className="text-sm text-muted-foreground">暂未形成结果，刷新以查看进度。</p> : null}<div className="flex gap-2">{cursor ? <Button variant="outline" onClick={() => setCursor("")}>返回首页</Button> : null}{page.nextCursor ? <Button variant="outline" onClick={() => setCursor(page.nextCursor!)}>下一页结果</Button> : null}</div></div> : null}</CollectionDialog>;
}
function APIManagement({ scope }: {
    scope: DataScope;
}) {
    const [overview, setOverview] = useState<Overview | null>(null), [overviewError, setOverviewError] = useState(""), [keys, setKeys] = useState<DataKey[] | null>(null), [error, setError] = useState(""), [refresh, setRefresh] = useState(0), [tab, setTab] = useState("overview"), [dialog, setDialog] = useState<"create" | "docs" | null>(null), [editing, setEditing] = useState<DataKey | null>(null), [confirmation, setConfirmation] = useState<{
        key: DataKey;
        state: "DISABLED" | "ACTIVE" | "REVOKED";
    } | null>(null), [secret, setSecret] = useState<{
        key: DataKey;
        value?: string;
    } | null>(null), [job, setJob] = useState<DataJob | null>(null), [jobs, setJobs] = useState<DataJob[] | null>(null), [history, setHistory] = useState<DataKey[]>([]), [nextHistory, setNextHistory] = useState(""), [historyLoaded, setHistoryLoaded] = useState(false);
    const command = useDataCommand(scope);
    const { registerOrganizationSwitchGuard } = useWorkbenchContext();
    useEffect(() => registerOrganizationSwitchGuard(() => !command.pending && !command.busy), [registerOrganizationSwitchGuard, command.pending, command.busy]);
    useEffect(() => {
        const controller = new AbortController();
        dataRequest(scope, "keys", z.array(keySchema).max(20), undefined, controller.signal).then(value => {
            if (!controller.signal.aborted)
                setKeys(value);
        }).catch(e => {
            if (!controller.signal.aborted) {
                setKeys(null);
                setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
            }
        });
        dataRequest(scope, "overview", overviewSchema, undefined, controller.signal).then(value => {
            if (!controller.signal.aborted)
                setOverview(value);
        }).catch(e => {
            if (!controller.signal.aborted) {
                setOverview(null);
                setOverviewError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
            }
        });
        return () => controller.abort();
    }, [scope, refresh]);
    useEffect(() => {
        if (tab !== "calls")
            return;
        const controller = new AbortController();
        dataRequest(scope, "amazon/jobs", z.array(jobSchema).max(100), undefined, controller.signal).then(value => {
            if (!controller.signal.aborted)
                setJobs(value);
        }).catch(e => {
            if (!controller.signal.aborted)
                setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
        });
        return () => controller.abort();
    }, [scope, tab, refresh]);
    const reload = () => {
        setKeys(null);
        setOverview(null);
        setJobs(null);
        setError("");
        setOverviewError("");
        setRefresh(n => n + 1);
    };
    const received = (value: unknown, path: string) => {
        if (path === "keys") {
            const created = keyCreatedSchema.safeParse(value);
            if (created.success)
                setSecret({ key: created.data.key, ...(created.data.secret ? { value: `DataKey ${created.data.key.id}.${created.data.secret}` } : {}) });
            else {
                const key = keySchema.safeParse(value);
                if (key.success)
                    setSecret({ key: key.data });
            }
        }
        setDialog(null);
        setEditing(null);
        setConfirmation(null);
        reload();
    };
    const usage = overview?.usage, disabled = command.busy || !!command.pending;
    const keyTable = (keys: DataKey[], controls: boolean, compact = false) => <div className={`${styles.table} ${compact ? styles.compactTable : ""}`}><table><thead><tr>{["名称", "密钥", compact ? "额度限制" : "权限／额度", ...(!compact ? ["有效期"] : []), "状态", ...(controls ? ["操作"] : [])].map(x => <th key={x}>{x}</th>)}</tr></thead><tbody>{keys.map(key => <tr key={key.id}><td className="font-medium">{key.limits.name}</td><td className="font-mono text-xs">••••{key.suffix}</td><td>{!compact ? <p>{key.limits.permissions.map(p => p === "amazon.acquire" ? "抓取" : "读结果").join(" / ")}</p> : null}<p className={compact ? "text-muted-foreground" : "mt-1 text-muted-foreground"}>每日 {key.limits.dailyRows} 条 · 月预算 {money(key.limits.monthlyCostFen)}</p></td>{!compact ? <td>{moment(key.limits.expiresAt)}</td> : null}<td><span className={`${styles.badge} ${key.state !== "ACTIVE" ? styles.neutralBadge : ""}`}>{stateNames[key.state]}</span></td>{controls ? <td><div className="flex gap-2"><Button size="sm" variant="ghost" disabled={disabled} onClick={() => setEditing(key)}>编辑</Button><Button size="sm" variant="ghost" disabled={disabled} onClick={() => setConfirmation({ key, state: key.state === "ACTIVE" ? "DISABLED" : "ACTIVE" })}>{key.state === "ACTIVE" ? "禁用" : "启用"}</Button><Button size="sm" variant="ghost" disabled={disabled} onClick={() => setConfirmation({ key, state: "REVOKED" })}>撤销</Button></div></td> : null}</tr>)}</tbody></table>{!keys.length ? <p className={styles.empty}>暂无密钥</p> : null}</div>;
    const callTable = (records: DataJob[]) => <div className={styles.table}><table><thead><tr>{["任务／来源", "请求时间", "状态", "保存条数", "已确认／待确认", "结果"].map(s => <th key={s}>{s}</th>)}</tr></thead><tbody>{records.map(j => <tr key={j.id}><td><p>Amazon {j.query.site.toUpperCase()}</p><p className="text-xs text-muted-foreground">{j.credentialId ? "API 密钥调用" : "控制台抓取"}</p></td><td>{moment(j.createdAt)}</td><td>{stateNames[j.state]}</td><td>{j.saved}</td><td>{money(j.confirmedFen)} / {money(j.pendingFen)}</td><td><Button variant="ghost" size="sm" onClick={() => setJob(j)}>查看</Button></td></tr>)}</tbody></table>{!records.length ? <p className={styles.empty}>暂无调用记录</p> : null}</div>;
    return <Frame title="API 管理" subtitle="管理 API 接入、密钥、调用记录与额度，便于业务系统获取产品数据。" actions={<><Button variant="outline" onClick={() => setDialog("docs")}>开发文档</Button><Button disabled={disabled || keys === null} onClick={() => setDialog("create")}>创建API密钥</Button></>}>
  <DataNotice error={error}/><Recovery command={command} onRecovered={received}/><nav aria-label="API 管理内容" className={styles.tabs}>{[["overview", "概览"], ["keys", "API 密钥"], ["calls", "调用记录"], ["usage", "用量与费用"]].map(([value, title]) => <button key={value} aria-current={tab === value ? "page" : undefined} onClick={() => { if (value !== tab && value === "calls") setJobs(null); setTab(value); }}>{title}</button>)}<span className={`${styles.badge} ${!overview?.options.acquisitionReady ? styles.neutralBadge : ""}`}>{overview ? overview.options.acquisitionReady ? "抓取环境已配置" : "抓取环境待配置" : overviewError ? "配置未读取" : "正在读取配置"}</span></nav>
  {keys === null && !error ? <p role="status">正在读取密钥…</p> : null}{!overview && !overviewError ? <p role="status">正在读取实际用量…</p> : null}{overviewError && keys !== null ? <p role="status" className="text-sm text-muted-foreground">用量、费用与抓取配置暂未读取；密钥管理仍可使用。{overviewError === "FORBIDDEN" ? " 当前身份没有相关读取权限。" : " 请稍后刷新重试。"}</p> : null}
  {keys !== null ? <>{overview ? <div className={styles.stats}>{[{ title: "启用的密钥", value: String(keys.filter(k => k.state === "ACTIVE").length) }, { title: "今日保存数据", value: `${usage!.dayRows} 条` }, { title: "本月已确认费用", value: money(usage!.monthConfirmedFen) }, { title: "本月任务成功率", value: usage!.successRate === null ? "暂无样本" : `${(usage!.successRate * 100).toFixed(1)}%` }].map(({ title, value }) => <Card key={title} className={styles.stat}><h2>{title}</h2><p>{value}</p></Card>)}</div> : null}
   {tab === "overview" ? <><div className={styles.apiGrid}><Card className={styles.card}><div className={styles.cardHeading}><div><h2>API 接入与密钥</h2><p>凭据绑定创建者与企业，原身份撤权后失效</p></div><Button variant="outline" size="sm" onClick={() => setTab("keys")}>管理密钥</Button></div><div className={styles.capability}><div><h3>Amazon 产品数据 API</h3><p>¥0.05 / 成功保存一条 · 使用预付 DATA_ROW</p></div><span className={`${styles.badge} ${!overview?.options.acquisitionReady ? styles.neutralBadge : ""}`}>{overview ? overview.options.acquisitionReady ? `已配置 ${overview.options.sites.length} 个站点` : "抓取环境待配置" : "配置未读取"}</span></div>{keyTable(keys.slice(0, 2), false, true)}<p className={styles.caption}>完整密钥仅在创建时显示一次；历史凭据不会重放明文。</p></Card>{overview ? <Card className={styles.card}><div className={styles.cardHeading}><h2>用量与额度</h2><Button variant="outline" size="sm" onClick={() => setTab("usage")}>查看详情</Button></div><BudgetPreview overview={overview}/><p className="text-xs text-muted-foreground">已保存待确认 {money(usage!.monthPendingFen)}</p><p className={styles.caption}>预算占用包含待确认项与预留量；实际费用以确认金额为准。</p></Card> : null}</div>{overview ? <Card className={`${styles.card} ${styles.recent}`}><div className={styles.cardHeading}><div><h2>最近调用记录</h2><p>本人实时抓取与 API 任务 · UTC 日/月统计</p></div><Button variant="outline" size="sm" onClick={() => setTab("calls")}>查看全部</Button></div>{callTable(overview.jobs)}<p className={styles.caption}>成功率仅统计本月已结束任务；没有已结束任务时显示“暂无样本”。</p></Card> : null}</> : <p className="text-xs text-muted-foreground">用量包含本人实时抓取与 API 任务；按 UTC 日/月统计，成功率仅计算本月已结束任务。</p>}
   {tab === "keys" ? <Card className="gap-4 p-6"><h2 className="font-semibold">有效 API 密钥</h2><p className="text-sm text-muted-foreground">密钥绑定当前创建者及企业。撤权或离职后失效；禁用的有效密钥仍占用名额，最多 20 个。</p>{keyTable(keys, true)}<Button variant="outline" disabled={historyLoaded && !nextHistory} onClick={async () => {
                    try {
                        const p = await dataRequest(scope, `keys/history${nextHistory ? `?cursor=${nextHistory}` : ""}`, keyHistorySchema);
                        setHistory(v => nextHistory ? [...v, ...p.items] : p.items);
                        setNextHistory(p.nextCursor ?? "");
                        setHistoryLoaded(true);
                    }
                    catch (e) {
                        setError(e instanceof DataAPIError ? e.code : "DATA_UNAVAILABLE");
                    }
                }}>{historyLoaded ? nextHistory ? "读取更多历史密钥" : "历史密钥已全部读取" : "查看已撤销／过期密钥"}</Button>{history.length ? keyTable(history, false) : null}</Card> : null}
   {tab === "calls" ? <Card className="gap-4 p-6"><div className="flex justify-between"><h2 className="font-semibold">最近 100 条任务调用记录</h2><Button variant="ghost" size="sm" onClick={reload}>刷新</Button></div>{jobs !== null ? callTable(jobs) : <p role="status">调用记录尚未读取。</p>}</Card> : null}
   {tab === "usage" && overview ? <Card className="gap-4 p-6"><h2 className="font-semibold">用量与费用</h2><p className="text-sm">本月已确认：{money(usage!.monthConfirmedFen)} · 已保存待确认：{money(usage!.monthPendingFen)}</p><p className="text-sm text-muted-foreground">成功保存才计量；抓取失败或取消的未保存项不计费。取消不会删除已保存数据，定制数据不计入实时抓取消耗。</p><div className="grid gap-3 sm:grid-cols-2">{overview.keyQuotas.map(q => { const key = keys.find(k => k.id === q.keyId); return key ? <div key={q.keyId} className="rounded-lg border p-4 text-sm"><p className="font-semibold">{key.limits.name}</p><p className="mt-2">当日已保存 {q.dayConsumedRows} 条 · 预留 {q.dayReservedRows} 条 · 上限 {key.limits.dailyRows} 条</p><p className="mt-2">月预算已计量 {money(q.monthConsumedFen)} · 预留 {money(q.monthReservedFen)} · 上限 {money(key.limits.monthlyCostFen)}</p><p className="mt-2 text-xs text-muted-foreground">预算计量包含已保存待确认项；实际费用以上方确认金额为准。</p></div> : null; })}</div>{keyTable(keys, false)}<Button variant="outline" asChild><Link href="/workbench/account/organization/resources">查看资源余额与账单</Link></Button></Card> : null}
  </> : null}
  {dialog === "create" || editing ? <CollectionDialog title={editing ? "编辑密钥限制" : "创建 API 密钥"} onClose={() => { setDialog(null); setEditing(null); }}><KeyForm initial={editing?.limits} disabled={disabled} onSubmit={async (input) => {
                const path = editing ? `keys/${editing.id}/changes` : "keys";
                const value = await command.run(path, editing ? { expectedRevision: editing.revision, patch: { state: editing.state, limits: input } } : input);
                if (value !== undefined)
                    received(value, path);
            }}/></CollectionDialog> : null}
  {confirmation ? <CollectionDialog title="确认密钥操作" onClose={() => setConfirmation(null)}><p className="text-sm">{confirmation.state === "REVOKED" ? "撤销后该密钥永久失效，无法恢复。" : "此操作会改变密钥可用状态。"} 密钥：{confirmation.key.limits.name}</p><Button disabled={disabled} variant={confirmation.state === "REVOKED" ? "destructive" : "default"} onClick={async () => {
                const path = `keys/${confirmation.key.id}/changes`;
                const value = await command.run(path, { expectedRevision: confirmation.key.revision, patch: { state: confirmation.state } });
                if (value !== undefined)
                    received(value, path);
            }}>确认{stateNames[confirmation.state]}</Button></CollectionDialog> : null}
  {secret ? <CollectionDialog title="API 密钥" onClose={() => setSecret(null)}>{secret.value ? <><p className="text-sm text-amber-700">完整凭据仅在本次创建时显示。请妥善保存，关闭后无法再次读取。</p><pre className="overflow-auto rounded-lg bg-muted p-4 text-sm">{secret.value}</pre><Button onClick={async () => {
                    try {
                        await navigator.clipboard.writeText(secret.value!);
                    }
                    catch {
                        setError("DATA_UNAVAILABLE");
                    }
                }}>复制完整凭据</Button></> : <><p className="text-sm">已核实原密钥记录。完整凭据未能交付，系统不会重放明文；可撤销这个密钥后重新创建。</p><p className="font-mono text-xs">{secret.key.id} · ••••{secret.key.suffix}</p><Button variant="outline" onClick={() => { setConfirmation({ key: secret.key, state: "REVOKED" }); setSecret(null); }}>撤销此密钥</Button></>}</CollectionDialog> : null}
  {dialog === "docs" ? <CollectionDialog title="Amazon 数据 API 使用说明" onClose={() => setDialog(null)}><p className="text-sm">在服务的 HTTPS 域名下使用 API。将完整凭据放在 Authorization 头，任务与结果归属创建者的“我的数据”。每次新任务使用一个 UUID 作为 Idempotency-Key；重试保留同一编号和完整请求条件。</p><pre className="overflow-auto rounded-lg bg-muted p-4 text-xs">{`POST /data-api/v1/amazon/jobs\nAuthorization: DataKey <密钥ID>.<完整secret>\nIdempotency-Key: <本次请求UUID>\nContent-Type: application/json\n\n{"query":{"site":"us","mode":"keyword","keyword":"desk lamp","limit":10,"fields":["asin","title","price","currency"]},"maximumRows":10,"maximumCostFen":50}\n\nGET /data-api/v1/amazon/jobs/<任务ID>\nGET /data-api/v1/amazon/jobs/<任务ID>/results?limit=100\nGET /data-api/v1/amazon/jobs/by-command/<原请求UUID>`}</pre><p className="text-sm">站点仅接受已开放列表。类目模式使用 categoryNode（数字节点 ID），ASIN 模式使用 asins 数组；关键词可附类目过滤。最多 200 条/次，5 分/成功保存一条。结果返回 nextCursor 时，用 cursor 继续读取下一页。</p><p className="text-sm">DATA_UNKNOWN 表示结果待核实，先按原请求 UUID 查询，再保留原编号重试。403 表示原身份、权限、密钥状态或 IP 不满足要求。原始缺失字段会在 missing 中列出。</p></CollectionDialog> : null}
  {job ? <JobDialog scope={scope} job={job} onClose={() => setJob(null)}/> : null}
 </Frame>;
}
function BudgetPreview({ overview }: {
    overview: Overview;
}) {
    const [selected, setSelected] = useState(overview.keys[0]?.id ?? "");
    const key = overview.keys.find(k => k.id === selected) ?? overview.keys[0];
    const quota = key && overview.keyQuotas.find(q => q.keyId === key.id);
    if (!key || !quota)
        return <p className={`${styles.usage} text-muted-foreground`}>创建密钥后可查看该密钥的额度与预留量。</p>;
    const day = quota.dayConsumedRows + quota.dayReservedRows;
    const month = quota.monthConsumedFen + quota.monthReservedFen;
    return <div className={styles.usage}><Select aria-label="额度所属密钥" value={key.id} onChange={e => setSelected(e.target.value)}>{overview.keys.map(k => <option key={k.id} value={k.id}>{k.limits.name}</option>)}</Select><div className={styles.meter}><p><span>每日条数占用（UTC）</span><span>{day} / {key.limits.dailyRows}</span></p><progress aria-label="该密钥当日条数占用" max={key.limits.dailyRows} value={day}/></div><div className={styles.meter}><p><span>月预算占用（UTC）</span><span>{money(month)} / {money(key.limits.monthlyCostFen)}</span></p><progress aria-label="该密钥月预算占用" max={key.limits.monthlyCostFen} value={month}/></div></div>;
}
function KeyForm({ initial, disabled, onSubmit }: {
    initial?: DataKey["limits"];
    disabled: boolean;
    onSubmit: (input: DataKey["limits"]) => void;
}) {
    const originalExpiryDate = initial ? new Date(initial.expiresAt).toISOString().slice(0, 10) : "";
    const [name, setName] = useState(initial?.name ?? ""), [expiry, setExpiry] = useState(originalExpiryDate), [rows, setRows] = useState(initial ? String(initial.dailyRows) : ""), [budget, setBudget] = useState(initial ? String(initial.monthlyCostFen / 100) : ""), [permissions, setPermissions] = useState<string[]>(initial?.permissions ?? ["amazon.acquire", "amazon.result.read"]), [cidrs, setCIDRs] = useState(initial?.cidrs?.join("\n") ?? ""), [error, setError] = useState("");
    const nameTooLong = dataTextBytes(name) > KEY_NAME_MAX_BYTES;
    return <form className="space-y-4" onSubmit={e => {
            e.preventDefault();
            if (nameTooLong || !/^\d+(\.\d{1,2})?$/.test(budget)) {
                setError("INVALID_DATA_REQUEST");
                return;
            }
            ;
            const expiresAt = initial && expiry === originalExpiryDate ? initial.expiresAt : `${expiry}T00:00:00Z`;
            const parsed = keyLimitsSchema.safeParse({ name, expiresAt, dailyRows: Number(rows), monthlyCostFen: Math.round(Number(budget) * 100), permissions, cidrs: cidrs.split(/\r?\n/).map(s => s.trim()).filter(Boolean) });
            if (!parsed.success) {
                setError("INVALID_DATA_REQUEST");
                return;
            }
            ;
            onSubmit(parsed.data);
        }}>
  <Field title="密钥名称"><Input required aria-label="密钥名称" aria-describedby="key-name-limit" aria-invalid={nameTooLong} maxLength={KEY_NAME_MAX_BYTES} value={name} onChange={e => setName(e.target.value)}/><ByteLimitHint id="key-name-limit" value={name} maximum={KEY_NAME_MAX_BYTES}/></Field><Field title="到期日（UTC，最多 365 天）"><Input required type="date" value={expiry} onChange={e => setExpiry(e.target.value)}/></Field>{initial ? <p className="text-xs text-muted-foreground">原到期时间（UTC）：{new Date(initial.expiresAt).toISOString()}。日期未改动时保留原时刻。</p> : null}<div className="grid gap-4 sm:grid-cols-2"><Field title="每日最大成功条数"><Input required type="number" min={1} max={1e9} step={1} value={rows} onChange={e => setRows(e.target.value)}/></Field><Field title="月费用上限（元）"><Input required type="number" min="0.01" max={1e9} step="0.01" value={budget} onChange={e => setBudget(e.target.value)}/></Field></div><fieldset className="space-y-2"><legend className="mb-2 text-sm font-medium">密钥能力</legend>{[["amazon.acquire", "创建 Amazon 抓取任务"], ["amazon.result.read", "读取此密钥创建的任务与结果"]].map(([value, label]) => <label className="flex gap-2 text-sm" key={value}><input type="checkbox" checked={permissions.includes(value)} onChange={e => setPermissions(p => e.target.checked ? [...p, value] : p.filter(x => x !== value))}/>{label}</label>)}</fieldset><Field title="IP 白名单（可选，每行一个 CIDR，最多 20 条）"><Textarea rows={3} value={cidrs} onChange={e => setCIDRs(e.target.value)} placeholder="203.0.113.0/24"/></Field><p className="text-xs text-muted-foreground">额度包含正在执行任务的预留量，不能降低到现有占用以下。密钥的能力始终受创建者当前企业权限约束。</p><DataNotice error={error}/><Button disabled={disabled || nameTooLong} type="submit">{disabled ? "正在保存…" : initial ? "保存限制" : "创建密钥"}</Button>
 </form>;
}
