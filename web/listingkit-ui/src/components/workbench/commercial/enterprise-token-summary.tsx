"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Card } from "@/components/ui/card";
import { getMemberTokenAllocations } from "@/lib/api/account-allocation";
import styles from "./commercial.module.css";

const utc = (value: string) => value.replace("T", " ").replace(/Z$/, " UTC");

export function EnterpriseTokenSummary({ userId, organizationId, sequence }: { userId: string; organizationId: string; sequence: number }) {
  const response = useQuery({
    queryKey: ["workbench", userId, organizationId, "member-token-allocations", sequence],
    queryFn: ({ signal }) => getMemberTokenAllocations({ expectedUserId: userId, expectedOrganizationId: organizationId }, signal),
    gcTime: 0, staleTime: 0, retry: false,
    refetchOnWindowFocus: false, refetchOnReconnect: false,
  });
  const loading = response.isPending || response.isFetching;
  const data = !loading && !response.isError && response.data?.organizationId === organizationId ? response.data : undefined;
  return <Card role="region" aria-label="AI Token 分配额度" className={styles.panel}>
    <h2>AI Token 分配额度</h2>
    {loading ? <p role="status">正在读取当前企业 Token 分配额度…</p> : !data ?
      <p role="alert">本次未取得 AI Token 分配额度，请确认当前企业后刷新。</p> : <>
        <p className={styles.resourceValue}>{data.enterprise.total} Token</p>
        <dl className={styles.facts}>
          <div><dt>已分配</dt><dd>{data.enterprise.allocated} Token</dd></div>
          <div><dt>未分配</dt><dd>{data.enterprise.unallocated} Token</dd></div>
          <div><dt>分配周期</dt><dd><time dateTime={data.windowStart}>{utc(data.windowStart)}</time> 至 <time dateTime={data.windowEnd}>{utc(data.windowEnd)}</time>（期末不含）</dd></div>
        </dl>
      </>}
    <p className={styles.subtle}>企业成员分配额度按 Token 计量，不折算为 AI 点数；生成图片扣减的 AI 点数余额单独展示。</p>
    <Link href="/workbench/account/organization/resources" prefetch={false}>查看成员 Token 分配</Link>
  </Card>;
}
