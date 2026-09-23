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
type PayoutMethodInput = { type: "ALIPAY" | "BANK_TRANSFER"; displayName: string; destination: string };
type UnknownReferralWrite =
  | { kind: "payout-method"; input: PayoutMethodInput; idempotencyKey: string }
  | { kind: "withdrawal"; input: { amountMinor: string; payoutMethodId: string; expectedVersion: string }; idempotencyKey: string }
  | { kind: "cancel"; withdrawalId: string; expectedVersion: string; idempotencyKey: string };
type ReferralWithdrawalState = ReferralWithdrawal | Awaited<ReturnType<typeof getReferralWithdrawals>>["withdrawals"][number];
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
    enabled: view === "withdrawals" && projection.data?.earnings.availability === "available",
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
  const [withdrawalResult, setWithdrawalResult] = useState<ReferralWithdrawalState | null>(null);
  const [unknownWrite, setUnknownWrite] = useState<UnknownReferralWrite | null>(null);
  const [reconcilingUnknownWrite, setReconcilingUnknownWrite] = useState(false);
  const payoutMethodAttempt = useRef<Extract<UnknownReferralWrite, { kind: "payout-method" }> | null>(null);
  const withdrawalAttempt = useRef<Extract<UnknownReferralWrite, { kind: "withdrawal" }> | null>(null);
  const cancelAttempt = useRef<Extract<UnknownReferralWrite, { kind: "cancel" }> | null>(null);
  const reconcileUnknownWrite = async () => {
    const pending = unknownWrite;
    if (!pending || reconcilingUnknownWrite) return;
    setReconcilingUnknownWrite(true);
    try {
      if (pending.kind === "payout-method") {
        await createReferralPayoutMethod(expectedUserId, pending.input, pending.idempotencyKey);
        await queryClient.fetchQuery({ queryKey: ["account", "referral-payout-methods", expectedUserId] as const, queryFn: ({ signal }) => getReferralPayoutMethods(expectedUserId, signal), staleTime: 0 });
        createPayoutMethod.reset();
        payoutMethodAttempt.current = null;
        setUnknownWrite(null);
        return;
      }
      if (pending.kind === "withdrawal") {
        const result = await requestReferralWithdrawal(expectedUserId, pending.input, pending.idempotencyKey);
        const [, withdrawals] = await Promise.all([
          queryClient.fetchQuery({ queryKey, queryFn: ({ signal }) => getAccountReferrals(expectedUserId, signal), staleTime: 0 }),
          queryClient.fetchQuery({ queryKey: ["account", "referral-withdrawals", expectedUserId] as const, queryFn: ({ signal }) => getReferralWithdrawals(expectedUserId, signal), staleTime: 0 }),
        ]);
        withdrawal.reset();
        withdrawalAttempt.current = null;
        setWithdrawalResult(withdrawals.withdrawals.find((item) => item.id === result.id) ?? result);
        setUnknownWrite(null);
        return;
      }
      const result = await cancelReferralWithdrawal(expectedUserId, pending.withdrawalId, pending.expectedVersion, pending.idempotencyKey);
      const [, withdrawals] = await Promise.all([
        queryClient.fetchQuery({ queryKey, queryFn: ({ signal }) => getAccountReferrals(expectedUserId, signal), staleTime: 0 }),
        queryClient.fetchQuery({ queryKey: ["account", "referral-withdrawals", expectedUserId] as const, queryFn: ({ signal }) => getReferralWithdrawals(expectedUserId, signal), staleTime: 0 }),
      ]);
      if (pending.kind === "cancel") {
        cancelWithdrawal.reset();
        cancelAttempt.current = null;
        setWithdrawalResult(withdrawals.withdrawals.find((item) => item.id === pending.withdrawalId) ?? result);
      }
      setUnknownWrite(null);
    } catch {
      // Keep the lock when reconciliation itself fails; the provider fact is still unknown.
    } finally {
      setReconcilingUnknownWrite(false);
    }
  };
  const selectedPayoutMethod = payoutMethods.data?.methods.find((item) => item.methodId === payoutMethodId) ?? payoutMethods.data?.methods[0];
  const withdrawal = useMutation({
    mutationFn: () => {
      const currentEarnings = projection.data?.earnings;
      if (!currentEarnings || currentEarnings.availability !== "available") throw new Error("earnings unavailable");
      if (!selectedPayoutMethod) throw new Error("payout method unavailable");
      const input = { amountMinor, payoutMethodId: selectedPayoutMethod.methodId, expectedVersion: currentEarnings.version };
      const idempotencyKey = crypto.randomUUID();
      withdrawalAttempt.current = { kind: "withdrawal", input, idempotencyKey };
      return requestReferralWithdrawal(expectedUserId, input, idempotencyKey);
    },
    onSuccess: (result) => { withdrawalAttempt.current = null; setUnknownWrite(null); setWithdrawalResult(result); setAmountMinor(""); void queryClient.invalidateQueries({ queryKey }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-payout-methods", expectedUserId] }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-withdrawals", expectedUserId] }); },
    onError: (error) => { const attempt = withdrawalAttempt.current; if (error instanceof AccountReadError && error.outcome === "unknown" && attempt) setUnknownWrite(attempt); else withdrawalAttempt.current = null; },
  });
  const cancelTarget = withdrawalResult ?? withdrawalHistory.data?.withdrawals.find((item) => item.status === "REQUESTED");
  const createPayoutMethod = useMutation({
    mutationFn: () => { const input = { type: payoutType, displayName: payoutDisplayName, destination: payoutDestination }; const idempotencyKey = crypto.randomUUID(); payoutMethodAttempt.current = { kind: "payout-method", input, idempotencyKey }; return createReferralPayoutMethod(expectedUserId, input, idempotencyKey); },
    onSuccess: () => { payoutMethodAttempt.current = null; setUnknownWrite(null); setPayoutDisplayName(""); setPayoutDestination(""); void queryClient.invalidateQueries({ queryKey: ["account", "referral-payout-methods", expectedUserId] }); },
    onError: (error) => { const attempt = payoutMethodAttempt.current; if (error instanceof AccountReadError && error.outcome === "unknown" && attempt) setUnknownWrite(attempt); else payoutMethodAttempt.current = null; },
  });
  const cancelWithdrawal = useMutation({
    mutationFn: () => {
      if (!cancelTarget || cancelTarget.status !== "REQUESTED") throw new Error("withdrawal not cancelable");
      const idempotencyKey = crypto.randomUUID();
      cancelAttempt.current = { kind: "cancel", withdrawalId: cancelTarget.id, expectedVersion: cancelTarget.version, idempotencyKey };
      return cancelReferralWithdrawal(expectedUserId, cancelTarget.id, cancelTarget.version, idempotencyKey);
    },
    onSuccess: (result) => { cancelAttempt.current = null; setUnknownWrite(null); setWithdrawalResult(result); void queryClient.invalidateQueries({ queryKey }); void queryClient.invalidateQueries({ queryKey: ["account", "referral-withdrawals", expectedUserId] }); },
    onError: (error) => { const attempt = cancelAttempt.current; if (error instanceof AccountReadError && error.outcome === "unknown" && attempt) setUnknownWrite(attempt); else cancelAttempt.current = null; },
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
  const showOverview = view === "overview" || view === "complete";
  const showCenter = view === "center";
  const showWithdrawals = view === "withdrawals";
  return <div className={styles.pageBody}>
    {mode === "complete" ? <section className={styles.notice} aria-labelledby="complete-title">
      <h2 id="complete-title">确认新注册关系</h2>
      <p>系统只会按当前 Auth.js 登录 subject 查找同一注册 Intent，并重新核对官方验证事实。</p>
      {receipt ? <div role="status"><strong>推广关系已确认</strong><p>确认时间：{formatTime(receipt.boundAt)}</p></div> : <Button type="button" onClick={() => complete.mutate()} disabled={complete.isPending}>{complete.isPending ? "正在确认…" : "完成推广关系"}</Button>}
      {complete.isError ? <ReferralError error={complete.error} compact /> : null}
    </section> : null}
    {showOverview || showCenter || showWithdrawals ? <section className={styles.grid} aria-label={showCenter ? "推广中心摘要" : showWithdrawals ? "提现概览" : "推广与收益概览"}>
      {showCenter || showOverview ? <article className={styles.metric}><span>已建立推广关系</span><strong>{data.count}</strong><small>来源：当前账户的推广关系 owner</small></article> : null}
      {showCenter ? <article className={styles.metric}><span>有效注册</span><strong className={styles.unavailable}>未提供</strong><small>当前 owner 未返回访问或注册转化数</small></article> : null}
      {showCenter ? <article className={styles.metric}><span>付费用户</span><strong className={styles.unavailable}>未提供</strong><small>当前 owner 未返回付费用户统计</small></article> : null}
      {showOverview || showWithdrawals ? <article className={styles.metric}><span>可提现收益</span><strong className={data.earnings.availability === "available" ? undefined : styles.unavailable}>{effectiveAvailableMinor === null ? "暂不可用" : formatMinor(effectiveAvailableMinor)}</strong><small>{data.earnings.availability === "available" ? "已结算收益扣除退款 / 拒付调整" : "当前没有可读取的收益 projection"}</small></article> : null}
      {showOverview || showCenter ? <article className={styles.metric}><span>待结算收益</span><strong className={data.earnings.availability === "available" ? undefined : styles.unavailable}>{data.earnings.availability === "available" ? formatMinor(data.earnings.pendingMinor) : "暂不可用"}</strong><small>{data.earnings.availability === "available" ? "来源：当前收益 projection" : "当前 owner 未返回收益事实"}</small></article> : null}
      {showWithdrawals ? <article className={styles.metric}><span>已预留提现金额</span><strong className={data.earnings.availability === "available" ? undefined : styles.unavailable}>{data.earnings.availability === "available" ? formatMinor(data.earnings.reservedMinor) : "暂不可用"}</strong><small>来源：当前收益 projection</small></article> : null}
    </section> : null}
    {showOverview ? <section className={styles.entryGrid} aria-label="推广与收益管理">
      {([["推广中心", "推广码与可用关系数据", referralPaths.center], ["收益明细", "来自权威支付事实的收益 projection", referralPaths.earnings], ["提现管理", "收款方式与人工提现申请", referralPaths.withdrawals], ["推广规则", "当前有效的收益与结算规则", referralPaths.rules]] as const).map(([title, description, href]) => <article className={styles.entryCard} key={href}><div><h2>{title}</h2><p>{description}</p></div><Button asChild variant="outline"><Link href={href} prefetch={false}>查看详情</Link></Button></article>)}
      <article className={styles.entryCard}><div><h2>近期收益动态</h2><p>当前账户页面没有独立收益动态汇总；逐笔事实见收益明细。</p></div><Button asChild variant="outline"><Link href={referralPaths.earnings} prefetch={false}>查看收益</Link></Button></article>
      <article className={styles.entryCard}><div><h2>提现状态</h2>{withdrawalHistory.isPending ? <p>正在读取提现记录…</p> : withdrawalHistory.isError ? <p role="alert">提现记录暂不可用，请稍后重试。</p> : withdrawalHistory.data?.withdrawals.length === 0 ? <p>暂无提现申请。</p> : <ul>{withdrawalHistory.data?.withdrawals.slice(0, 3).map(item => <li key={item.id}>{formatMinor(item.amountMinor)} · {item.status} · {formatTime(item.updatedAt)}</li>)}</ul>}</div><Button asChild variant="outline"><Link href={referralPaths.withdrawals} prefetch={false}>管理提现</Link></Button></article>
    </section> : null}
    {showCenter ? <>
      <section className={styles.codeCard} aria-labelledby="code-title"><div><h2 id="code-title">我的推广</h2><p>好友通过推广链接或推广码完成注册后，关系以系统最终记录为准。</p>{data.codeAvailability === "available" ? <><label>推广链接<code>{`/referrals/register?code=${encodeURIComponent(data.code)}`}</code></label><label>推广码<code>{data.code}</code></label><p>生成于本次读取：{formatTime(data.generatedAt)}</p></> : <p>尚未创建推广码。</p>}</div>{data.codeAvailability === "available" ? registrationAvailable ? <ShareReferralLink code={data.code} /> : <span className={styles.unavailable}>注册入口暂不可用</span> : <Button type="button" onClick={() => createCode.mutate()} disabled={createCode.isPending}>{createCode.isPending ? "正在创建…" : "创建推广码"}</Button>}</section>
      <section className={styles.codeCard} aria-labelledby="conversion-title"><div><h2 id="conversion-title">推广转化</h2><p>访问量、有效注册、付费用户及收益动态暂未由当前 owner 提供，页面不显示未核实的统计。</p></div><span className={styles.unavailable}>转化统计暂不可用</span></section>
    </> : null}
    {showWithdrawals ? <>
      <section className={styles.codeCard} aria-labelledby="withdrawal-history-title"><div><h2 id="withdrawal-history-title">提现记录</h2>{withdrawalHistory.isPending ? <p>正在读取提现记录…</p> : withdrawalHistory.isError ? <p className={styles.error} role="alert">提现记录暂不可用，请稍后重试。</p> : withdrawalHistory.data?.withdrawals.length === 0 ? <p>暂无提现申请。</p> : <ul>{withdrawalHistory.data?.withdrawals.map((item) => <li key={item.id}>申请 {formatMinor(item.amountMinor)} · {item.status} · {formatTime(item.updatedAt)}</li>)}</ul>}</div></section>
      <section className={styles.codeCard} aria-labelledby="withdrawal-title"><div><h2 id="withdrawal-title">申请提现</h2><p>最低 ¥100；申请后进入人工审核。结算前提是账户已完成邮箱和手机号验证。</p></div>{unknownWrite ? <div className={styles.notice} role="alert"><strong>操作结果待核实</strong><p>提现或收款方式操作可能已经提交，不能生成新请求。刷新会使用原幂等键核对并恢复结果。</p><Button type="button" variant="outline" onClick={() => void reconcileUnknownWrite()} disabled={reconcilingUnknownWrite}>{reconcilingUnknownWrite ? "正在刷新提现状态…" : "刷新提现状态"}</Button></div> : null}{data.earnings.availability !== "available" ? <p className={styles.unavailable} role="status">收益数据暂不可用，暂不能读取收款方式或提交提现申请。请刷新后重试。</p> : payoutMethods.isPending ? <p>正在读取已验证收款方式…</p> : payoutMethods.isError ? <p className={styles.error} role="alert">收款方式暂不可用，请稍后重试。</p> : payoutMethods.data?.methods.length === 0 ? <form onSubmit={event => { event.preventDefault(); createPayoutMethod.mutate(); }}><p className={styles.unavailable}>请先登记一个收款方式。</p><label>渠道<select value={payoutType} onChange={event => setPayoutType(event.target.value as "ALIPAY" | "BANK_TRANSFER")} disabled={unknownWrite !== null}><option value="ALIPAY">支付宝</option><option value="BANK_TRANSFER">银行转账</option></select></label><label>名称<input value={payoutDisplayName} onChange={event => setPayoutDisplayName(event.target.value)} required maxLength={128} disabled={unknownWrite !== null} /></label><label>账号或收款地址<input value={payoutDestination} onChange={event => setPayoutDestination(event.target.value)} required maxLength={512} disabled={unknownWrite !== null} /></label><Button type="submit" disabled={createPayoutMethod.isPending || unknownWrite !== null}>{createPayoutMethod.isPending ? "登记中…" : "登记收款方式"}</Button>{createPayoutMethod.isError ? <ReferralError error={createPayoutMethod.error} compact /> : null}</form> : <form onSubmit={event => { event.preventDefault(); withdrawal.mutate(); }}><label>金额（分）<input inputMode="numeric" pattern="[0-9]*" value={amountMinor} onChange={event => setAmountMinor(event.target.value)} placeholder="10000" disabled={data.earnings.availability !== "available" || withdrawal.isPending || unknownWrite !== null} /></label><label>收款方式<select value={selectedPayoutMethod?.methodId ?? ""} onChange={event => setPayoutMethodId(event.target.value)} disabled={withdrawal.isPending || unknownWrite !== null}>{payoutMethods.data?.methods.map((item: PayoutMethod) => <option key={item.methodId} value={item.methodId}>{item.displayName} · {item.maskedDestination}</option>)}</select></label><Button type="submit" disabled={data.earnings.availability !== "available" || withdrawal.isPending || unknownWrite !== null || amountMinor === "" || !selectedPayoutMethod}>{withdrawal.isPending ? "提交中…" : "申请提现"}</Button>{withdrawal.isError ? <ReferralError error={withdrawal.error} compact /> : null}{cancelTarget ? <div className={styles.notice} role="status"><p>提现状态：{cancelTarget.status}。</p>{cancelTarget.status === "REQUESTED" ? <Button type="button" variant="outline" onClick={() => cancelWithdrawal.mutate()} disabled={cancelWithdrawal.isPending || unknownWrite !== null}>{cancelWithdrawal.isPending ? "取消中…" : "取消提现申请"}</Button> : null}{cancelWithdrawal.isError ? <ReferralError error={cancelWithdrawal.error} compact /> : null}</div> : null}</form>}</section>
    </> : null}
    {createCode.isError ? <ReferralError error={createCode.error} compact /> : null}
    <p className={styles.observed}>数据观察时间：{formatTime(data.generatedAt)}</p>
  </div>;
}

function ReferralRulesView({ data }: { data: ReferralRules }) {
  return <div className={styles.pageBody}>
    <section className={styles.ruleNotice}><strong>当前规则以平台实际生效版本为准</strong><p>规则由当前推广 owner 返回。页面不自行计算或累计收益；汇总以服务端不可变账本投影为准。</p></section>
    <div className={styles.ruleGrid}>
      <article className={styles.ruleCard}><h2>推广关系如何建立</h2><p>用户通过推广链接或推广码完成注册并通过官方验证后，关系以系统最终记录为准。</p></article>
      <article className={styles.ruleCard}><h2>哪些订单产生收益</h2><p>收益只消费 canonical settled payment、refund 和 chargeback 事实；本页面不会创建支付事实。</p></article>
      <article className={styles.ruleCard}><h2>收益如何计算</h2><p>当前有效比例为 {data.commissionRateBps / 100}%；最终金额以不可变收益账本投影为准。</p></article>
      <article className={styles.ruleCard}><h2>结算与退款处理</h2><p>结算周期为 {data.settlementPeriodDays} 日。退款与拒付通过 owner 记录的调整进入收益投影。</p></article>
      <article className={styles.ruleCard}><h2>提现规则</h2><p>最低申请金额 {formatMinor(data.minimumWithdrawalMinor)}；申请经人工审核，外部打款渠道由平台处理。</p></article>
      <article className={styles.ruleCard}><h2>违规推广处理</h2><p>推广关系、订单和收益异常由平台规则处理；当前页面不提供审批或风控操作。</p></article>
    </div>
  </div>;
}

function EarningsView({ data }: { data: ReferralEarnings }) {
  const effectiveAvailableMinor = addMinor(data.availableMinor, data.adjustmentMinor);
  return <div className={styles.pageBody}>
    <section className={styles.earningsGrid} aria-label="收益概览">
      <article className={styles.metric}><span>可用收益</span><strong>{formatMinor(effectiveAvailableMinor)}</strong><small>可用余额已包含退款与拒付调整</small></article>
      <article className={styles.metric}><span>待结算</span><strong>{formatMinor(data.pendingMinor)}</strong><small>等待结算周期完成</small></article>
      <article className={styles.metric}><span>冻结中</span><strong>{formatMinor(data.reservedMinor)}</strong><small>当前被提现流程占用</small></article>
      <article className={styles.metric}><span>退款 / 拒付调整</span><strong>{formatMinor(data.adjustmentMinor)}</strong><small>来源：当前收益 projection</small></article>
    </section>
    <section className={styles.codeCard} aria-labelledby="earnings-detail-title"><div><h2 id="earnings-detail-title">收益明细</h2><p>Projection 版本：{data.version} · 更新时间：{data.updatedAt ? formatTime(data.updatedAt) : "未提供"}</p>{data.entries?.length ? <div className={styles.earningsTableWrap} role="region" aria-label="逐笔收益记录，可横向滚动" tabIndex={0}><table className={styles.earningsTable}><thead><tr><th>时间</th><th>来源</th><th>金额</th><th>业务单据</th></tr></thead><tbody>{data.entries.map(entry => <tr key={entry.entryId}><td>{formatTime(entry.occurredAt)}</td><td>{entry.entryType}</td><td>{formatMinor(entry.amountMinor)}</td><td>{entry.referenceId}</td></tr>)}</tbody></table></div> : <p>当前没有可展示的逐笔收益记录。</p>}<p>逐笔列表仅展示最新最多 {data.entryLimit} 条记录；汇总以完整收益 projection 为准。</p></div></section>
  </div>;
}

function ShareReferralLink({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(new URL(`/referrals/register?code=${encodeURIComponent(code)}`, window.location.origin).toString());
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return <div className={styles.shareActions}><Button type="button" variant="outline" onClick={() => void copy()}>{copied ? "已复制" : "复制链接"}</Button><span role="status">{copied ? "推广链接已复制到剪贴板。" : "二维码暂不可用"}</span></div>;
}

function IdentityError({ returnTo = referralPaths.overview }: { returnTo?: string }) {
  return <ConsoleState kind="error" title="登录身份已变化"><p>旧推广数据已清除。请重新登录后读取当前账户。</p><Button asChild variant="outline"><Link href={`/login?returnTo=${encodeURIComponent(returnTo)}`} prefetch={false}>重新登录</Link></Button></ConsoleState>;
}

function ReferralError({ error, compact = false }: { error: unknown; compact?: boolean }) {
  const code = error instanceof ReferralRequestError || error instanceof AccountReadError ? error.code : "DEPENDENCY_UNAVAILABLE";
  const title: Record<string, string> = {
    AUTHENTICATION_REQUIRED: "登录已失效", IDENTITY_CONTEXT_CHANGED: "登录身份已变化", RESULT_UNVERIFIED: "操作结果待核实",
    referral_authentication_required: "登录已失效", referral_verification_pending: "官方邮箱或认证方式尚未完成",
    referral_expired: "注册确认期限已结束", referral_conflict: "推广关系存在冲突",
    referral_outcome_unknown: "暂时无法确认操作结果", REFERRALS_NOT_CONFIGURED: "推广服务尚未配置",
    CONFLICT: "提现状态已变化", PAYOUT_ELIGIBILITY_UNMET: "未满足提现条件", INVALID_TRANSITION: "提现状态不允许该操作", DEPENDENCY_UNAVAILABLE: "推广服务暂不可用", DEADLINE_EXCEEDED: "推广请求超时",
  };
  const unknownOutcome = ["referral_outcome_unknown", "RESULT_UNVERIFIED", "DEADLINE_EXCEEDED"].includes(code) || error instanceof AccountReadError && error.outcome === "unknown";
  const definiteRejection = ["CONFLICT", "PAYOUT_ELIGIBILITY_UNMET", "INVALID_TRANSITION"].includes(code);
  const message = unknownOutcome ||
    compact && ["DEPENDENCY_UNAVAILABLE", "INVALID_UPSTREAM_RESPONSE"].includes(code)
    ? "操作结果尚未确认。请先刷新相关状态核对服务端事实，再决定是否重试。"
    : definiteRejection ? "服务端已拒绝本次操作；请刷新提现状态后重新核对版本、资格或当前状态。"
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
