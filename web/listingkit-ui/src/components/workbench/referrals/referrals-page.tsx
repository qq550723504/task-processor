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
import { AccountReadError } from "@/lib/api/account";
import { cancelReferralWithdrawal, createReferralPayoutMethod, getReferralEarnings, getReferralPayoutMethods, getReferralRules, getReferralWithdrawals, requestReferralWithdrawal, type PayoutMethod, type ReferralEarnings, type ReferralRules, type ReferralWithdrawal } from "@/lib/api/account-referral-economics";
import { AccountShell } from "../account/account-shell";
import { ConsoleState } from "../console/console-page";
import styles from "./referrals.module.css";

export type ReferralView = "overview" | "complete" | "center" | "earnings" | "withdrawals" | "rules";
const referralPaths: Record<ReferralView, string> = {
  overview: "/workbench/account/referrals",
  complete: "/workbench/account/referrals/complete",
  center: "/workbench/account/referrals/center",
  earnings: "/workbench/account/referrals/earnings",
  withdrawals: "/workbench/account/referrals/withdrawals",
  rules: "/workbench/account/referrals/rules",
};
const referralTitles: Record<ReferralView, string> = { overview: "推广与收益", complete: "完成注册", center: "推广中心", earnings: "收益明细", withdrawals: "提现管理", rules: "推广规则" };

export function ReferralsPage({ mode, view = mode, expectedUserId, registrationAvailable = true }: { mode: "overview" | "complete"; view?: ReferralView; expectedUserId: string; registrationAvailable?: boolean }) {
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
  const pathname = referralPaths[view];
  const title = referralTitles[view];
  const description = view === "complete" ? "用当前登录身份确认这次新注册，并完成不可变推广关系。" : view === "center" ? "查看推广码、推广关系与邀请入口。" : view === "earnings" ? "查看来自权威支付事实的收益汇总。" : view === "withdrawals" ? "登记收款方式并管理人工提现申请。" : view === "rules" ? "查看收益结算、退款调整与提现规则。" : "查看你的推广码、真实关系数量与当前可用汇总。";
  const authError = [context.error, context.blockingError].some((error) => error?.code === "AUTHENTICATION_REQUIRED");
  const identityChanged = Boolean(context.user && context.user.id !== expectedUserId);
  let content: React.ReactNode;
  if (leaving || authError || identityChanged) content = <IdentityError returnTo={pathname} />;
  else content = <ScopedReferrals key={`${view}:${expectedUserId}`} mode={mode} view={view} expectedUserId={expectedUserId} registrationAvailable={registrationAvailable} />;
  return <AccountShell pathname={pathname} title={title} description={description}>{content}</AccountShell>;
}

function ScopedReferrals({ mode, view, expectedUserId, registrationAvailable }: { mode: "overview" | "complete"; view: ReferralView; expectedUserId: string; registrationAvailable: boolean }) {
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
    enabled: view !== "earnings" && view !== "rules",
  });
  const earnings = useQuery({
    queryKey: ["account", "referral-earnings", expectedUserId] as const,
    queryFn: ({ signal }) => getReferralEarnings(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled: view === "earnings",
  });
  const rules = useQuery({
    queryKey: ["account", "referral-rules"] as const,
    queryFn: ({ signal }) => getReferralRules(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled: view === "rules",
  });
  const payoutMethods = useQuery({
    queryKey: ["account", "referral-payout-methods", expectedUserId] as const,
    queryFn: ({ signal }) => getReferralPayoutMethods(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled: ["overview", "withdrawals"].includes(view) && projection.data?.earnings.availability === "available",
  });
  const withdrawalHistory = useQuery({
    queryKey: ["account", "referral-withdrawals", expectedUserId] as const,
    queryFn: ({ signal }) => getReferralWithdrawals(expectedUserId, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    enabled: ["overview", "withdrawals"].includes(view),
  });
  const [amountMinor, setAmountMinor] = useState("");
  const [payoutMethodId, setPayoutMethodId] = useState("");
  const [payoutType, setPayoutType] = useState<"ALIPAY" | "BANK_TRANSFER">("ALIPAY");
  const [payoutDisplayName, setPayoutDisplayName] = useState("");
  const [payoutDestination, setPayoutDestination] = useState("");
  const [withdrawalResult, setWithdrawalResult] = useState<ReferralWithdrawal | null>(null);
  const selectedPayoutMethod = payoutMethods.data?.methods.find((item) => item.methodId === payoutMethodId) ?? payoutMethods.data?.methods[0];
  const withdrawal = useMutation({
    mutationFn: () => {
      const currentEarnings = projection.data?.earnings;
      if (!currentEarnings || currentEarnings.availability !== "available") throw new Error("earnings unavailable");
      if (!selectedPayoutMethod) throw new Error("payout method unavailable");
      return requestReferralWithdrawal(expectedUserId, { amountMinor, payoutMethodId: selectedPayoutMethod.methodId, expectedVersion: currentEarnings.version }, crypto.randomUUID());
    },
    onSuccess: (result) => { setWithdrawalResult(result); setAmountMinor(""); void queryClient.invalidateQueries({ queryKey }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-payout-methods", expectedUserId] }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-withdrawals", expectedUserId] }); },
  });
  const cancelTarget = withdrawalResult ?? withdrawalHistory.data?.withdrawals.find((item) => item.status === "REQUESTED");
  const createPayoutMethod = useMutation({
    mutationFn: () => createReferralPayoutMethod(expectedUserId, { type: payoutType, displayName: payoutDisplayName, destination: payoutDestination }, crypto.randomUUID()),
    onSuccess: () => { setPayoutDisplayName(""); setPayoutDestination(""); void queryClient.invalidateQueries({ queryKey: ["account", "referral-payout-methods", expectedUserId] }); },
  });
  const cancelWithdrawal = useMutation({
    mutationFn: () => {
      if (!cancelTarget || cancelTarget.status !== "REQUESTED") throw new Error("withdrawal not cancelable");
      return cancelReferralWithdrawal(expectedUserId, cancelTarget.id, cancelTarget.version, crypto.randomUUID());
    },
    onSuccess: (result) => { setWithdrawalResult(result); void queryClient.invalidateQueries({ queryKey }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-withdrawals", expectedUserId] }); },
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

  if (view === "rules") {
    if (rules.isPending || rules.isFetching) return <ConsoleState kind="loading" title="正在读取推广规则">正在核对当前收益合同。</ConsoleState>;
    if (rules.isError && isIdentityError(rules.error)) return <IdentityError returnTo={referralPaths.rules} />;
    if (rules.isError || !rules.data) return <ReferralError error={rules.error} />;
    return <ReferralRulesView data={rules.data} />;
  }
  if (view === "earnings") {
    if (earnings.isPending || earnings.isFetching) return <ConsoleState kind="loading" title="正在读取收益事实">正在核对当前身份与服务端收益 projection。</ConsoleState>;
    if (earnings.isError && isIdentityError(earnings.error)) return <IdentityError returnTo={referralPaths.earnings} />;
    if (earnings.isError || !earnings.data) return <ReferralError error={earnings.error} />;
    return <EarningsView data={earnings.data} />;
  }
  if (projection.isPending || projection.isFetching) return <ConsoleState kind="loading" title="正在读取推广事实">正在核对当前身份与持久化关系。</ConsoleState>;
  if (projection.isError && isIdentityError(projection.error)) return <IdentityError returnTo={referralPaths[view]} />;
  if (projection.isError && !receipt) return <ReferralError error={projection.error} />;
  if (projection.isError) return <div className={styles.pageBody}><section className={styles.notice} role="status"><strong>推广关系已确认</strong><p>确认时间：{formatTime(receipt!.boundAt)}</p></section><ConsoleState kind="error" title="推广汇总暂不可用"><p>关系回执已保留，请稍后重试汇总读取。</p></ConsoleState></div>;
  const data = projection.data;
  const effectiveAvailableMinor = data.earnings.availability === "available" ? addMinor(data.earnings.availableMinor, data.earnings.adjustmentMinor) : null;
  const showCenter = view === "overview" || view === "complete" || view === "center";
  const showEarnings = view === "overview" || view === "complete";
  const showWithdrawals = view === "overview" || view === "complete" || view === "withdrawals";
  return <div className={styles.pageBody}>
    {mode === "complete" ? <section className={styles.notice} aria-labelledby="complete-title">
      <h2 id="complete-title">确认新注册关系</h2>
      <p>系统只会按当前 Auth.js 登录 subject 查找同一注册 Intent，并重新核对官方验证事实。</p>
      {receipt ? <div role="status"><strong>推广关系已确认</strong><p>确认时间：{formatTime(receipt.boundAt)}</p></div> : <Button type="button" onClick={() => complete.mutate()} disabled={complete.isPending}>{complete.isPending ? "正在确认…" : "完成推广关系"}</Button>}
      {complete.isError ? <ReferralError error={complete.error} compact /> : null}
    </section> : null}
    {showCenter || showEarnings ? <section className={styles.grid} aria-label="推广概览">
      {showCenter ? <article className={styles.metric}><span>已建立关系</span><strong>{data.count}</strong><small>来自不可变推广关系的实时计数</small></article> : null}
      {showEarnings ? <article className={styles.metric}><span>可提现收益</span><strong className={data.earnings.availability === "available" ? undefined : styles.unavailable}>{effectiveAvailableMinor === null ? "收益数据暂不可用" : formatMinor(effectiveAvailableMinor)}</strong><small>{data.earnings.availability === "available" ? `待结算 ${formatMinor(data.earnings.pendingMinor)} · 冻结 ${formatMinor(data.earnings.reservedMinor)} · 调整 ${formatMinor(data.earnings.adjustmentMinor)}` : "当前没有可读取的收益 projection"}</small></article> : null}
    </section> : null}
    {showWithdrawals ? <section className={styles.codeCard} aria-labelledby="withdrawal-history-title"><div><h2 id="withdrawal-history-title">提现记录</h2>{withdrawalHistory.isPending ? <p>正在读取提现记录…</p> : withdrawalHistory.isError ? <p className={styles.error} role="alert">提现记录暂不可用，请稍后重试。</p> : withdrawalHistory.data?.withdrawals.length === 0 ? <p>暂无提现申请。</p> : <ul>{withdrawalHistory.data?.withdrawals.map((item) => <li key={item.id}>申请 {formatMinor(item.amountMinor)} · {item.status} · {formatTime(item.updatedAt)}</li>)}</ul>}</div></section> : null}
    {showCenter ? <section className={styles.codeCard} aria-labelledby="code-title"><div><h2 id="code-title">我的推广码</h2>{data.codeAvailability === "available" ? <><code>{data.code}</code><p>生成于本次读取：{formatTime(data.generatedAt)}</p></> : <p>尚未创建推广码</p>}</div>{data.codeAvailability === "available" ? registrationAvailable ? <Button asChild variant="outline"><Link href={`/referrals/register?code=${encodeURIComponent(data.code)}`} prefetch={false}>打开邀请链接</Link></Button> : <span className={styles.unavailable}>注册入口暂不可用</span> : <Button type="button" onClick={() => createCode.mutate()} disabled={createCode.isPending}>{createCode.isPending ? "正在创建…" : "创建推广码"}</Button>}</section> : null}
    {showWithdrawals ? <section className={styles.codeCard} aria-labelledby="withdrawal-title"><div><h2 id="withdrawal-title">申请提现</h2><p>最低 ¥100；申请后进入人工审核。结算前提是账户已完成邮箱和手机号验证。</p></div>{data.earnings.availability !== "available" ? <p className={styles.unavailable} role="status">收益数据暂不可用，暂不能读取收款方式或提交提现申请。请刷新后重试。</p> : payoutMethods.isPending ? <p>正在读取已验证收款方式…</p> : payoutMethods.isError ? <p className={styles.error} role="alert">收款方式暂不可用，请稍后重试。</p> : payoutMethods.data?.methods.length === 0 ? <form onSubmit={event => { event.preventDefault(); createPayoutMethod.mutate(); }}><p className={styles.unavailable}>请先登记一个收款方式。</p><label>渠道<select value={payoutType} onChange={event => setPayoutType(event.target.value as "ALIPAY" | "BANK_TRANSFER")}><option value="ALIPAY">支付宝</option><option value="BANK_TRANSFER">银行转账</option></select></label><label>名称<input value={payoutDisplayName} onChange={event => setPayoutDisplayName(event.target.value)} required maxLength={128} /></label><label>账号或收款地址<input value={payoutDestination} onChange={event => setPayoutDestination(event.target.value)} required maxLength={512} /></label><Button type="submit" disabled={createPayoutMethod.isPending}>{createPayoutMethod.isPending ? "登记中…" : "登记收款方式"}</Button>{createPayoutMethod.isError ? <p className={styles.error} role="alert">收款方式登记未完成，请稍后重试。</p> : null}</form> : <form onSubmit={event => { event.preventDefault(); withdrawal.mutate(); }}><label>金额（分）<input inputMode="numeric" pattern="[0-9]*" value={amountMinor} onChange={event => setAmountMinor(event.target.value)} placeholder="10000" disabled={data.earnings.availability !== "available" || withdrawal.isPending} /></label><label>收款方式<select value={selectedPayoutMethod?.methodId ?? ""} onChange={event => setPayoutMethodId(event.target.value)} disabled={withdrawal.isPending}>{payoutMethods.data?.methods.map((item: PayoutMethod) => <option key={item.methodId} value={item.methodId}>{item.displayName} · {item.maskedDestination}</option>)}</select></label><Button type="submit" disabled={data.earnings.availability !== "available" || withdrawal.isPending || amountMinor === "" || !selectedPayoutMethod}>{withdrawal.isPending ? "提交中…" : "申请提现"}</Button>{withdrawal.isError ? <p className={styles.error} role="alert">提现未提交：请确认余额、验证状态和版本仍有效。</p> : null}{cancelTarget ? <div className={styles.notice} role="status"><p>提现状态：{cancelTarget.status}。</p>{cancelTarget.status === "REQUESTED" ? <Button type="button" variant="outline" onClick={() => cancelWithdrawal.mutate()} disabled={cancelWithdrawal.isPending}>{cancelWithdrawal.isPending ? "取消中…" : "取消提现申请"}</Button> : null}{cancelWithdrawal.isError ? <p className={styles.error}>取消提现未完成：请稍后重新核对状态。</p> : null}</div> : null}</form>}</section> : null}
    {createCode.isError ? <ReferralError error={createCode.error} compact /> : null}
    <p className={styles.observed}>数据观察时间：{formatTime(data.generatedAt)}</p>
  </div>;
}

function ReferralRulesView({ data }: { data: ReferralRules }) {
  return <div className={styles.pageBody}><section className={styles.codeCard} aria-labelledby="referral-rules-title"><div><h2 id="referral-rules-title">推广规则</h2><dl><dt>收益来源</dt><dd>仅消费已确认的 canonical settled payment、refund 和 chargeback 事实。</dd><dt>佣金比例</dt><dd>{data.commissionRateBps / 100}%，按人民币最小货币单位计算。</dd><dt>结算周期</dt><dd>支付结算后 {data.settlementPeriodDays} 日进入可用收益。</dd><dt>提现条件</dt><dd>最低 {formatMinor(data.minimumWithdrawalMinor)}，提交后进入人工审核。</dd></dl><p>页面不自行计算或累计收益；当前汇总以服务端不可变账本投影为准。</p></div></section></div>;
}

function EarningsView({ data }: { data: ReferralEarnings }) {
  const effectiveAvailableMinor = addMinor(data.availableMinor, data.adjustmentMinor);
  return <div className={styles.pageBody}>
    <section className={styles.grid} aria-label="收益概览">
      <article className={styles.metric}><span>可用收益</span><strong>{formatMinor(effectiveAvailableMinor)}</strong><small>可用余额已包含退款与拒付调整</small></article>
      <article className={styles.metric}><span>待结算</span><strong>{formatMinor(data.pendingMinor)}</strong><small>等待结算周期完成</small></article>
      <article className={styles.metric}><span>冻结中</span><strong>{formatMinor(data.reservedMinor)}</strong><small>当前被提现流程占用</small></article>
    </section>
    <section className={styles.codeCard} aria-labelledby="earnings-detail-title"><div><h2 id="earnings-detail-title">收益明细</h2><ul><li>可用收益：{formatMinor(data.availableMinor)}</li><li>退款/拒付调整：{formatMinor(data.adjustmentMinor)}</li><li>Projection 版本：{data.version}</li></ul>{data.entries?.length ? <ul aria-label="逐笔收益记录">{data.entries.map(entry => <li key={entry.entryId}>{entry.entryType} · {formatMinor(entry.amountMinor)} · 业务单据 {entry.referenceId} · {formatTime(entry.occurredAt)}</li>)}</ul> : <p>当前没有可展示的逐笔收益记录。</p>}<p>逐笔列表仅展示最新最多 {data.entryLimit} 条记录；汇总以完整收益 projection 为准。</p><p>数据更新时间：{data.updatedAt ? formatTime(data.updatedAt) : "未提供"}</p></div></section>
  </div>;
}

function IdentityError({ returnTo = referralPaths.overview }: { returnTo?: string }) {
  return <ConsoleState kind="error" title="登录身份已变化"><p>旧推广数据已清除。请重新登录后读取当前账户。</p><Button asChild variant="outline"><Link href={`/login?returnTo=${encodeURIComponent(returnTo)}`} prefetch={false}>重新登录</Link></Button></ConsoleState>;
}

function ReferralError({ error, compact = false }: { error: unknown; compact?: boolean }) {
  const code = error instanceof ReferralRequestError || error instanceof AccountReadError ? error.code : "DEPENDENCY_UNAVAILABLE";
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
  return (error instanceof ReferralRequestError || error instanceof AccountReadError) && ["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED", "referral_authentication_required"].includes(error.code);
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

function addMinor(left: string, right: string) {
  return (BigInt(left) + BigInt(right)).toString();
}
