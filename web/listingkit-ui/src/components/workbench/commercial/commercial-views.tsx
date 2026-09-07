import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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

function ResourceCards({ options = false }: { options?: boolean }) {
  const cards = [
    { name: "店铺服务", color: "blue", value: options ? "价格未提供" : "使用数量未提供", details: [options ? "币种与计价周期尚未提供" : "服务中的店铺数、到期数尚未提供", "绑定店铺不开始计费，显式激活才开始服务周期", "本页不提供购买、续费或重新激活"] },
    { name: "AI 点数", color: "teal", value: "资源余额未提供", details: ["企业 AI 点数读取尚未接入", "AI 点数不等于模型 token 数，也不等于现金", "充值与资源购买暂未开放"] },
    { name: "数据额度", color: "purple", value: "资源余额未提供", details: ["企业数据额度读取尚未接入", "单位为条；未提供数量、有效期或换算价格", "不以操作次数或存储字节替代数据额度"] },
  ];
  return <div className={styles.threeColumns}>{cards.map(card => <Card className={`${styles.resourceCard} ${styles[card.color]}`} key={card.name}>
    <div className={styles.cardHeading}><h3>{card.name}</h3><span>暂未提供</span></div>
    <p className={styles.resourceValue}>{card.value}</p>
    <ul>{card.details.map(detail => <li key={detail}><Image src={`/console/commercial/bullet-${card.color}.svg`} alt="" width={5} height={5} unoptimized /><span>{detail}</span></li>)}</ul>
  </Card>)}</div>;
}

export function PlanOptions({ data }: { data: CommercialOverview }) {
  return <div className={styles.stack}>
    <Observation data={data} />
    <Panel title="方案说明" className={styles.summary}>
      {data.plans.map(plan => <article key={plan.code} className={styles.plan}>
        <p className={styles.eyebrow}>{plan.availability === "invitation_only" ? "方案描述 · 仅限受邀" : "方案描述 · 暂不销售"}</p>
        <h3>{plan.name}</h3><p className={styles.subtle}>方案代码：{plan.code}</p>
        <p>价格未提供 · 币种未提供</p>
        <p className={styles.subtle}>来源：已批准的产品说明。方案说明不代表当前订阅，也不授予权益；计价周期尚未提供。</p>
      </article>)}
    </Panel>
    <div className={styles.sectionHeading}><h2>收费构成</h2><p>价格与资源获取能力尚未提供，以下项目分别展示。</p></div>
    <ResourceCards options />
    <div className={styles.twoColumns}>
      <Panel title="计费与数据说明"><ol className={styles.rules}>
        <li>方案目录、实际订阅和已授予权益是不同信息。</li>
        <li>权益以当前企业读取结果为准，不根据方案名称推算。</li>
        <li>已记录用量不是账单，也不是 AI 点数、数据或现金余额。</li>
        <li>购买、充值、续费、退款与提现暂未开放。</li>
      </ol></Panel>
      <Panel title="相关管理"><Button asChild variant="outline"><Link href="/workbench/plans/entitlements" prefetch={false}>查看当前权益</Link></Button>
        <p className={styles.subtle}>钱包、用量明细、账单与订单暂未开放。</p><p className={styles.subtle}>成员资源分配不在本片范围。</p>
      </Panel>
    </div>
  </div>;
}

export function EntitlementsOverview({ data }: { data: CommercialOverview }) {
  return <div className={styles.stack}>
    <Observation data={data} />
    <Panel title="企业总权益" className={styles.summary}>
      <p className={styles.subtle}>企业资源读取尚未接入；下方另列实际订阅、已授予权益和已记录用量。</p>
      <div className={styles.resourceSummary}>
        <div className={styles.blue}><p>店铺服务</p><strong>使用数量未提供</strong><small>有效期与到期数量未提供</small></div>
        <div className={styles.teal}><p>AI 点数余额</p><strong>未提供</strong><small>资源余额 · 未接入</small></div>
        <div className={styles.purple}><p>数据额度余额</p><strong>未提供</strong><small>资源余额 · 未接入</small></div>
      </div>
    </Panel>
    <div className={styles.sectionHeading}><h2>权益构成</h2><p>资源、已授予权益与用量分别呈现。</p></div>
    <ResourceCards />
    <div className={styles.twoColumns}>
      <Panel title="已授予权益">
        <p className={styles.subtle}>仅展示已授予的限制，不含方案继承额度；不代表所有平台功能均已开通，也不代表成员访问权限。</p>
        {data.entitlements.length === 0 ? <p>暂无已授予权益</p> : <div className={styles.grants}>{data.entitlements.map(grant => <article key={grant.module_code}>
          <h3>{moduleLabels[grant.module_code] ?? grant.module_code}</h3><Validity row={grant} />
          {grant.limits.length ? <dl className={styles.limits}>{grant.limits.map(limit => <div key={limit.metric}><dt>{metricLabels[limit.metric] ?? limit.metric}</dt><dd>{limit.kind === "unlimited" ? `不限额（${units[limit.unit]}）` : `${limit.value} ${units[limit.unit]}`}</dd></div>)}</dl> : <p className={styles.subtle}>未提供已解释的限制；不能判断为不限额。</p>}
          {grant.uninterpreted_limit_count > 0 ? <p className={styles.subtle}>另有 {grant.uninterpreted_limit_count} 项限制尚未解释。</p> : null}
        </article>)}</div>}
      </Panel>
      <Panel title="资源分配在哪里？"><p className={styles.subtle}>本页只读展示企业信息。成员资源分配尚未开放。</p>
        <p className={styles.subtle}>现金余额：本片未提供。现金、AI 点数与数据额度均不从订阅用量推算。</p>
        <p className={styles.subtle}>钱包、充值与用量明细暂未开放。</p>
      </Panel>
    </div>
    <Panel title="实际订阅">{data.subscription ? <><h3>{data.subscription.plan_name ?? "套餐名称未提供"}</h3><p className={styles.subtle}>实际套餐代码：{data.subscription.plan_code}</p><Validity row={data.subscription} /></> : <p>无订阅</p>}</Panel>
    <Panel title="已记录用量概要">
      <p className={styles.subtle}>来源：订阅用量账本。操作按 UTC 自然月观察，不是购买账期。存储为当前保留字节；商品图作业不是图片张数。所有数量为精确整数。</p>
      <div className={styles.usage}>{data.usage.map(row => <article key={row.metric}>
        <h3>{metricLabels[row.metric] ?? row.metric}</h3>
        <p className={styles.subtle}>{row.unit === "byte" ? "当前存储观测，无统计起止" : <>统计周期：<Timestamp value={row.window_start} /> 至 <Timestamp value={row.window_end} />（期末不含）</>}</p>
        <dl className={styles.facts}>
          <div><dt>{row.unit === "byte" ? "当前已记账存储" : "已记账用量"}</dt><dd>{row.state === "unknown" ? "未知" : `${row.committed} ${units[row.unit]}`}</dd></div>
          <div><dt>{row.unit === "byte" ? "待处理净字节变化" : "预留用量"}</dt><dd>{row.state === "unknown" ? "未知" : `${row.reserved} ${units[row.unit]}`}</dd></div>
          <div><dt>数据状态</dt><dd>{row.state === "known" ? "已记录" : "未知（尚无记录）"}</dd></div>
          <div><dt>用量更新时间</dt><dd><Timestamp value={row.updated_at} missing="未知" /></dd></div>
        </dl>
      </article>)}</div>
    </Panel>
  </div>;
}
