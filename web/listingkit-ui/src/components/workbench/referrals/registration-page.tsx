"use client";

import Link from "next/link";
import { FormEvent, useRef, useState, useSyncExternalStore } from "react";

import { Button } from "@/components/ui/button";
import {
  ReferralRequestError,
  resumeReferralRegistration,
  startReferralRegistration,
  type ReferralAdmission,
  type ReferralRegistrationInput,
} from "@/lib/api/referrals";
import styles from "./referrals.module.css";

const completionPath = "/workbench/account/referrals/complete";
const loginHref = `/login?returnTo=${encodeURIComponent(completionPath)}`;

export function RegistrationPage({ code }: { code: string }) {
  const validCode = isBounded(code, 1, 200);
  const [admission, setAdmission] = useState<ReferralAdmission | null>(null);
  const [created, setCreated] = useState(false);
  const fragment = useSyncExternalStore(subscribeHash, () => window.location.hash, () => "");
  const recovery = readRecoveryFragment(fragment);
  const activeRecovery = admission
    ? { intentID: admission.intentID, resumeSecret: admission.resumeSecret }
    : recovery;
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const attempt = useRef<{ input: ReferralRegistrationInput; key: string } | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!validCode || pending) return;
    const data = new FormData(event.currentTarget);
    const input: ReferralRegistrationInput = {
      code,
      email: String(data.get("email") ?? "").trim(),
      givenName: String(data.get("givenName") ?? "").trim(),
      familyName: String(data.get("familyName") ?? "").trim(),
    };
    if (!attempt.current) attempt.current = { input, key: randomKey() };
    setPending(true);
    setError("");
    try {
      const result = await startReferralRegistration(attempt.current.input, attempt.current.key);
      setAdmission(result);
      const fragment = new URLSearchParams({ intentID: result.intentID, resumeSecret: result.resumeSecret });
      window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}#${fragment.toString()}`);
      await resumeReferralRegistration(result.intentID, result.resumeSecret);
      setCreated(true);
    } catch (cause) {
      setError(cause instanceof ReferralRequestError ? cause.code : "DEPENDENCY_UNAVAILABLE");
    } finally {
      setPending(false);
    }
  }

  async function resume() {
    if (!activeRecovery || pending) return;
    setPending(true);
    setError("");
    try {
      await resumeReferralRegistration(activeRecovery.intentID, activeRecovery.resumeSecret);
      setCreated(true);
    } catch (cause) {
      setError(cause instanceof ReferralRequestError ? cause.code : "DEPENDENCY_UNAVAILABLE");
    } finally {
      setPending(false);
    }
  }

  if (!validCode) {
    return <RegistrationFrame><div className={styles.notice} role="alert"><h2>邀请链接无效或已缺少邀请码</h2><p>请向邀请人重新获取完整链接。</p></div></RegistrationFrame>;
  }

  if (created) {
    return <RegistrationFrame><div className={styles.notice} role="status">
      <h2>请查看官方验证邮件</h2>
      <p>邮件验证和首次认证方式均由官方登录页完成。本页面不会收集密码或验证码。</p>
      <Button asChild><Link href={loginHref} prefetch={false}>已完成验证，继续登录</Link></Button>
    </div></RegistrationFrame>;
  }

  if (activeRecovery) {
    return <RegistrationFrame><section className={styles.recovery} aria-labelledby="recovery-title">
      <h2 id="recovery-title">继续原注册</h2>
      <p>此浏览器保留了原流程的恢复片段。恢复只核对原 Intent，不会创建新的身份。</p>
      {error ? <div className={styles.error} role="alert"><strong>暂时无法确认注册结果</strong><p>请重试原请求；系统会沿用同一请求标识核对结果。</p></div> : null}
      <Button type="button" variant="outline" onClick={resume} disabled={pending}>{pending ? "正在恢复…" : "恢复原注册"}</Button>
    </section></RegistrationFrame>;
  }

  return <RegistrationFrame>
    <form className={styles.form} onSubmit={submit}>
      <label>邀请码<input name="code" value={code} readOnly /></label>
      <label>邮箱<input name="email" type="email" autoComplete="email" required maxLength={254} readOnly={Boolean(attempt.current)} /></label>
      <div className={styles.nameGrid}>
        <label>名字<input name="givenName" autoComplete="given-name" required maxLength={120} readOnly={Boolean(attempt.current)} /></label>
        <label>姓氏<input name="familyName" autoComplete="family-name" required maxLength={120} readOnly={Boolean(attempt.current)} /></label>
      </div>
      {error ? <div className={styles.error} role="alert"><strong>{error === "referral_outcome_unknown" || error === "DEPENDENCY_UNAVAILABLE" ? "暂时无法确认注册结果" : error === "referral_expired" ? "注册确认期限已结束" : error === "referral_conflict" ? "注册请求存在冲突" : "注册请求未完成"}</strong><p>{error === "referral_conflict" ? "该邮箱或请求与现有流程冲突，不能改绑到其他身份。" : error === "referral_expired" ? "原注册流程已过期，不能继续提交或改换身份。" : "请重试原请求；系统会沿用同一请求标识核对结果。"}</p></div> : null}
      <Button type="submit" disabled={pending}>{pending ? "正在提交…" : error ? "重试原请求" : "开始注册"}</Button>
    </form>
  </RegistrationFrame>;
}

function RegistrationFrame({ children }: { children: React.ReactNode }) {
  return <main className={styles.registration}><div className={styles.registrationCard}><p className={styles.eyebrow}>ListingKit 推荐计划</p><h1>接受好友邀请</h1><p className={styles.lead}>创建新的官方账户，完成邮箱验证与登录后建立推广关系。</p>{children}</div></main>;
}

function isBounded(value: string, min: number, max: number) {
  const bytes = new TextEncoder().encode(value).byteLength;
  return value === value.trim() && bytes >= min && bytes <= max && !/[\u0000-\u001f\u007f]/u.test(value);
}

function randomKey() {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}

function subscribeHash(onChange: () => void) {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

function readRecoveryFragment(raw: string) {
  const fragment = new URLSearchParams(raw.slice(1));
  const intentID = fragment.get("intentID") ?? "";
  const resumeSecret = fragment.get("resumeSecret") ?? "";
  return /^[A-Za-z0-9._:-]{1,200}$/.test(intentID) && /^[A-Za-z0-9_-]{43,256}$/.test(resumeSecret)
    ? { intentID, resumeSecret }
    : null;
}
