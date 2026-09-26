"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { CommercialOrder, CommercialOrderPage, CommercialOrderSummary, CommercialWallet, CommercialWalletEntryPage } from "@/lib/api/commercial-billing";
import { ConsoleState } from "../console/console-page";
import styles from "./commercial.module.css";

const entryLabels: Record<string, string> = { TOP_UP_CREDIT: "钱包充值", PURCHASE_RESERVE: "资源购买预留", PURCHASE_COMMIT: "资源购买", PURCHASE_RELEASE: "购买释放", REFUND_REVERSAL: "退款冲正", CHARGEBACK_REVERSAL: "拒付冲正", DEBT_REPAYMENT: "欠款偿还" };
const orderKindLabels: Record<CommercialOrder["kind"], string> = { WALLET_TOP_UP: "钱包充值", RESOURCE_PURCHASE: "资源购买", SUBSCRIPTION_PURCHASE: "套餐购买" };
const orderStatusLabels: Record<CommercialOrder["status"], string> = { PENDING: "待处理", FUNDS_RESERVED: "资金已预留", FULFILLING: "履约中", FULFILLED: "已完成", CANCELLED: "已取消", RECONCILIATION_REQUIRED: "待核对" };

function minorAmount(minor: string, currency = "CNY") {
  const value = BigInt(minor);
  const negative = value < 0;
  const absolute = negative ? -value : value;
  const major = (absolute / BigInt(100)).toLocaleString("zh-CN");
  const cents = (absolute % BigInt(100)).toString().padStart(2, "0");
  return `${negative ? "−" : ""}${currency === "CNY" ? "¥" : `${currency} `}${major}.${cents}`;
}

function timeLabel(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value));
}

function shanghaiDayBoundary(value: string, dayOffset = 0) {
  const [year, month, day] = value.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, day + dayOffset, -8)).toISOString();
}

function Panel({ title, children, className = "" }: { title: string; children: React.ReactNode; className?: string }) {
  return <Card role="region" aria-label={title} className={`${styles.panel} ${className}`}><h2>{title}</h2>{children}</Card>;
}

function Stat({ label, value, note, tone = "teal" }: { label: string; value: string; note: string; tone?: "blue" | "teal" | "purple" | "orange" }) {
  return <Card className={`${styles.billingStat} ${styles[tone]}`}><p>{label}</p><strong>{value}</strong><small>{note}</small></Card>;
}

function CapabilityNote({ children }: { children: React.ReactNode }) {
  return <div className={styles.capabilityNote} role="note"><strong>写入能力未开放</strong><span>{children}</span></div>;
}

export function WalletView({ wallet, entries, onNext }: { wallet: CommercialWallet; entries: CommercialWalletEntryPage; onNext: () => void }) {
  return <div className={styles.stack}>
    <CapabilityNote>支付服务和充值 intent 当前不可用；此处仅展示企业钱包 owner 返回的数据，不会提交充值、购买或资金变更。</CapabilityNote>
    <Panel title="企业钱包余额" className={styles.walletHero}>
      <div><p className={styles.eyebrow}>当前企业 · {wallet.currency}</p><strong className={styles.walletBalance}>{minorAmount(wallet.available_minor, wallet.currency)}</strong><p className={styles.subtle}>可用余额 · 观察于 {timeLabel(wallet.observed_at)}</p></div>
      <Button disabled aria-disabled="true">充值暂未开放</Button>
      <dl className={styles.walletFacts}><div><dt>预留金额</dt><dd>{minorAmount(wallet.reserved_minor, wallet.currency)}</dd></div><div><dt>待偿欠款</dt><dd>{minorAmount(wallet.debt_minor, wallet.currency)}</dd></div><div><dt>累计充值</dt><dd>{minorAmount(wallet.lifetime_topup_minor, wallet.currency)}</dd></div><div><dt>累计支出</dt><dd>{minorAmount(wallet.lifetime_spend_minor, wallet.currency)}</dd></div></dl>
    </Panel>
    <Panel title="钱包流水" className={styles.billingPanel}>
      {entries.items.length === 0 ? <ConsoleState kind="empty" title="暂无钱包流水">当前企业的钱包 owner 没有返回流水记录。</ConsoleState> : <div className={styles.billingTableWrap}><table className={styles.billingTable}><caption className="sr-only">企业钱包流水</caption><thead><tr><th>时间</th><th>类型</th><th>可用余额变动</th><th>预留变动</th><th>欠款变动</th><th>关联订单</th></tr></thead><tbody>{entries.items.map(row => <tr key={row.entry_id}><td>{timeLabel(row.occurred_at)}</td><td>{entryLabels[row.entry_type] ?? row.entry_type}</td><td>{minorAmount(row.available_delta_minor, row.currency)}</td><td>{minorAmount(row.reserved_delta_minor, row.currency)}</td><td>{minorAmount(row.debt_delta_minor, row.currency)}</td><td>{row.order_id ? <Link href={`/workbench/plans/orders/${encodeURIComponent(row.order_id)}`} prefetch={false}>{row.order_id}</Link> : "—"}</td></tr>)}</tbody></table></div>}
      <div className={styles.pagination}><span>当前页 {entries.items.length} 条；流水金额按最小货币单位精确格式化。</span>{entries.next_cursor ? <Button variant="outline" onClick={onNext}>下一页流水</Button> : null}</div>
    </Panel>
    <Panel title="使用充值余额"><p className={styles.subtle}>真实报价和扣款需要经企业确认；当前 UI 不发起会改变钱包或资源的请求。</p><div className={styles.resourcePurchaseCards}>{[["店铺服务", "续费周期由账单与资源 owner 决定"], ["token 积分", "资源数量和报价由服务端合同提供"], ["数据服务", "资源数量和报价由服务端合同提供"]].map(([label, detail]) => <Card key={label} className={styles.resourcePurchaseCard}><h3>{label}</h3><p>{detail}</p><Button disabled variant="outline">购买暂未开放</Button></Card>)}</div></Panel>
    <Panel title="充值与支付"><p className={styles.subtle}>当前没有可用的支付服务端入口。</p><p className={styles.subtle}>充值余额、充值记录和支付结果仅在真实 provider 接入后开放。</p></Panel>
  </div>;
}

export function OrdersView({ summary, page, onFilter, onNext }: { summary: CommercialOrderSummary; page: CommercialOrderPage; onFilter: (filters: { query: string; kind: string; status: string; from: string; until: string }) => void; onNext: () => void }) {
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("");
  const [status, setStatus] = useState("");
  const [from, setFrom] = useState("");
  const [until, setUntil] = useState("");
  const submit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const fromISO = from ? shanghaiDayBoundary(from) : "";
    const untilISO = until ? shanghaiDayBoundary(until, 1) : "";
    onFilter({ query: query.trim(), kind, status, from: fromISO, until: untilISO });
  };
  return <div className={styles.stack}>
    <div className={styles.billingStats}><Stat label="近 30 天支出" value={minorAmount(summary.spend_minor, summary.currency)} note={`${timeLabel(summary.from)} – ${timeLabel(summary.until)}`} /><Stat tone="blue" label="店铺服务" value={minorAmount(summary.store_renewal_spend_minor, summary.currency)} note="当前账单 owner 汇总" /><Stat label="AI 点数" value={minorAmount(summary.ai_point_spend_minor, summary.currency)} note="当前账单 owner 汇总" /><Stat tone="purple" label="数据资源" value={minorAmount(summary.data_row_spend_minor, summary.currency)} note="当前账单 owner 汇总" /></div>
    <CapabilityNote>订单只读。新建订单会产生真实资金或资源副作用，本页面不提供购买、退款、发票开具或导出写操作。</CapabilityNote>
    <Panel title="账单与订单" className={styles.billingPanel}>
      <form className={styles.billingFilters} onSubmit={submit}>
        <label>搜索订单<input aria-label="搜索订单" maxLength={128} value={query} onChange={event => setQuery(event.target.value)} placeholder="订单号或内容" /></label>
        <label>开始日期<input aria-label="开始日期" type="date" value={from} onChange={event => setFrom(event.target.value)} /></label>
        <label>结束日期<input aria-label="结束日期" type="date" value={until} onChange={event => setUntil(event.target.value)} /></label>
        <label>订单类型<select aria-label="订单类型" value={kind} onChange={event => setKind(event.target.value)}><option value="">全部类型</option><option value="SUBSCRIPTION_PURCHASE">套餐购买</option><option value="RESOURCE_PURCHASE">资源购买</option><option value="WALLET_TOP_UP">钱包充值</option></select></label>
        <label>订单状态<select aria-label="订单状态" value={status} onChange={event => setStatus(event.target.value)}><option value="">全部状态</option>{Object.entries(orderStatusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        <Button type="submit">筛选</Button><Button type="button" variant="outline" onClick={() => { setQuery(""); setKind(""); setStatus(""); setFrom(""); setUntil(""); onFilter({ query: "", kind: "", status: "", from: "", until: "" }); }}>重置</Button>
      </form>
      <div className={styles.actions}><Button disabled variant="outline">导出暂未开放</Button><Button disabled>发票管理暂未开放</Button></div>
      {page.items.length === 0 ? <ConsoleState kind="empty" title="没有匹配的订单">调整搜索或筛选条件后重试。</ConsoleState> : <div className={styles.billingTableWrap}><table className={styles.billingTable}><caption className="sr-only">企业账单与订单</caption><thead><tr><th>时间</th><th>类型</th><th>内容</th><th>金额</th><th>状态</th><th>详情</th></tr></thead><tbody>{page.items.map(order => <tr key={order.order_id}><td>{timeLabel(order.created_at)}</td><td>{orderKindLabels[order.kind]}</td><td>{order.description}</td><td>{minorAmount(order.total_minor, order.currency)}</td><td>{orderStatusLabels[order.status]}</td><td><Button asChild variant="outline" size="sm"><Link href={`/workbench/plans/orders/${encodeURIComponent(order.order_id)}`} prefetch={false}>详情</Link></Button></td></tr>)}</tbody></table></div>}
      <div className={styles.pagination}><span>显示 {page.items.length} 条 · 每页最多 50 条</span>{page.next_cursor ? <Button variant="outline" onClick={onNext}>下一页</Button> : null}</div>
      <p className={styles.subtle}>汇总与订单由账单 owner 返回；当前页不推算未返回的历史记录。</p>
    </Panel>
  </div>;
}

export function OrderDetailView({ order }: { order: CommercialOrder }) {
  return <div className={styles.stack}>
    <Panel title="订单信息"><dl className={styles.orderFacts}><div><dt>订单编号</dt><dd>{order.order_id}</dd></div><div><dt>类型</dt><dd>{orderKindLabels[order.kind]}</dd></div><div><dt>状态</dt><dd>{orderStatusLabels[order.status]}</dd></div><div><dt>金额</dt><dd>{minorAmount(order.total_minor, order.currency)}</dd></div><div><dt>创建时间</dt><dd>{timeLabel(order.created_at)}</dd></div><div><dt>更新时间</dt><dd>{timeLabel(order.updated_at)}</dd></div><div><dt>说明</dt><dd>{order.description}</dd></div>{order.failure_code ? <div><dt>失败原因</dt><dd>{order.failure_code}</dd></div> : null}</dl></Panel>
    {order.kind === "SUBSCRIPTION_PURCHASE" ? <Panel title="套餐购买详情"><dl className={styles.orderFacts}>
      <div><dt>套餐代码</dt><dd>{order.plan_code}</dd></div><div><dt>开通周期</dt><dd>{order.term_months} 个月</dd></div>
      <div><dt>结算方式</dt><dd>{order.settlement_mode === "ZERO_PRICE" ? "正式 0 元" : "企业钱包"}</dd></div>
      <div><dt>激活结果</dt><dd>{order.activation_proof?.outcome === "ACTIVATED" ? "订阅 owner 已激活" : order.activation_proof?.outcome === "REJECTED" ? `订阅 owner 已拒绝（${order.activation_proof.failure_code}）` : order.status === "RECONCILIATION_REQUIRED" ? "正在核对，不代表权益已生效" : "尚无激活凭据，不代表权益已生效"}</dd></div>
      {order.activation_proof?.outcome === "ACTIVATED" ? <><div><dt>生效时间</dt><dd>{timeLabel(order.activation_proof.starts_at)}</dd></div><div><dt>到期时间</dt><dd>{timeLabel(order.activation_proof.expires_at)}</dd></div></> : null}
    </dl><Button asChild variant="outline"><Link href="/workbench/plans/entitlements" prefetch={false}>回读我的权益</Link></Button></Panel> : <Panel title="订单项目">{order.items.length === 0 ? <p className={styles.subtle}>该订单没有返回项目明细。</p> : <div className={styles.billingTableWrap}><table className={styles.billingTable}><caption className="sr-only">订单项目明细</caption><thead><tr><th>资源类型</th><th>数量</th><th>金额</th></tr></thead><tbody>{order.items.map(item => <tr key={item.order_item_id}><td>{item.product_kind}</td><td>{item.resource_quantity}</td><td>{minorAmount(item.amount_minor, order.currency)}</td></tr>)}</tbody></table></div>}</Panel>}
    <Button asChild variant="outline"><Link href="/workbench/plans/orders" prefetch={false}>返回账单与订单</Link></Button>
  </div>;
}
