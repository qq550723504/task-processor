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
import { getReferralPayoutMethods, requestReferralWithdrawal, type PayoutMethod } from "@/lib/api/account-referral-economics";
import { AccountShell } from "../account/account-shell";
import { ConsoleState } from "../console/console-page";
import styles from "./referrals.module.css";

export function ReferralsPage({ mode, expectedUserId, registrationAvailable = true }: { mode: "overview" | "complete"; expectedUserId: string; registrationAvailable?: boolean }) {
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
  const pathname = mode === "complete" ? "/workbench/account/referrals/complete" : "/workbench/account/referrals";
  const title = mode === "complete" ? "完成注册" : "推广与收益";
  const description = mode === "complete" ? "用当前登录身份确认这次新注册，并完成不可变推广关系。" : "查看你的推广码、真实关系数量与当前可用汇总。";
  const authError = [context.error, context.blockingError].some((error) => error?.code === "AUTHENTICATION_REQUIRED");
  const identityChanged = Boolean(context.user && context.user.id !== expectedUserId);
  let content: React.ReactNode;
  if (leaving || authError || identityChanged) content = <IdentityError />;
  else content = <ScopedReferrals key={`${mode}:${expectedUserId}`} mode={mode} expectedUserId={expectedUserId} registrationAvailable={registrationAvailable} />;
  return <AccountShell pathname={pathname} title={title} description={description}>{content}</AccountShell>;
}

function ScopedReferrals({ mode, expectedUserId, registrationAvailable }: { mode: "overview" | "complete"; expectedUserId: string; registrationAvailable: boolean }) {
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
  const payoutMethods = useQuery({
    queryKey: ["account", "referral-payout-methods", expectedUserId] as const,
    queryFn: ({ signal }) => getReferralPayoutMethods(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled: mode === "overview" && projection.data?.earnings.availability === "available",
  });
  const [amountMinor, setAmountMinor] = useState("");
  const [payoutMethodId, setPayoutMethodId] = useState("");
  const selectedPayoutMethod = payoutMethods.data?.methods.find((item) => item.methodId === payoutMethodId) ?? payoutMethods.data?.methods[0];
  const withdrawal = useMutation({
    mutationFn: () => {
      const currentEarnings = projection.data?.earnings;
      if (!currentEarnings || currentEarnings.availability !== "available") throw new Error("earnings unavailable");
      if (!selectedPayoutMethod) throw new Error("payout method unavailable");
      return requestReferralWithdrawal(expectedUserId, { amountMinor, payoutMethodId: selectedPayoutMethod.methodId, expectedVersion: currentEarnings.version }, crypto.randomUUID());
    },
    onSuccess: () => { setAmountMinor(""); void queryClient.invalidateQueries({ queryKey }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-payout-methods", expectedUserId] }); },
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
    onSuccess: (result) => { setReceipt(result); void queryClient.invalidateQueries({ queryKey }); },
  });

  if (projection.isPending || projection.isFetching) return <ConsoleState kind="loading" title="正在读取推广事实">正在核对当前身份与持久化关系。</ConsoleState>;
  if (projection.isError && isIdentityError(projection.error)) return <IdentityError />;
  if (projection.isError && !receipt) return <ReferralError error={projection.error} />;
  if (projection.isError) return <div className={styles.pageBody}><section className={styles.notice} role="status"><strong>推广关系已确认</strong><p>确认时间：{formatTime(receipt!.boundAt)}</p></section><ConsoleState kind="error" title="推广汇总暂不可用"><p>关系回执已保留，请稍后重试汇总读取。</p></ConsoleState></div>;
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
      <article className={styles.metric}><span>可提现收益</span><strong className={data.earnings.availability === "available" ? undefined : styles.unavailable}>{data.earnings.availability === "available" ? formatMinor(data.earnings.availableMinor) : "收益数据暂不可用"}</strong><small>{data.earnings.availability === "available" ? `待结算 ${formatMinor(data.earnings.pendingMinor)} · 冻结 ${formatMinor(data.earnings.reservedMinor)}` : "当前没有可读取的收益 projection"}</small></article>
    </section>
    <section className={styles.codeCard} aria-labelledby="code-title"><div><h2 id="code-title">我的推广码</h2>{data.codeAvailability === "available" ? <><code>{data.code}</code><p>生成于本次读取：{formatTime(data.generatedAt)}</p></> : <p>尚未创建推广码</p>}</div>{data.codeAvailability === "available" ? registrationAvailable ? <Button asChild variant="outline"><Link href={`/referrals/register?code=${encodeURIComponent(data.code)}`} prefetch={false}>打开邀请链接</Link></Button> : <span className={styles.unavailable}>注册入口暂不可用</span> : <Button type="button" onClick={() => createCode.mutate()} disabled={createCode.isPending}>{createCode.isPending ? "正在创建…" : "创建推广码"}</Button>}</section>
    <section className={styles.codeCard} aria-labelledby="withdrawal-title"><div><h2 id="withdrawal-title">申请提现</h2><p>最低 ¥100；申请后进入人工审核。结算前提是账户已完成邮箱和手机号验证。</p></div>{payoutMethods.isPending ? <p>正在读取已验证收款方式…</p> : payoutMethods.isError ? <p className={styles.error} role="alert">收款方式暂不可用，请稍后重试。</p> : payoutMethods.data?.methods.length === 0 ? <p className={styles.unavailable}>暂无可用收款方式，请先完成收款方式登记。</p> : <form onSubmit={event => { event.preventDefault(); withdrawal.mutate(); }}><label>金额（分）<input inputMode="numeric" pattern="[0-9]*" value={amountMinor} onChange={event => setAmountMinor(event.target.value)} placeholder="10000" disabled={data.earnings.availability !== "available" || withdrawal.isPending} /></label><label>收款方式<select value={selectedPayoutMethod?.methodId ?? ""} onChange={event => setPayoutMethodId(event.target.value)} disabled={withdrawal.isPending}>{payoutMethods.data?.methods.map((item: PayoutMethod) => <option key={item.methodId} value={item.methodId}>{item.displayName} · {item.maskedDestination}</option>)}</select></label><Button type="submit" disabled={data.earnings.availability !== "available" || withdrawal.isPending || amountMinor === "" || !selectedPayoutMethod}>{withdrawal.isPending ? "提交中…" : "申请提现"}</Button>{withdrawal.isError ? <p className={styles.error} role="alert">提现未提交：请确认余额、验证状态和版本仍有效。</p> : null}{withdrawal.isSuccess ? <p className={styles.notice} role="status">提现申请已提交，状态：{withdrawal.data.status}。</p> : null}</form>}</section>
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
  const message = ["referral_outcome_unknown", "DEADLINE_EXCEEDED"].includes(code) ||
    compact && ["DEPENDENCY_UNAVAILABLE", "INVALID_UPSTREAM_RESPONSE"].includes(code)
    ? "操作结果尚未确认。请使用原操作恢复或稍后重新核对。"
    : "没有生成或推测推广事实。请稍后重试原操作。";
  const content = <><strong>{title[code] ?? "推广请求未完成"}</strong><p>{message}</p></>;
  return compact ? <div className={styles.error} role="alert">{content}</div> : <ConsoleState kind="error" title={title[code] ?? "推广请求未完成"}><p>{message}</p></ConsoleState>;
}

function isIdentityError(error: unknown) {
  return error instanceof ReferralRequestError && ["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED", "referral_authentication_required"].includes(error.code);
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Shanghai" }).format(new Date(value));
}
function formatMinor(value: string) {
  const negative = value.startsWith("-");
  const digits = negative ? value.slice(1) : value;
  const padded = digits.padStart(3, "0");
  return `${negative ? "-" : ""}¥${padded.slice(0, -2)}.${padded.slice(-2)}`;
}
