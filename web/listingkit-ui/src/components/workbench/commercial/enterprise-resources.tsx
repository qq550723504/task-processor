"use client";

import { useQuery } from "@tanstack/react-query";
import Image from "next/image";
import Link from "next/link";
import { Card } from "@/components/ui/card";
import { getCommercialResources, type CommercialResources } from "@/lib/api/commercial-billing";
import styles from "./commercial.module.css";

const definitions = [
  { type: "store_renewal_period", name: "店铺续费期数", color: "blue", unit: "期", note: "续费期数不代表已绑定、服务中或到期的店铺数量。" },
  { type: "ai_point", name: "AI 点数", color: "teal", unit: "点", note: "AI 点数按配置费率扣减；模型 Token 单独记录用量，钱包资金用于购买点数。" },
  { type: "data_row", name: "数据额度", color: "purple", unit: "条", note: "按数据条数计量，不以操作次数或存储字节替代。" },
] as const;

export function EnterpriseResources({ userId, organizationId, scope, sequence, showSummary = false }: { userId: string; organizationId: string; scope: string; sequence: number; showSummary?: boolean }) {
  const response = useQuery({ queryKey: ["workbench", "commercial-resources", userId, organizationId, scope, sequence], queryFn: ({ signal }) => getCommercialResources(userId, organizationId, signal), gcTime: 0, staleTime: 0, retry: false, refetchOnWindowFocus: false, refetchOnReconnect: false });
  const loading = response.isPending || response.isFetching;
  const data = !loading && !response.isError && response.data?.organization_id === organizationId ? response.data : undefined;
  const code = response.error && "code" in response.error ? String(response.error.code) : "INVALID_UPSTREAM_RESPONSE";
  const errors: Record<string, string> = { PERMISSION_DENIED: "无资源查看权限", AUTHENTICATION_REQUIRED: "登录已失效", ORGANIZATION_ACCESS_REVOKED: "企业访问已撤销", ORGANIZATION_ACCESS_DENIED: "企业访问被拒绝", ORGANIZATION_SUSPENDED: "企业已暂停访问", ORGANIZATION_CONTEXT_CHANGED: "企业上下文已变化", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", DEPENDENCY_UNAVAILABLE: "资源服务暂不可用", DEADLINE_EXCEEDED: "资源读取超时" };
  const fallback = loading ? "正在读取" : "本次未取得";
  const balance = (kind: CommercialResources["resources"][number]["resource_type"]) => data?.resources.find(v => v.resource_type === kind);
  const values = definitions.map(definition => {
    const resource = balance(definition.type);
    return { ...definition, resource, value: resource?.state === "recorded" ? `${BigInt(resource.available) + BigInt(resource.allocated)} ${definition.unit}` : resource?.state === "not_recorded" ? "尚无资源记录" : fallback };
  });
  return <div className={styles.stack}>
    {loading ? <p role="status">正在读取当前企业资源余额</p> : !data ? <p role="alert">{errors[code] ?? "资源响应无效"}：本次未取得余额，请确认企业后刷新。</p> : <p className={styles.observation}>资源观察时间：<time dateTime={data.observed_at}>{data.observed_at.replace("T", " ").replace(/Z$/, " UTC")}</time></p>}
    {showSummary ? <Card role="region" aria-label="企业总权益" className={`${styles.panel} ${styles.summary}`}><h2>企业总权益</h2><p className={styles.subtle}>企业可用资源余额；成员月度消费上限与店铺服务期分别展示。</p><div className={styles.resourceSummary}>{values.map(card => <div className={styles[card.color]} key={card.type}><p>{card.name}余额</p><strong>{card.value}</strong><small>{card.resource?.state === "recorded" ? "企业可用余额" : card.value}</small></div>)}</div></Card> : null}
    <div className={styles.threeColumns}>{values.map(card => <Card role="region" aria-label={`${card.name}余额`} className={`${styles.resourceCard} ${styles[card.color]}`} key={card.type}>
      <div className={styles.cardHeading}><h3>{card.name}</h3><span>{card.resource?.state === "recorded" ? "已记录" : card.resource?.state === "not_recorded" ? "尚无记录" : fallback}</span></div>
      <p className={styles.resourceValue}>{card.value}</p>
      {card.resource?.state === "recorded" && card.type !== "ai_point" ? <p className={styles.subtle}>未分配（管理员可用）{card.resource.available} {card.unit} · 成员可用 {card.resource.allocated} {card.unit}</p> : null}
      <ul>
        {card.resource?.state === "recorded" ? <><li><Image src={`/console/commercial/bullet-${card.color}.svg`} alt="" width={5} height={5} unoptimized /><span>预留 {card.resource.reserved} {card.unit} · 已消费 {card.resource.consumed} {card.unit}</span></li>{BigInt(card.resource.debt) > BigInt(0) ? <li><span>待偿还 {card.resource.debt} {card.unit}</span></li> : null}</> : null}
        <li><Image src={`/console/commercial/bullet-${card.color}.svg`} alt="" width={5} height={5} unoptimized /><span>{card.note}</span></li>
      </ul>
      {card.resource?.state === "recorded" ? <p className={styles.subtle}>更新时间：<time dateTime={card.resource.updated_at}>{card.resource.updated_at.replace("T", " ").replace(/Z$/, " UTC")}</time></p> : null}
    </Card>)}</div>
    <p className={styles.subtle}><Link href="/workbench/account/organization/resources" prefetch={false}>管理成员额度</Link> · <Link href="/workbench/plans/top-up" prefetch={false}>查看钱包与充值状态</Link></p>
  </div>;
}
