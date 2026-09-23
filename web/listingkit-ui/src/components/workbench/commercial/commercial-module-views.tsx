import Link from "next/link";
import type { ReactNode } from "react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import type { CommercialOverview, CommercialUsage } from "@/lib/api/commercial";
import styles from "./commercial.module.css";

const usageLabels: Record<CommercialUsage["metric"], string> = {
  listingkit_generations_succeeded: "资料生成作业",
  product_image_jobs_succeeded: "商品图作业",
  shein_drafts_succeeded: "SHEIN 草稿",
  shein_publishes_succeeded: "SHEIN 发布",
  storage_bytes_current: "当前保留存储",
};

const usageTypes: Record<CommercialUsage["metric"], string> = {
  listingkit_generations_succeeded: "商品资料",
  product_image_jobs_succeeded: "商品图片",
  shein_drafts_succeeded: "SHEIN 草稿",
  shein_publishes_succeeded: "SHEIN 发布",
  storage_bytes_current: "数据服务",
};

const unitLabels: Record<CommercialUsage["unit"], string> = {
  operation: "作业次",
  byte: "字节",
};

function formatInteger(value: string | null, missing = "未知") {
  return value === null ? missing : BigInt(value).toLocaleString("zh-CN");
}

function formatUsage(row: CommercialUsage) {
  return row.state === "known" ? `${formatInteger(row.committed)} ${unitLabels[row.unit]}` : `未知（${unitLabels[row.unit]}）`;
}

function usageWindow(row: CommercialUsage) {
  if (row.unit === "byte") return "当前保留观测，无统计起止";
  return row.window_start && row.window_end
    ? `${row.window_start.slice(0, 10)} 至 ${row.window_end.slice(0, 10)}（期末不含）`
    : "统计周期未提供";
}

function Panel({ title, children, className = "" }: { title: string; children: ReactNode; className?: string }) {
  return <Card role="region" aria-label={title} className={`${styles.panel} ${className}`}><h2>{title}</h2>{children}</Card>;
}

function PageLink({ href, children }: { href: string; children: ReactNode }) {
  return <Button asChild variant="outline" size="sm"><Link href={href} prefetch={false}>{children}</Link></Button>;
}

function SummaryMetric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone: "blue" | "teal" | "purple" }) {
  return <div className={`${styles.summaryMetric} ${styles[tone]}`}><p>{label}</p><strong>{value}</strong><small>{detail}</small></div>;
}

function ModuleEntries() {
  const entries = [
    ["套餐方案", "已批准方案说明与计费规则", "/workbench/plans/options", "blue"],
    ["我的权益", "当前企业已授予的权益与限制", "/workbench/plans/entitlements", "teal"],
    ["用量明细", "订阅用量账本中的已记录用量", "/workbench/plans/usage", "purple"],
    ["充值中心", "钱包与充值能力", "/workbench/plans/top-up", "orange"],
    ["账单与订单", "消费、退款与发票记录", "/workbench/plans/orders", "green"],
  ] as const;
  return <div className={styles.moduleEntries}>{entries.map(([title, description, href, tone]) => <Card key={href} className={`${styles.moduleEntry} ${styles[tone]}`}>
    <div><h3>{title}</h3><p>{description}</p></div><PageLink href={href}>进入</PageLink>
  </Card>)}</div>;
}

function UsageSummary({ data }: { data: CommercialOverview }) {
  return <Panel title="近 30 天用量" className={styles.usageSummary}>
    <p className={styles.subtle}>仅展示当前订阅用量账本的真实观察；金额、预算和百分比未由当前 owner 提供。</p>
    <div className={styles.usageSummaryGrid}>{data.usage.slice(0, 3).map(row => <article key={row.metric}>
      <div className={styles.usageSummaryHeading}><h3>{usageLabels[row.metric]}</h3><strong>{formatUsage(row)}</strong></div>
      <p>{usageWindow(row)}</p>
    </article>)}</div>
    <PageLink href="/workbench/plans/usage">查看用量明细</PageLink>
  </Panel>;
}

export function CommercialOverviewView({ data }: { data: CommercialOverview }) {
  const subscription = data.subscription;
  const storeLimit = data.entitlements.flatMap(item => item.limits).find(limit => limit.metric === "store_count");
  const planName = subscription?.plan_name ?? (subscription ? "套餐名称未提供" : "无订阅");
  const storeValue = storeLimit?.kind === "unlimited" ? "不限额" : storeLimit?.value === null || storeLimit?.value === undefined ? "未提供" : `${formatInteger(storeLimit.value)} 家`;
  return <div className={styles.stack}>
    <Panel title="当前方案" className={styles.overviewHero}>
      <div className={styles.overviewHeroHeader}><div><p className={styles.eyebrow}>当前企业的真实订阅观察</p><h3>{planName}</h3><p className={styles.subtle}>订阅状态由商业 owner 返回；方案目录不等于当前订阅，也不会在页面推算权益。</p></div><div className={styles.actions}><PageLink href="/workbench/plans/entitlements">查看我的权益</PageLink><PageLink href="/workbench/plans/top-up">进入钱包</PageLink></div></div>
      <div className={styles.summaryMetrics}>
        <SummaryMetric tone="blue" label="店铺服务" value={storeValue} detail="当前已授予店铺限制" />
        <SummaryMetric tone="teal" label="订阅用量" value={data.usage.filter(item => item.state === "known").length.toString()} detail="可读取的账本项目" />
        <SummaryMetric tone="purple" label="已授予权益" value={`${data.entitlements.length} 项`} detail="明确的企业 grant" />
      </div>
    </Panel>
    <div className={styles.sectionHeading}><h2>管理入口</h2><p>方案、权益、用量、充值和账单分开管理，避免信息混在一起。</p></div>
    <ModuleEntries />
    <div className={styles.overviewColumns}><UsageSummary data={data} /><Panel title="钱包与账单"><p className={styles.subtle}>钱包余额、资金流水、账单汇总与订单由独立商业 owner 提供，不从订阅用量推导。</p><div className={styles.noticeList}><div><strong>企业钱包</strong><span>打开页面读取当前企业钱包快照</span><PageLink href="/workbench/plans/top-up">查看钱包</PageLink></div><div><strong>账单与订单</strong><span>打开页面读取当前企业账单与订单</span><PageLink href="/workbench/plans/orders">查看账单</PageLink></div></div></Panel></div>
  </div>;
}

export function UsageDetailsView({ data }: { data: CommercialOverview }) {
  const [type, setType] = useState<CommercialUsage["metric"] | "all">("all");
  const [time, setTime] = useState<"current" | "all">("current");
  const visibleRows = type === "all" ? data.usage : data.usage.filter(row => row.metric === type);
  return <div className={styles.stack}>
    <div className={styles.usageStats}>{[
      ["订阅用量项目", data.usage.length.toString(), "当前响应中的全部账本指标", "blue"],
      ["已知记录", data.usage.filter(item => item.state === "known").length.toString(), "服务端返回 known", "teal"],
      ["费用金额", "未提供", "当前 owner 未返回人民币成本", "orange"],
    ].map(([label, value, detail, tone]) => <Card key={label} className={`${styles.usageStat} ${styles[tone]}`}><p>{label}</p><strong>{value}</strong><small>{detail}</small></Card>)}</div>
    <Panel title="流水明细" className={styles.usagePanel}>
      <p className={styles.subtle}>可筛选当前真实账本记录；资源用量与人民币成本分开，当前没有成本字段时不会自行换算。</p>
      <div className={styles.filterBar} role="group" aria-label="用量筛选">
        <label>时间范围<select aria-label="时间范围" value={time} onChange={event => setTime(event.target.value as "current" | "all")}><option value="current">当前账期</option><option value="all">全部已返回</option></select></label>
        <label>类型<select aria-label="用量类型" value={type} onChange={event => setType(event.target.value as CommercialUsage["metric"] | "all")}><option value="all">全部类型</option>{Object.entries(usageTypes).map(([metric, label]) => <option key={metric} value={metric}>{label}</option>)}</select></label>
        <label className={styles.filterSearch}>关联说明<input aria-label="关联说明" placeholder="搜索关联说明" disabled /></label>
        <Button type="button" onClick={() => undefined}>筛选</Button>
        <Button type="button" variant="outline" onClick={() => { setType("all"); setTime("current"); }}>重置</Button>
      </div>
      <div className={styles.usageTableWrap}><table className={styles.usageTable}><caption className="sr-only">当前订阅用量明细</caption><thead><tr><th>时间</th><th>类型</th><th>计量指标</th><th>账期</th><th>已记录用量</th><th>预留用量</th><th>金额</th></tr></thead><tbody>{visibleRows.map(row => <tr key={row.metric}><td>{row.updated_at?.slice(0, 16).replace("T", " ") ?? "未提供"}</td><td>{usageTypes[row.metric]}</td><td>{usageLabels[row.metric]}</td><td>{usageWindow(row)}</td><td>{formatUsage(row)}</td><td>{row.state === "known" ? `${formatInteger(row.reserved)} ${unitLabels[row.unit]}` : `未知（${unitLabels[row.unit]}）`}</td><td>未提供</td></tr>)}</tbody></table></div>
      <p className={styles.subtle}>共 {visibleRows.length} 项 · 当前响应仅包含 {time === "current" ? "当前账期" : "已返回"} 的订阅用量账本指标。</p>
    </Panel>
  </div>;
}
