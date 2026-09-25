"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getCommercialOverview, type CommercialOverview } from "@/lib/api/commercial";
import { getCommercialWallet } from "@/lib/api/commercial-billing";
import { createSubscriptionOrder, createSubscriptionQuote, getSubscriptionOrder, type SubscriptionOffer, type SubscriptionOffers, type SubscriptionOrder, type SubscriptionQuote } from "@/lib/api/subscription-purchase";
import { ConsoleState } from "../console/console-page";
import styles from "./commercial.module.css";

const pendingSchema = z.object({ offerId: z.string().min(1).max(128), quoteId: z.string().min(1).max(128), key: z.string().uuid(), orderId: z.string().min(1).max(128).optional() }).strict();
type PendingPurchase = z.infer<typeof pendingSchema>;

function storageKey(userId: string, organizationId: string) { return `subscription-purchase.pending:${JSON.stringify([userId, organizationId])}`; }
function subscribePending(callback: () => void) {
  window.addEventListener("subscription-pending", callback);
  window.addEventListener("storage", callback);
  return () => { window.removeEventListener("subscription-pending", callback); window.removeEventListener("storage", callback); };
}
function readPending(raw: string): { value: PendingPurchase | null; invalid: boolean } {
  try {
    if (!raw) return { value: null, invalid: false };
    const parsed = pendingSchema.safeParse(JSON.parse(raw));
    return parsed.success ? { value: parsed.data, invalid: false } : { value: null, invalid: true };
  } catch { return { value: null, invalid: true }; }
}
function money(minor: string) {
  const value = BigInt(minor);
  return `¥${(value / BigInt(100)).toLocaleString("zh-CN")}.${(value % BigInt(100)).toString().padStart(2, "0")}`;
}
function time(value: string) { return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value)); }
function errorCode(error: unknown) { return error && typeof error === "object" && "code" in error && typeof error.code === "string" ? error.code : "DEPENDENCY_UNAVAILABLE"; }
const failureLabels: Record<string, string> = {
  INVALID_REQUEST: "请求无效，请重新读取套餐。", AUTHENTICATION_REQUIRED: "登录已失效。", FORBIDDEN: "当前企业购买权限已撤销。", PERMISSION_DENIED: "无购买权限。", ORGANIZATION_ACCESS_REVOKED: "当前企业访问已撤销。", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化，请重新选择企业。", IDENTITY_CONTEXT_CHANGED: "登录身份已变化。", QUOTE_EXPIRED: "报价已过期，请重新获取报价。", OFFER_UNAVAILABLE: "套餐暂不可开通。", PAYMENT_METHOD_UNAVAILABLE: "支付方式暂未开放。", INSUFFICIENT_FUNDS: "企业钱包余额不足。", ACTIVE_SUBSCRIPTION_EXISTS: "已有生效套餐，暂不支持切换或续费。", PLAN_CHANGED: "套餐内容已变化，请重新获取报价。", NOT_FOUND: "原报价或订单不存在，请重新读取套餐。", IDEMPOTENCY_CONFLICT: "原订单身份冲突，请联系支持核对。", RECONCILIATION_REQUIRED: "正在核对原订单，请勿重新购买。", DEADLINE_EXCEEDED: "请求超时，结果尚未确认。请恢复原订单，勿重新购买。", DEPENDENCY_UNAVAILABLE: "服务暂不可用，结果尚未确认。请恢复原订单。",
};
const terminalOrderErrors = new Set(["QUOTE_EXPIRED", "OFFER_UNAVAILABLE", "PAYMENT_METHOD_UNAVAILABLE", "INSUFFICIENT_FUNDS", "ACTIVE_SUBSCRIPTION_EXISTS", "PLAN_CHANGED", "NOT_FOUND"]);

export function SubscriptionPlanOptions({ userId, organizationId, organizationName, roles, overview, offers }: { userId: string; organizationId: string; organizationName: string; roles: string[]; overview: CommercialOverview; offers: SubscriptionOffers }) {
  const client = useQueryClient();
  const key = storageKey(userId, organizationId);
  const rawPending = useSyncExternalStore(subscribePending, () => { try { return sessionStorage.getItem(key) ?? ""; } catch { return "__unavailable__"; } }, () => "__loading__");
  const storageReady = rawPending !== "__loading__";
  const stored = readPending(rawPending);
  const pending = stored.value;
  const storageInvalid = stored.invalid || rawPending === "__unavailable__";
  const [selected, setSelected] = useState<SubscriptionOffer | null>(null);
  const [quote, setQuote] = useState<SubscriptionQuote | null>(null);
  const [order, setOrder] = useState<SubscriptionOrder | null>(null);
  const [readback, setReadback] = useState<CommercialOverview | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const active = useRef<AbortController | null>(null);
  const busyRef = useRef(false);
  useEffect(() => () => active.current?.abort(), []);
  const walletNeeded = offers.items.some(item => item.settlement_mode === "WALLET" && item.availability === "available");
  const wallet = useQuery({ queryKey: ["workbench", organizationId, "commercial-wallet", userId, "subscription-purchase"], queryFn: ({ signal }) => getCommercialWallet(userId, organizationId, signal), enabled: walletNeeded, gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: false });
  const canPurchase = roles.some(role => role === "listingkit_admin" || role === "platform_admin");

  function persist(value: PendingPurchase | null): boolean {
    try { if (value) sessionStorage.setItem(key, JSON.stringify(value)); else sessionStorage.removeItem(key); window.dispatchEvent(new Event("subscription-pending")); return true; }
    catch { setMessage("无法保存原订单恢复信息，已停止提交购买请求。"); return false; }
  }
  async function run<T>(action: (signal: AbortSignal) => Promise<T>, onError?: (error: unknown) => void): Promise<T | null> {
    if (busyRef.current) return null;
    busyRef.current = true; setBusy(true); setMessage("");
    const controller = new AbortController(); active.current = controller;
    try { return await action(controller.signal); }
    catch (error) { onError?.(error); setMessage(failureLabels[errorCode(error)] ?? "本次请求未确认，请核对原订单。"); return null; }
    finally { busyRef.current = false; setBusy(false); if (active.current === controller) active.current = null; }
  }
  async function choose(offer: SubscriptionOffer) {
    if (!storageReady || storageInvalid || pending || !canPurchase || offer.availability !== "available") return;
    await run(async signal => {
      const result = await createSubscriptionQuote(userId, organizationId, offer.offer_id, signal);
      if (result.plan_code !== offer.plan_code || result.term_months !== offer.term_months || result.settlement_mode !== offer.settlement_mode || result.currency !== offer.currency) {
        setMessage("报价与所选套餐不一致，请刷新后重试。"); return;
      }
      setSelected(offer); setQuote(result);
    });
  }
  async function readEntitlements(purchased: SubscriptionOrder, signal: AbortSignal) {
    const result = await getCommercialOverview(organizationId, signal);
    if (result.organization_id !== organizationId || result.subscription?.plan_code !== purchased.plan_code || !["active", "trialing"].includes(result.subscription.effective_status)) {
      setMessage("订单已完成，但权益回读尚未确认。请查询原订单并重试权益回读。"); return;
    }
    setReadback(result);
    setMessage("权益已生效，以下是订阅 owner 的最新回读。");
    persist(null);
    void client.invalidateQueries({ queryKey: ["workbench", organizationId, "account-resources"], refetchType: "none" });
    void client.invalidateQueries({ queryKey: ["workbench", organizationId, "commercial", "entitlements"], refetchType: "none" });
  }
  async function handleOrder(result: SubscriptionOrder, intent: PendingPurchase, signal: AbortSignal) {
    const retained = { ...intent, orderId: result.order_id };
    persist(retained);
    setOrder(result); setQuote(null); setSelected(null);
    if (result.status === "FULFILLED") {
      if (result.activation_proof?.outcome !== "ACTIVATED") { setMessage("订单状态与激活凭据不一致，正在核对原订单。"); return; }
      try { await readEntitlements(result, signal); }
      catch { setMessage("订单已完成，但权益回读尚未确认。请查询原订单并重试权益回读。"); }
    } else if (result.status === "CANCELLED") {
      persist(null);
      setMessage(failureLabels[result.failure_code ?? ""] ?? "原订单已取消，未开通套餐。");
    } else if (result.status === "RECONCILIATION_REQUIRED") setMessage("正在核对原订单，请勿重新购买。");
    else setMessage("原订单正在处理，尚未开通权益。请稍后查询订单状态。");
  }
  async function submit() {
    if (!storageReady || storageInvalid || !quote || !selected || pending || !canPurchase) return;
    if (quote.settlement_mode === "WALLET" && (!wallet.data || wallet.data.organization_id !== organizationId || BigInt(wallet.data.available_minor) < BigInt(quote.total_minor))) { setMessage("当前钱包余额不足以支付报价，请取消并刷新套餐。"); return; }
    const intent = { offerId: selected.offer_id, quoteId: quote.quote_id, key: crypto.randomUUID() };
    if (!persist(intent)) return;
    setQuote(null); setSelected(null);
    await run(async signal => {
      const result = await createSubscriptionOrder(userId, organizationId, intent.quoteId, intent.key, signal);
      await handleOrder(result, intent, signal);
    }, error => { if (terminalOrderErrors.has(errorCode(error))) persist(null); });
  }
  async function resume() {
    if (!pending) return;
    const intent = pending;
    await run(async signal => {
      const result = intent.orderId ? await getSubscriptionOrder(userId, organizationId, intent.orderId, signal) : await createSubscriptionOrder(userId, organizationId, intent.quoteId, intent.key, signal);
      await handleOrder(result, intent, signal);
    }, error => { if (!intent.orderId && terminalOrderErrors.has(errorCode(error))) persist(null); });
  }

  function availability(offer: SubscriptionOffer) {
    if (!storageReady || storageInvalid) return storageInvalid ? "原订单恢复信息无效，请联系支持核对；已暂停新购买。" : "正在检查原订单";
    const current = readback?.subscription ?? overview.subscription;
    if (current && ["active", "trialing"].includes(current.effective_status)) return current.plan_code === offer.plan_code ? "当前套餐" : "已有生效套餐，暂不支持切换";
    if (offer.availability === "current_plan") return "当前套餐";
    if (offer.availability === "active_subscription_conflict") return "已有生效套餐，暂不支持切换";
    if (offer.availability === "payment_unavailable" || offer.settlement_mode === "EXTERNAL_PAYMENT") return "支付方式暂未开放";
    if (offer.availability === "offer_unavailable") return "暂不可开通";
    if (!canPurchase) return "无购买权限";
    if (offer.settlement_mode === "WALLET") {
      if (!wallet.data || wallet.data.organization_id !== organizationId) return wallet.isError ? "余额暂不可确认" : "正在确认钱包余额";
      if (BigInt(wallet.data.available_minor) < BigInt(offer.total_minor)) return "余额不足";
    }
    return "";
  }

  return <div className={styles.stack}>
    <p className={styles.observation}>当前有效企业：{organizationName || "未提供名称"}（{organizationId}） · 正式可售套餐、价格和开通能力仅来自服务端目录。</p>
    {offers.items.length === 0 ? <ConsoleState kind="empty" title="暂无正式可售套餐">当前企业没有服务端配置的可售套餐，暂不可开通。</ConsoleState> : <div className={styles.subscriptionOffers}>{offers.items.map(offer => {
      const blocked = availability(offer);
      return <Card key={offer.offer_id} role="region" aria-label={offer.plan_name || offer.plan_code} className={styles.panel}>
        <p className={styles.eyebrow}>{offer.plan_code} · {offer.term_months} 个月</p>
        <h2>{offer.plan_name || "套餐暂不可用"}</h2>
        <p className={styles.subscriptionPrice}>{money(offer.total_minor)} <span>{offer.currency} · {offer.settlement_mode === "ZERO_PRICE" ? "正式 0 元" : offer.settlement_mode === "WALLET" ? "企业钱包" : "外部支付"}</span></p>
        {pending ? <p className={styles.subtle}>已有待确认的原订单，请先恢复或查询。</p> : blocked ? <p className={styles.subtle}>{blocked}</p> : <Button disabled={busy} onClick={() => void choose(offer)}>立即开通</Button>}
      </Card>;
    })}</div>}
    {quote && selected && !pending ? <Card role="region" aria-label="报价确认" className={styles.panel}>
      <h2>报价确认</h2><dl className={styles.orderFacts}>
        <div><dt>当前企业</dt><dd>{organizationName || "未提供名称"}（{organizationId}）</dd></div>
        <div><dt>套餐</dt><dd>{selected.plan_name}（{quote.plan_code}）</dd></div>
        <div><dt>周期</dt><dd>{quote.term_months} 个月</dd></div>
        <div><dt>金额</dt><dd>{money(quote.total_minor)}</dd></div>
        <div><dt>结算方式</dt><dd>{quote.settlement_mode === "ZERO_PRICE" ? "正式 0 元" : quote.settlement_mode === "WALLET" ? "企业钱包" : "外部支付暂不可用"}</dd></div>
        <div><dt>报价有效期</dt><dd>{time(quote.expires_at)}</dd></div>
        <div><dt>生效语义</dt><dd>仅首次开通；实际起止时间由订阅 owner 在激活时确定。</dd></div>
      </dl>{quote.settlement_mode === "WALLET" && wallet.data && BigInt(wallet.data.available_minor) < BigInt(quote.total_minor) ? <p role="alert">当前钱包余额不足以支付报价，请取消并刷新套餐。</p> : null}<div className={styles.actions}><Button disabled={busy || quote.settlement_mode === "EXTERNAL_PAYMENT" || (quote.settlement_mode === "WALLET" && (!wallet.data || wallet.data.organization_id !== organizationId || BigInt(wallet.data.available_minor) < BigInt(quote.total_minor)))} onClick={() => void submit()}>确认开通</Button><Button variant="outline" disabled={busy} onClick={() => { setQuote(null); setSelected(null); }}>取消</Button></div>
    </Card> : null}
    {pending ? <Card role="region" aria-label="原订单恢复" className={styles.panel}><h2>原订单待确认</h2><p>报价编号：{pending.quoteId}{pending.orderId ? ` · 订单编号：${pending.orderId}` : ""}</p><p className={styles.subtle}>刷新或切换页面后仍保留原幂等操作；不会自动新建订单。</p><Button disabled={busy || (!pending.orderId && !canPurchase)} onClick={() => void resume()}>{pending.orderId ? "查询原订单" : "恢复原订单"}</Button>{!pending.orderId && !canPurchase ? <p role="alert">购买权限已变化，请由有权限的企业管理员核对原操作。</p> : null}</Card> : null}
    {busy ? <p role="status">正在向当前企业 owner 核对购买结果…</p> : null}
    {message ? <p role={readback ? "status" : "alert"}>{message}</p> : null}
    {order ? <Card role="region" aria-label="套餐订单" className={styles.panel}><h2>套餐订单</h2><p>订单编号：{order.order_id} · 状态：{order.status}</p><div className={styles.actions}><Button asChild variant="outline"><Link href={`/workbench/plans/orders/${encodeURIComponent(order.order_id)}`} prefetch={false}>查看订单</Link></Button><Button asChild variant="outline"><Link href="/workbench/plans/entitlements" prefetch={false}>查看我的权益</Link></Button></div></Card> : null}
    {readback ? <Card role="region" aria-label="权益回读" className={styles.panel}><h2>订阅与权益回读</h2><p>当前套餐：{readback.subscription?.plan_name ?? readback.subscription?.plan_code}</p><p>已授予权益：{readback.entitlements.map(item => item.module_code).join("、") || "无已授予权益"}</p><Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>查看资源与额度</Link></Button></Card> : null}
  </div>;
}
