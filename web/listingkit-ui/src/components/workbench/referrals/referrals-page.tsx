"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import {
  completeReferralRegistration,
  createAccountReferralCode,
  getAccountReferrals,
  ReferralRequestError,
  type ReferralReceipt,
} from "@/lib/api/referrals";
import { AccountShell } from "../account/account-shell";
import { ConsoleState } from "../console/console-page";
import styles from "./referrals.module.css";

export function ReferralsPage({ mode, expectedUserId }: { mode: "overview" | "complete"; expectedUserId: string }) {
  const context = useWorkbenchContext();
  const [leaving, setLeaving] = useState(false);
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      const link = event.target instanceof Element ? event.target.closest("a") : null;
      if (link && new URL(link.href, window.location.href).pathname === "/api/zitadel-auth/logout") setLeaving(true);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, []);
  const currentSubject = context.user?.id ?? "";
  const authError = [context.error, context.blockingError].some((error) => error?.code === "AUTHENTICATION_REQUIRED");
  const identityChanged = Boolean(currentSubject && currentSubject !== expectedUserId);
  const pathname = mode === "complete" ? "/workbench/account/referrals/complete" : "/workbench/account/referrals";
  const title = mode === "complete" ? "完成注册" : "推广与收益";
  const description = mode === "complete" ? "用当前登录身份确认这次新注册，并完成不可变推广关系。" : "查看你的推广码、真实关系数量与当前可用汇总。";
  let content: React.ReactNode;
  if (leaving || authError || identityChanged) content = <IdentityError />;
  else if (!currentSubject || context.isLoading) content = <ConsoleState kind="loading" title="正在确认登录身份">旧推广数据已清除。</ConsoleState>;
  else content = <ScopedReferrals key={`${mode}:${expectedUserId}:${currentSubject}`} mode={mode} expectedUserId={expectedUserId} />;
  return <AccountShell pathname={pathname} title={title} description={description}>{content}</AccountShell>;
}

function ScopedReferrals({ mode, expectedUserId }: { mode: "overview" | "complete"; expectedUserId: string }) {
  const queryClient = useQueryClient();
  const [receipt, setReceipt] = useState<ReferralReceipt | null>(null);
  const mutationController = useRef<AbortController | null>(null);
  useEffect(() => () => mutationController.current?.abort(), []);
  const queryKey = ["account", "referrals", expectedUserId] as const;
  const projection = useQuery({
    queryKey,
    queryFn: ({ signal }) => getAccountReferrals(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const createCode = useMutation({
    mutationFn: async () => {
      mutationController.current?.abort();
      mutationController.current = new AbortController();
      return createAccountReferralCode(expectedUserId, mutationController.current.signal);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
  });
  const complete = useMutation({
    mutationFn: async () => {
      mutationController.current?.abort();
      mutationController.current = new AbortController();
      return completeReferralRegistration(expectedUserId, mutationController.current.signal);
    },
    onSuccess: async (result) => {
      setReceipt(result);
      await queryClient.invalidateQueries({ queryKey });
    },
  });

  if (projection.isPending || projection.isFetching) return <ConsoleState kind="loading" title="正在读取推广事实">正在核对当前身份与持久化关系。</ConsoleState>;
  if (projection.isError) return <ReferralError error={projection.error} />;
  const data = projection.data;
  return <div className={styles.pageBody}>
    {mode === "complete" ? <section className={styles.notice} aria-labelledby="complete-title">
      <h2 id="complete-title">确认新注册关系</h2>
      <p>系统只会按当前 Auth.js 登录 subject 查找同一注册 Intent，并重新核对官方验证事实。</p>
      {receipt ? <div role="status"><strong>推广关系已确认</strong><p>确认时间：{formatTime(receipt.boundAt)}</p></div> : <Button type="button" onClick={() => complete.mutate()} disabled={complete.isPending}>{complete.isPending ? "正在确认…" : "完成推广关系"}</Button>}
      {complete.isError ? <ReferralError error={complete.error} compact /> : null}
    </section> : null}
    <section className={styles.grid} aria-label="推广概览">
      <article className={styles.metric}><span>已建立关系</span><strong>{data.count}</strong><small>来自不可变推广关系的实时计数</small></article>
      <article className={styles.metric}><span>收益状态</span><strong className={styles.unavailable}>收益数据暂不可用</strong><small>当前没有收益金额的权威数据源</small></article>
    </section>
    <section className={styles.codeCard} aria-labelledby="code-title"><div><h2 id="code-title">我的推广码</h2>{data.codeAvailability === "available" ? <><code>{data.code}</code><p>生成于本次读取：{formatTime(data.generatedAt)}</p></> : <p>尚未创建推广码</p>}</div>{data.codeAvailability === "available" ? <Button asChild variant="outline"><Link href={`/referrals/register?code=${encodeURIComponent(data.code)}`} prefetch={false}>打开邀请链接</Link></Button> : <Button type="button" onClick={() => createCode.mutate()} disabled={createCode.isPending}>{createCode.isPending ? "正在创建…" : "创建推广码"}</Button>}</section>
    {createCode.isError ? <ReferralError error={createCode.error} compact /> : null}
    <p className={styles.observed}>数据观察时间：{formatTime(data.generatedAt)}</p>
  </div>;
}

function IdentityError() {
  return <ConsoleState kind="error" title="登录身份已变化"><p>旧推广数据已清除。请重新登录后读取当前账户。</p><Button asChild variant="outline"><Link href="/login?returnTo=%2Fworkbench%2Faccount%2Freferrals" prefetch={false}>重新登录</Link></Button></ConsoleState>;
}

function ReferralError({ error, compact = false }: { error: unknown; compact?: boolean }) {
  const code = error instanceof ReferralRequestError ? error.code : "DEPENDENCY_UNAVAILABLE";
  const title: Record<string, string> = {
    AUTHENTICATION_REQUIRED: "登录已失效", IDENTITY_CONTEXT_CHANGED: "登录身份已变化",
    referral_authentication_required: "登录已失效", referral_verification_pending: "官方邮箱或认证方式尚未完成",
    referral_expired: "注册确认期限已结束", referral_conflict: "推广关系存在冲突",
    referral_outcome_unknown: "暂时无法确认操作结果", REFERRALS_NOT_CONFIGURED: "推广服务尚未配置",
    DEPENDENCY_UNAVAILABLE: "推广服务暂不可用", DEADLINE_EXCEEDED: "推广请求超时",
  };
  const content = <><strong>{title[code] ?? "推广请求未完成"}</strong><p>没有生成或推测推广事实。请稍后重试原操作。</p></>;
  return compact ? <div className={styles.error} role="alert">{content}</div> : <ConsoleState kind="error" title={title[code] ?? "推广请求未完成"}><p>没有生成或推测推广事实。请稍后重试原操作。</p></ConsoleState>;
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value));
}
