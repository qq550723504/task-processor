import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import type { CommercialOverview, CommercialSubscription } from "@/lib/api/commercial";
import styles from "./commercial.module.css";

const statusLabels: Record<CommercialSubscription["effective_status"], string> = { active: "生效中", trialing: "试用中", expired: "已过期", disabled: "已停用", not_started: "尚未开始" };
const moduleLabels: Record<string, string> = { store_management: "店铺管理", task_import: "导入模块", rules: "规则模块", operation_strategy: "运营策略", listingkit: "商品资料处理", oss_storage: "对象存储" };
const metricLabels: Record<string, string> = { store_count: "店铺数量限制", listingkit_generations_succeeded: "资料生成作业", product_image_jobs_succeeded: "商品图作业", shein_drafts_succeeded: "SHEIN 草稿", shein_publishes_succeeded: "SHEIN 发布", storage_bytes_current: "当前保留存储" };
const units = { store: "家", operation: "作业次", byte: "字节" };

function Timestamp({ value, missing = "未提供" }: { value: string | null; missing?: string }) {
  return value ? <time dateTime={value}>{value.replace("T", " ").replace(/Z$/, " UTC")}</time> : <span>{missing}</span>;
}
function Panel({ title, children, className = "" }: { title: string; children: ReactNode; className?: string }) {
  return <Card role="region" aria-label={title} className={`${styles.panel} ${className}`}><h2>{title}</h2>{children}</Card>;
}
function Observation({ data }: { data: CommercialOverview }) {
  return <p className={styles.observation}>数据观察时间：<Timestamp value={data.observed_at} /> · 来源：企业订阅与已授予权益、订阅用量账本。数值精度：整数；金额与币种未提供。</p>;
}
function Validity({ row }: { row: CommercialSubscription | CommercialOverview["entitlements"][number] }) {
  return <dl className={styles.facts}>
    <div><dt>生效状态</dt><dd className={styles.status} data-status={row.effective_status}>{statusLabels[row.effective_status]}</dd></div>
    <div><dt>登记状态</dt><dd>{statusLabels[row.status]}</dd></div>
    <div><dt>有效期起点</dt><dd><Timestamp value={row.starts_at} missing="未设起点" /></dd></div>
    <div><dt>有效期终点</dt><dd><Timestamp value={row.expires_at} missing="未设终点" /></dd></div>
    <div><dt>更新时间</dt><dd><Timestamp value={row.updated_at} /></dd></div>
  </dl>;
}

export function EntitlementsOverview({ data }: { data: CommercialOverview }) {
  return <div className={styles.stack}>
    <Observation data={data} />
    <div className={styles.sectionHeading}><h2>权益构成</h2><p>资源、已授予权益与用量分别呈现。</p></div>
    <div className={styles.twoColumns}>
      <Panel title="已授予权益">
        <p className={styles.subtle}>仅展示已授予的限制，不含方案继承额度；不代表所有平台功能均已开通，也不代表成员访问权限。</p>
        {data.entitlements.length === 0 ? <p>暂无已授予权益</p> : <div className={styles.grants}>{data.entitlements.map(grant => <article key={grant.module_code}>
          <h3>{moduleLabels[grant.module_code] ?? grant.module_code}</h3><Validity row={grant} />
          {grant.limits.length ? <dl className={styles.limits}>{grant.limits.map(limit => <div key={limit.metric}><dt>{metricLabels[limit.metric] ?? limit.metric}</dt><dd>{limit.kind === "unlimited" ? `不限额（${units[limit.unit]}）` : `${limit.value} ${units[limit.unit]}`}</dd></div>)}</dl> : <p className={styles.subtle}>未提供已解释的限制；不能判断为不限额。</p>}
          {grant.uninterpreted_limit_count > 0 ? <p className={styles.subtle}>另有 {grant.uninterpreted_limit_count} 项配置未在此摘要展开；Token 分配额度单独读取。</p> : null}
        </article>)}</div>}
      </Panel>
      <Panel title="资源与余额说明"><p className={styles.subtle}>成员 AI Token 分配请查看“我的账户 → 企业空间 → 资源与额度”；可用状态和操作权限以该页实际读取结果为准。</p>
        <p className={styles.subtle}>此处不展示现金余额；现金、AI 点数与数据额度均不从订阅用量推算。</p>
        <p className={styles.subtle}>钱包余额、充值状态与用量明细可在“套餐与权益”中查看。</p>
      </Panel>
    </div>
    <Panel title="实际订阅">{data.subscription ? <><h3>{data.subscription.plan_name ?? "套餐名称未提供"}</h3><p className={styles.subtle}>实际套餐代码：{data.subscription.plan_code}</p><Validity row={data.subscription} /></> : <p>无订阅</p>}</Panel>
    <Panel title="已记录用量概要">
      <p className={styles.subtle}>来源：订阅用量账本。操作按 UTC 自然月观察，不是购买账期。存储为当前保留字节；商品图作业不是图片张数。所有数量为精确整数。</p>
      <div className={styles.usage}>{data.usage.map(row => <article key={row.metric}>
        <h3>{metricLabels[row.metric] ?? row.metric}</h3>
        <p className={styles.subtle}>{row.unit === "byte" ? "当前存储观测，无统计起止" : <>统计周期：<Timestamp value={row.window_start} /> 至 <Timestamp value={row.window_end} />（期末不含）</>}</p>
        <dl className={styles.facts}>
          <div><dt>{row.unit === "byte" ? "当前已记账存储" : "已记账用量"}</dt><dd>{row.state === "unknown" ? `未知（${units[row.unit]}）` : `${row.committed} ${units[row.unit]}`}</dd></div>
          <div><dt>{row.unit === "byte" ? "待处理净字节变化" : "预留用量"}</dt><dd>{row.state === "unknown" ? `未知（${units[row.unit]}）` : `${row.reserved} ${units[row.unit]}`}</dd></div>
          <div><dt>数据状态</dt><dd>{row.state === "known" ? "已记录" : "未知（尚无记录）"}</dd></div>
          <div><dt>用量更新时间</dt><dd><Timestamp value={row.updated_at} missing="未知" /></dd></div>
        </dl>
      </article>)}</div>
    </Panel>
  </div>;
}
