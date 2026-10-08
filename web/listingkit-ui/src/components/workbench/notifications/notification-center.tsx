"use client";

import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useRef, useState } from "react";
import { ArrowLeft, Bell, Check, ChevronRight, Diamond, Info, RefreshCw, CircleDot, X } from "lucide-react";
import { z } from "zod";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { NotificationError, notificationCommandSchema, notificationItemSchema, notificationListSchema, notificationRequest, notificationSnapshotSchema, type NotificationItem, type NotificationList, type NotificationScope } from "@/lib/api/notifications";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import styles from "./notifications.module.css";

const policy = { staleTime: 0, gcTime: 0, retry: false, refetchOnWindowFocus: false, refetchOnReconnect: false } as const;
const labels: Record<string, string> = { official: "官方消息", "workbench-plan": "对话计划", "workbench-task": "AI 任务", "product-review": "标题审核", acquisition: "商品获取", store: "店铺状态", "org-resource": "企业资源", "member-resource": "个人分配资源", "member-limit": "月度用量", billing: "订单与账单", "invitation-admin": "企业邀请", "invitation-recipient": "收到的邀请", membership: "成员操作", knowledge: "知识文件", "organization-verification": "企业认证", "personal-verification": "个人认证", "referral-earnings": "推广收益", "referral-withdrawal": "提现", inventory: "库存", advertising: "广告", opportunity: "商机", report: "报告", "merchant-chat": "商家聊天", fulfillment: "履约" };
const officialTypes: Record<string, string> = { PRODUCT: "产品更新", SYSTEM: "系统通知", ACTIVITY: "活动公告", POLICY: "规则说明" };
function notificationTime(value: string | null) { return value ? new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }) : "发生时间未提供"; }
function errorMessage(error: unknown) {
  const code = error instanceof NotificationError ? error.code : "NOTIFICATION_UNAVAILABLE";
  if (code === "OUTCOME_UNKNOWN") return "阅读状态待核实，请核实原操作。";
  if (["IDENTITY_CONTEXT_CHANGED", "ORGANIZATION_CONTEXT_CHANGED", "AUTHENTICATION_REQUIRED", "FORBIDDEN"].includes(code)) return "登录或访问权限已变化，请刷新当前上下文。";
  if (["STALE_SNAPSHOT", "NOT_FOUND"].includes(code)) return "通知状态已变化，请刷新后查看。";
  return "通知暂时无法读取，请稍后重试。";
}
function useScope(userId: string, tab: "official" | "business"): { scope: NotificationScope; available: boolean; companyUnavailable: boolean } {
  const context = useWorkbenchContext();
  const confirmed = !context.isLoading && !context.isSwitching && !context.error && !context.blockingError && !context.selectionRequired && context.user?.id === userId;
  const organizationId = confirmed ? context.effectiveOrganization?.id : undefined;
  const authenticationError = context.error?.code === "AUTHENTICATION_REQUIRED" || context.blockingError?.code === "AUTHENTICATION_REQUIRED";
  return { scope: { userId, channel: tab === "official" ? "official" : organizationId ? "business" : "personal", ...(organizationId && tab === "business" ? { organizationId } : {}) }, available: Boolean(userId) && !authenticationError && !context.isSwitching && (!context.user || context.user.id === userId), companyUnavailable: tab === "business" && !organizationId };
}
function Coverage({ data }: { data?: NotificationList }) {
  if (!data) return null;
  const failed = data.coverage.filter(c => !c.complete);
  const denied = data.coverage.filter(c => c.state === "DENIED");
  const unopened = data.coverage.filter(c => c.state === "DEPENDENCY_MISSING");
  return <>
    {failed.length ? <p role="status" className={styles.notice}>部分来源暂时不可用：{failed.map(c => labels[c.source] ?? c.source).join("、")}。当前数字仅包含已读取的消息。</p> : null}
    {denied.length ? <p className={styles.coverage}>无权读取的来源已隐藏：{denied.map(c => labels[c.source] ?? c.source).join("、")}。</p> : null}
    {unopened.length ? <details className={styles.coverage}><summary>尚未开放的业务提醒</summary><p>{unopened.map(c => labels[c.source] ?? c.source).join("、")}：对应业务尚未开放，暂无消息。</p></details> : null}
  </>;
}
type Intent = { stage: "snapshot" | "read-all" | "read"; key: string; body: unknown; snapshotKey?: string; unknown: boolean };
function useReading(scope: NotificationScope, changed: () => void) {
  const [intent, setIntent] = useState<Intent | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const controller = useRef<AbortController | null>(null);
  const lock = useRef(false);
  useEffect(() => () => controller.current?.abort(), []);
  async function execute(current: Intent, verify: boolean) {
    if (lock.current) return;
    lock.current = true; setBusy(true); setError(null); controller.current = new AbortController();
    const signal = controller.current.signal;
    try {
      if (verify) {
        try {
          const receipt = await notificationRequest(scope, "/commands/" + current.key, notificationCommandSchema, { signal });
          if (receipt.operation !== current.stage) throw new NotificationError("NOTIFICATION_UNAVAILABLE");
          if (current.stage !== "snapshot") { setIntent(null); changed(); return; }
        } catch (e) { if (!(e instanceof NotificationError && e.status === 404)) throw e; }
      }
      if (current.stage === "snapshot") {
        // A lost snapshot response is replayed with its original key; it never
        // silently establishes a newer, wider collection.
        const snapshot = await notificationRequest(scope, "/snapshot", notificationSnapshotSchema, { key: current.key, body: {}, signal });
        current = { stage: "read-all", key: crypto.randomUUID(), body: { id: snapshot.id, fingerprint: snapshot.fingerprint }, snapshotKey: current.key, unknown: false };
        setIntent(current);
      }
      await notificationRequest(scope, "/" + current.stage, notificationCommandSchema, { key: current.key, body: current.body, signal });
      setIntent(null); changed();
    } catch (e) {
      const unknown = e instanceof NotificationError && (e.code === "OUTCOME_UNKNOWN" || verify && (e.status >= 500 || e.code === "NOTIFICATION_UNAVAILABLE"));
      setIntent(unknown ? { ...current, unknown: true } : null); setError(e);
    } finally { lock.current = false; setBusy(false); }
  }
  return { busy, intent, error, read: (id: string) => { const next: Intent = { stage: "read", key: crypto.randomUUID(), body: { ref: id }, unknown: false }; setIntent(next); void execute(next, false); }, all: () => { const next: Intent = { stage: "snapshot", key: crypto.randomUUID(), body: {}, unknown: false }; setIntent(next); void execute(next, false); }, verify: () => { if (intent) void execute(intent, true); } };
}
function ReadingStatus({ reading }: { reading: ReturnType<typeof useReading> }) {
  return reading.error ? <div role="alert" className={styles.notice}>{errorMessage(reading.error)}{reading.intent?.unknown ? <button type="button" disabled={reading.busy} onClick={reading.verify}>核实原操作</button> : null}</div> : null;
}
function Tabs({ tab, onChange, data }: { tab: "official" | "business"; onChange: (tab: "official" | "business") => void; data?: NotificationList }) {
  return <div className={styles.tabs} role="tablist" aria-label="消息分类">
    <button type="button" role="tab" aria-label="硕米官方通知" aria-selected={tab === "official"} onClick={() => onChange("official")}><Diamond size={15} aria-hidden="true" />硕米官方通知</button>
    <button type="button" role="tab" aria-label="商家经营触发通知" aria-selected={tab === "business"} onClick={() => onChange("business")}><Bell size={15} aria-hidden="true" />商家经营触发通知{data?.pending && tab === "business" ? <span className={styles.badge}>{data.exact ? "" : "≥"}{data.pending}</span> : null}</button>
  </div>;
}
function ItemRow({ item, compact = false, onNavigate }: { item: NotificationItem; compact?: boolean; onNavigate?: () => void }) {
  const detail = `/workbench/notifications/${item.category}/${item.id}`;
  return <Link href={detail} prefetch={false} className={`${styles.row} ${compact ? styles.compact : ""}`} onClick={onNavigate}>
    <span className={styles.marker}>{!item.read ? <span className={styles.dot} aria-label="未读" /> : null}</span>
    <span className={`${styles.icon} ${item.category === "business" || item.type === "PRODUCT" ? styles.businessIcon : ""}`} aria-hidden="true">{item.category === "business" ? <Bell size={19} /> : item.type === "PRODUCT" ? <b>AI</b> : item.type === "SYSTEM" ? <Info size={19} /> : item.type === "POLICY" ? <Check size={19} /> : <CircleDot size={19} />}</span>
    <span className={styles.rowText}><strong>{item.title}</strong><span>{item.summary}</span></span>
    <span className={styles.rowMeta}><time dateTime={item.occurredAt ?? undefined}>{notificationTime(item.occurredAt)}</time><span className={styles.type}>{item.attention ? "待处理" : officialTypes[item.type] ?? labels[item.source] ?? "业务结果"}</span></span>
    <ChevronRight className={styles.chevron} size={16} aria-hidden="true" />
  </Link>;
}
export function NotificationCenterPage({ expectedUserId }: { expectedUserId: string }) {
  const [tab, setTab] = useState<"official" | "business">("official");
  const { scope, available, companyUnavailable } = useScope(expectedUserId, tab);
  return <section className={styles.page}>
    <div className={styles.heading}><h1>通知中心</h1><p>官方消息与业务提醒集中查看，重要事项及时掌握。</p></div>
    {available ? <NotificationFeed key={JSON.stringify(scope)} scope={scope} tab={tab} setTab={setTab} companyUnavailable={companyUnavailable} /> : <p role="status" className={styles.notice}>登录或企业上下文正在变化，消息已停止加载。</p>}
  </section>;
}
function NotificationFeed({ scope, tab, setTab, companyUnavailable, compact = false, onNavigate }: { scope: NotificationScope; tab: "official" | "business"; setTab: (tab: "official" | "business") => void; companyUnavailable: boolean; compact?: boolean; onNavigate?: () => void }) {
  const client = useQueryClient();
  const [filtered, setFiltered] = useState(false);
  const [cursors, setCursors] = useState([""]);
  const filter = filtered ? tab === "official" ? "unread" : "pending" : "all";
  const query = useQuery({ ...policy, queryKey: ["notifications", scope, filter, cursors.at(-1)], queryFn: ({ signal }) => notificationRequest(scope, `?filter=${filter}&limit=${compact ? 4 : 20}${cursors.at(-1) ? "&after=" + cursors.at(-1) : ""}`, notificationListSchema, { signal }) });
  const reading = useReading(scope, () => { setCursors([""]); void client.invalidateQueries({ queryKey: ["notifications"] }); });
  const data = query.error ? undefined : query.data;
  return <>
    <div className={styles.controls}><Tabs tab={tab} onChange={setTab} data={data} />{!compact ? <label className={styles.filter}><input type="checkbox" checked={filtered} onChange={event => { setFiltered(event.target.checked); setCursors([""]); }} />{tab === "official" ? "仅看未读" : "仅看待处理"}{data ? <span className={styles.badge}>{data.exact ? "" : "≥"}{tab === "official" ? data.unread : data.pending}</span> : null}</label> : null}</div>
    {companyUnavailable ? <p className={styles.coverage} role="status">当前仅显示个人提醒。选择可访问的企业后可查看企业业务消息。</p> : null}
    <ReadingStatus reading={reading} />
    <section className={styles.panel} aria-label={tab === "official" ? "官方消息列表" : "业务提醒列表"}>
      <div className={styles.panelHead}><span>{tab === "official" ? "最新消息" : "最新提醒"}{data && !compact ? <small>共{data.exact ? "" : "至少"} {data.count} 条</small> : null}</span><div><button type="button" aria-label="刷新通知" onClick={() => void query.refetch()} disabled={query.isFetching || reading.busy}><RefreshCw size={14} aria-hidden="true" /></button><button type="button" onClick={reading.all} disabled={!data?.exact || reading.busy || Boolean(reading.intent) || data.unread === 0}><Check size={13} aria-hidden="true" />全部已读</button></div></div>
      {query.isPending ? <p role="status" className={styles.empty}>正在读取消息…</p> : query.error ? <p role="alert" className={styles.empty}>{errorMessage(query.error)}</p> : data?.items.length ? <div className={styles.rows}>{data.items.map(item => <ItemRow key={item.id} item={item} compact={compact} onNavigate={onNavigate} />)}</div> : <p className={styles.empty}>{data?.exact ? filtered ? "当前筛选下暂无消息" : tab === "official" ? "暂无官方消息" : "暂无业务提醒" : "已读取的来源暂无消息，部分来源尚未读取完成"}</p>}
    </section>
    <Coverage data={data} />
    {!compact && data ? <div className={styles.pagination}><button type="button" disabled={cursors.length === 1 || reading.busy || query.isFetching} onClick={() => setCursors(old => old.slice(0, -1))}>上一页</button><span>第 {cursors.length} 页</span><button type="button" disabled={!data.next || reading.busy || query.isFetching} onClick={() => setCursors(old => [...old, data.next])}>下一页</button></div> : null}
  </>;
}
export function NotificationDetailPage({ expectedUserId, category, notificationRef }: { expectedUserId: string; category: "official" | "business"; notificationRef: string }) {
  const { scope, available, companyUnavailable } = useScope(expectedUserId, category);
  return <section className={styles.page}><div className={styles.heading}><h1>通知详情</h1><p>查看消息内容与相关业务入口。</p></div><Link className={styles.back} href="/workbench/notifications" prefetch={false}><ArrowLeft size={15} aria-hidden="true" />返回通知中心</Link>{available ? <NotificationDetail key={JSON.stringify(scope)} scope={scope} id={notificationRef} companyUnavailable={companyUnavailable} /> : <p className={styles.notice} role="status">上下文正在变化，通知内容已隐藏。</p>}</section>;
}
function NotificationDetail({ scope, id, companyUnavailable }: { scope: NotificationScope; id: string; companyUnavailable: boolean }) {
  const client = useQueryClient();
  const query = useQuery({ ...policy, queryKey: ["notifications", scope, id], queryFn: ({ signal }) => notificationRequest(scope, "/" + id, notificationItemSchema, { signal }) });
  const reading = useReading(scope, () => { void client.invalidateQueries({ queryKey: ["notifications"] }); });
  const item = query.error ? undefined : query.data;
  // Opening the detail performs a read only. Business attention is unchanged.
  const attempted = useRef(false);
  useEffect(() => {
    if (!item || item.read || attempted.current) return;
    const timer = setTimeout(() => { attempted.current = true; reading.read(item.id); }, 0);
    return () => clearTimeout(timer);
  }, [item, reading]);
  if (!item) return <section className={styles.panel}><p role={query.error ? "alert" : "status"} className={styles.empty}>{query.error ? errorMessage(query.error) : "正在读取通知详情…"}</p><button className={styles.retry} type="button" onClick={() => void query.refetch()} disabled={query.isFetching}>重新读取</button></section>;
  return <><ReadingStatus reading={reading} /><div className={styles.detailGrid}><article className={styles.article}><div className={styles.articleCategory}><Diamond size={16} aria-hidden="true" />{officialTypes[item.type] ?? labels[item.source] ?? "业务提醒"}</div><h2>{item.title}</h2><time dateTime={item.occurredAt ?? undefined}>{notificationTime(item.occurredAt)}</time><div className={styles.body}>{(item.paragraphs.length ? item.paragraphs : [item.summary]).map((paragraph, i) => <p key={i}>{paragraph}</p>)}</div>{item.href ? <Link className={styles.primary} href={item.href} prefetch={false}>查看相关业务<ChevronRight size={15} aria-hidden="true" /></Link> : null}</article><aside className={styles.info}><h3><Info size={17} aria-hidden="true" />消息信息</h3><dl><dt>消息类型</dt><dd>{officialTypes[item.type] ?? labels[item.source] ?? "业务提醒"}</dd><dt>阅读状态</dt><dd>{item.read ? "已读" : "未读"}</dd><dt>业务状态</dt><dd>{item.attention ? "仍待处理" : "结果通知"}</dd><dt>归属范围</dt><dd>{item.category === "official" ? "官方公告" : item.organizationId ? "当前企业" : "个人"}</dd><dt>发生时间</dt><dd>{notificationTime(item.occurredAt)}</dd></dl><p>阅读消息不会完成业务操作，请在原业务页面处理。</p>{companyUnavailable && item.category === "business" ? <p>企业消息需确认企业访问权限后查看。</p> : null}</aside></div></>;
}
export function NotificationPopover() {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null), trigger = useRef<HTMLButtonElement>(null), close = useRef<HTMLButtonElement>(null);
  const id = useId();
  useEffect(() => {
    if (!open) return;
    close.current?.focus();
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false); };
    document.addEventListener("pointerdown", outside); return () => document.removeEventListener("pointerdown", outside);
  }, [open]);
  function dismiss() { setOpen(false); trigger.current?.focus(); }
  return <div ref={root} className={styles.popoverRoot} onKeyDown={event => { if (open && event.key === "Escape") { event.preventDefault(); dismiss(); } }}><button className={styles.trigger} type="button" ref={trigger} aria-label="通知" aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(value => !value)}><Bell size={16} aria-hidden="true" /><span>通知</span></button>{open ? <section className={styles.popover} id={id} aria-label="通知面板"><div className={styles.popoverHead}><strong>通知中心</strong><button ref={close} type="button" aria-label="关闭通知面板" onClick={dismiss}><X size={17} aria-hidden="true" /></button></div><PopoverFeed onNavigate={() => setOpen(false)} /><Link className={styles.viewAll} href="/workbench/notifications" prefetch={false} onClick={() => setOpen(false)}>查看全部通知<ChevronRight size={14} aria-hidden="true" /></Link></section> : null}</div>;
}
function PopoverFeed({ onNavigate }: { onNavigate: () => void }) {
  const [tab, setTab] = useState<"official" | "business">("official");
  const identity = useQuery({ ...policy, queryKey: ["notification-session-identity"], queryFn: async ({ signal }) => {
    const response = await fetch("/api/notifications/identity", { signal, cache: "no-store", redirect: "error" });
    if (!response.ok) throw new NotificationError("AUTHENTICATION_REQUIRED", response.status);
    return z.object({ userId: z.string().min(1).max(128) }).strict().parse(await readBoundedStrictJSON(response, 1024, signal));
  } });
  const { scope, available, companyUnavailable } = useScope(identity.data?.userId ?? "", tab);
  if (identity.isPending) return <p className={styles.empty} role="status">正在读取通知…</p>;
  if (identity.error || !available) return <p className={styles.empty} role="status">登录或企业上下文尚未确认。<Link href="/workbench/notifications" prefetch={false} onClick={onNavigate}>打开通知中心</Link></p>;
  return <NotificationFeed key={JSON.stringify(scope)} scope={scope} tab={tab} setTab={setTab} companyUnavailable={companyUnavailable} compact onNavigate={onNavigate} />;
}
