"use client";

import Image from "next/image";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getAccountAudit } from "@/lib/api/account-audit";
import { getMemberTokenAllocations } from "@/lib/api/account-allocation";
import { AccountReadError, updateAccountBusinessProfile, type AccountBusinessProfile, type AccountBusinessProfileInput, type AccountOrganization, type AccountProfile } from "@/lib/api/account";
import { getAccountIdentityProfile, resendAccountEmailVerification, resendAccountPhoneVerification, setAccountEmail, setAccountPhone, updateAccountIdentityProfile, updateAccountPassword, verifyAccountEmail, verifyAccountPhone, type AccountIdentityProfileInput } from "@/lib/api/account-identity";
import { getCommercialOverview } from "@/lib/api/commercial";
import { getMembers } from "@/lib/api/members";
import styles from "./account.module.css";
import { ConsoleState } from "../console/console-page";

const provided = (value: string | null) => value?.trim() || "未提供";
const verification = (value: boolean | null) => value === true ? "已验证" : value === false ? "未验证" : "未提供";
function Panel({ title, description, children, className = "" }: { title: string; description?: string; children: ReactNode; className?: string }) {
  return <Card className={`${styles.panel} ${className}`}><header><h2>{title}</h2>{description ? <p>{description}</p> : null}</header>{children}</Card>;
}
function Fields({ items }: { items: readonly (readonly [string, ReactNode])[] }) {
  return <dl className={styles.fields}>{items.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>;
}
function Provenance({ data }: { data: AccountProfile | AccountOrganization }) {
  return <details className={styles.provenance}><summary>资料来源与读取时间</summary><p>来源：{data.source === "zitadel_userinfo" ? "ZITADEL 账户资料" : "ZITADEL 项目授权"}</p><p>读取时间：<time dateTime={data.readAt}>{data.readAt}</time></p>{"authorizationMaxAgeSeconds" in data ? <p>授权缓存最长 60 秒；读取时间不代表授权刷新时间。</p> : null}</details>;
}

export function AccountOverviewView({ profile, organization }: { profile: AccountProfile; business: AccountBusinessProfile | null; organization: AccountOrganization }) {
  const verified = profile.emailVerified === true || profile.phoneNumberVerified === true;
  return <><Card className={styles.overview}><div><h2>账户概览</h2><p>查看当前账户、企业与收益状态</p></div><div className={styles.overviewMetric}><span>账号状态</span><strong>{verified ? "已验证" : "待完善"}</strong><small>{profile.emailVerified === true && profile.phoneNumberVerified === true ? "手机 / 邮箱已验证" : "验证状态以登录服务为准"}</small></div><div className={styles.overviewMetric}><span>企业状态</span><strong>当前授权有效</strong><small>{organization.roles.length ? organization.roles.join("、") : "未提供角色"}</small></div><div className={styles.overviewMetric}><span>推广收益</span><strong className={styles.unavailableValue}>进入推广与收益查看</strong><small>收益以不可变 ledger projection 为准</small></div></Card><div className={styles.overviewLinks}><Panel title="账户资料" description="身份、联系方式与经营画像"><Button asChild variant="outline"><Link href="/workbench/account/profile" prefetch={false}>查看账户资料</Link></Button></Panel><Panel title="企业空间" description={organization.name?.trim() || "当前有效企业"}><Button asChild variant="outline"><Link href="/workbench/account/organization" prefetch={false}>查看企业空间</Link></Button></Panel><Panel title="推广与收益" description="推广关系、收益账本与人工提现"><Button asChild variant="outline"><Link href="/workbench/account/referrals" prefetch={false}>查看推广信息</Link></Button></Panel></div><Provenance data={profile} /></>;
}

type ProfileSection = "summary" | "settings" | "business" | "verification";

export function ProfileView({ data, business, organization, organizationId, section = "summary", identityVerificationOutcomeUnknown = false, onIdentityVerificationOutcomeUnknown, identityContactOutcomeUnknown = false, onIdentityContactOutcomeUnknown }: { data: AccountProfile; business?: AccountBusinessProfile | null; organization?: AccountOrganization | null; organizationId?: string; section?: ProfileSection; identityVerificationOutcomeUnknown?: boolean; onIdentityVerificationOutcomeUnknown?: () => void; identityContactOutcomeUnknown?: boolean; onIdentityContactOutcomeUnknown?: () => void }) {
  const identity = <ProfileIdentity data={data} />;
  if (section === "settings") return <section className={styles.settingsWorkspace} aria-label="账户资料设置"><AccountSettingsPanel data={data} editable returnTo="/workbench/account/profile/settings" identityContactOutcomeUnknown={identityContactOutcomeUnknown} onIdentityContactOutcomeUnknown={onIdentityContactOutcomeUnknown} /><Provenance data={data} /></section>;
  if (section === "business") return <>{identity}<BusinessProfilePanel profile={business} organizationId={organizationId} /><Provenance data={data} /></>;
  if (section === "verification") return <>{identity}<VerificationPanel data={data} interactive organization={organization} returnTo="/workbench/account/profile/verification" identityVerificationOutcomeUnknown={identityVerificationOutcomeUnknown} onIdentityVerificationOutcomeUnknown={onIdentityVerificationOutcomeUnknown} /><Provenance data={data} /></>;
  return <>{identity}<Card className={styles.profileStatusMetric}><span>账户状态</span><strong>{data.emailVerified === true && data.phoneNumberVerified === true ? "已验证" : "待完善"}</strong><small>{data.emailVerified === true && data.phoneNumberVerified === true ? "邮箱与手机均由身份服务确认" : "验证状态以身份服务返回为准"}</small></Card>
    {![data.displayName, data.email, data.phoneNumber].some(value => value?.trim()) ? <ConsoleState kind="empty" title="暂未提供个人资料">登录服务本次未提供显示名称、手机号码或电子邮箱。你仍可查看账户标识并刷新资料。</ConsoleState> : null}
    <div className={styles.profileGrid}><div className={styles.profileMain}>
      <AccountSettingsPanel data={data} />
      <VerificationPanel data={data} />
    </div><BusinessProfilePanel profile={business} organizationId={organizationId} /></div><Provenance data={data} /></>;
}

function ProfileIdentity({ data }: { data: AccountProfile }) {
  return <Card className={styles.identity}><div><h2>{data.displayName?.trim() || "当前账户"}</h2><p>账户 ID：{data.userId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div><div className={styles.identityStatus}><span className={styles.badge}>账户资料 · 当前用户</span><span className={styles.identityMeta}>注册时间：未提供 · 最近登录：未提供</span></div></Card>;
}

function AccountSettingsPanel({ data, editable = false, returnTo, identityContactOutcomeUnknown = false, onIdentityContactOutcomeUnknown }: { data: AccountProfile; editable?: boolean; returnTo?: string; identityContactOutcomeUnknown?: boolean; onIdentityContactOutcomeUnknown?: () => void }) {
  if (editable) return <>
    <Panel title="账户信息" description="个人身份资料由 ZITADEL 官方 Auth API 管理" className={styles.settingsAccount}>
      <Fields items={[["显示名称", provided(data.displayName)], ["国家 / 地区", "未提供"], ["省 / 州", "未提供"], ["城市", "未提供"]]} />
      <IdentityProfileManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/settings"} />
    </Panel>
    <IdentityContactManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/settings"} identityContactOutcomeUnknown={identityContactOutcomeUnknown} onIdentityContactOutcomeUnknown={onIdentityContactOutcomeUnknown} />
  </>;
  return <div className={styles.settingsSections}>
    <Panel title="账户设置" description="个人身份资料由 ZITADEL 官方 Auth API 管理">
      <Fields items={[["显示名称", provided(data.displayName)], ["国家 / 地区", "未提供"], ["省 / 州", "未提供"], ["城市", "未提供"]]} />
      <p className={styles.note}>进入账户设置后，可在 Shuomi 内修改个人资料；字段由当前登录身份服务提供。</p>
    </Panel>
    <Panel title="联系方式"><Fields items={[["手机号码", <>{provided(data.phoneNumber)} · {verification(data.phoneNumberVerified)}</>], ["邮箱地址", <>{provided(data.email)} · {verification(data.emailVerified)}</>]]} /><p className={styles.note}>修改和验证由 ZITADEL 官方流程完成。</p></Panel>
  </div>;
}

function IdentityProfileManagement({ data, returnTo }: { data: AccountProfile; returnTo: string }) {
  const profileQuery = useQuery({ queryKey: ["account", "identity-profile", data.userId], queryFn: () => getAccountIdentityProfile({ expectedUserId: data.userId }), retry: false });
  if (profileQuery.isPending) return <p className={styles.note}>正在读取 ZITADEL 个人资料…</p>;
  if (profileQuery.isError || !profileQuery.data) return <><p className={styles.errorText}>个人资料暂时无法读取，请稍后重试。</p>{profileQuery.isError && identityMutationNeedsLogin(profileQuery.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</>;
  const profileKey = [profileQuery.data.userId, profileQuery.data.firstName, profileQuery.data.lastName, profileQuery.data.nickName, profileQuery.data.displayName, profileQuery.data.preferredLanguage, profileQuery.data.gender].join("\u001f");
  return <IdentityProfileForm key={profileKey} data={data} profile={profileQuery.data} returnTo={returnTo} />;
}

function IdentityProfileForm({ data, profile, returnTo }: { data: AccountProfile; profile: { firstName: string; lastName: string; nickName: string; displayName: string; preferredLanguage: string; gender: string }; returnTo: string }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<AccountIdentityProfileInput>(() => identityProfileInput(profile));
  const mutation = useMutation({ mutationFn: (input: AccountIdentityProfileInput) => updateAccountIdentityProfile({ expectedUserId: data.userId, input }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const field = (key: keyof AccountIdentityProfileInput, label: string) => <label key={key}>{label}<input value={form[key]} onChange={event => setForm(current => current ? { ...current, [key]: event.target.value } : current)} disabled={mutation.isPending} /></label>;
  return <div className={styles.profileForm}><h3>个人资料</h3><form onSubmit={event => { event.preventDefault(); mutation.mutate(form); }}>{field("firstName", "名")}{field("lastName", "姓")}{field("nickName", "昵称")}{field("displayName", "显示名称")}<div className={styles.formActions}><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "保存中…" : "保存个人资料"}</Button>{mutation.isError ? <span className={styles.errorText}>{identityMutationErrorText(mutation.error)}</span> : mutation.isSuccess ? <span className={styles.successText}>个人资料已保存</span> : null}{mutation.isError && identityMutationNeedsLogin(mutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form><p className={styles.note}>资料保存到当前登录用户的 ZITADEL Profile；Shuomi 不建立第二套身份资料。</p></div>;
}

function VerificationPanel({ data, organization, interactive = false, returnTo, identityVerificationOutcomeUnknown = false, onIdentityVerificationOutcomeUnknown }: { data: AccountProfile; organization?: AccountOrganization | null; interactive?: boolean; returnTo?: string; identityVerificationOutcomeUnknown?: boolean; onIdentityVerificationOutcomeUnknown?: () => void }) {
  return <>
    <Panel title="身份认证" description="个人与企业身份认证由独立认证服务提供；当前账户中心未接入该能力。" className={styles.status}>
      <div className={styles.verificationCards}><article><strong>个人身份认证</strong><span>暂不可用</span><small>当前没有个人实名状态或认证流程 owner。</small></article><article><strong>企业身份认证</strong><span>暂不可用</span><small>组织授权状态不代表企业身份认证结果。</small></article></div>
    </Panel>
    <Panel title="账户联系方式验证" description="手机和邮箱验证状态由 ZITADEL 官方 Auth API 返回；企业授权仅表示当前登录授权有效。" className={styles.status}>
      <Fields items={[["手机验证", verification(data.phoneNumberVerified)], ["邮箱验证", verification(data.emailVerified)], ["企业授权", organization ? "已读取当前企业授权" : "未读取当前企业授权"]]} />{interactive ? <IdentityVerificationManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/verification"} identityVerificationOutcomeUnknown={identityVerificationOutcomeUnknown} onIdentityVerificationOutcomeUnknown={onIdentityVerificationOutcomeUnknown} /> : null}
    </Panel>
  </>;
}

function IdentityContactManagement({ data, returnTo, identityContactOutcomeUnknown, onIdentityContactOutcomeUnknown }: { data: AccountProfile; returnTo: string; identityContactOutcomeUnknown: boolean; onIdentityContactOutcomeUnknown?: () => void }) {
  const queryClient = useQueryClient();
  const [email, setEmail] = useState(data.email ?? "");
  const [phone, setPhone] = useState(data.phoneNumber ?? "");
  const emailMutation = useMutation({ mutationFn: () => setAccountEmail({ expectedUserId: data.userId, email }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityContactOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const phoneMutation = useMutation({ mutationFn: () => setAccountPhone({ expectedUserId: data.userId, phone }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityContactOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const [oldPassword, setOldPassword] = useState(""); const [newPassword, setNewPassword] = useState("");
  const passwordMutation = useMutation({ mutationFn: (input: { oldPassword: string; newPassword: string }) => updateAccountPassword({ expectedUserId: data.userId, ...input }), onSettled: () => { setOldPassword(""); setNewPassword(""); } });
  const errorText = (mutation: { isError: boolean; error: unknown }) => mutation.isError ? identityMutationErrorText(mutation.error) : null;
  return <div className={styles.settingsSections}>
    {identityContactOutcomeUnknown ? <p className={styles.errorText} role="alert">操作结果待核实，已刷新资料；请确认当前联系方式状态后，再点击“刷新资料”重新提交。</p> : null}
    <Panel title="联系方式" description="更换手机号或邮箱时，需要单独完成身份验证。">
      <form onSubmit={event => { event.preventDefault(); emailMutation.mutate(); }}><label>邮箱地址<input type="email" value={email} onChange={event => setEmail(event.target.value)} disabled={emailMutation.isPending || identityContactOutcomeUnknown} /></label><div className={styles.formActions}><Button type="submit" disabled={emailMutation.isPending || identityContactOutcomeUnknown}>{emailMutation.isPending ? "提交中…" : "更换邮箱"}</Button>{errorText(emailMutation) ? <span className={styles.errorText}>{errorText(emailMutation)}</span> : emailMutation.isSuccess ? <span className={styles.successText}>已发送验证邮件</span> : null}{emailMutation.isError && identityMutationNeedsLogin(emailMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
      <form onSubmit={event => { event.preventDefault(); phoneMutation.mutate(); }}><label>手机号码<input type="tel" value={phone} onChange={event => setPhone(event.target.value)} disabled={phoneMutation.isPending || identityContactOutcomeUnknown} /></label><div className={styles.formActions}><Button type="submit" disabled={phoneMutation.isPending || identityContactOutcomeUnknown}>{phoneMutation.isPending ? "提交中…" : "更换手机号"}</Button>{errorText(phoneMutation) ? <span className={styles.errorText}>{errorText(phoneMutation)}</span> : phoneMutation.isSuccess ? <span className={styles.successText}>已发送短信验证码</span> : null}{phoneMutation.isError && identityMutationNeedsLogin(phoneMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
    </Panel>
    <Panel title="登录与安全" description="密码更新由当前登录身份服务处理。">
      <form onSubmit={event => { event.preventDefault(); passwordMutation.mutate({ oldPassword, newPassword }); }}><label>当前密码<input type="password" value={oldPassword} onChange={event => setOldPassword(event.target.value)} autoComplete="current-password" disabled={passwordMutation.isPending} /></label><label>新密码<input type="password" value={newPassword} onChange={event => setNewPassword(event.target.value)} autoComplete="new-password" disabled={passwordMutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={passwordMutation.isPending}>{passwordMutation.isPending ? "提交中…" : "修改密码"}</Button>{errorText(passwordMutation) ? <span className={styles.errorText}>{errorText(passwordMutation)}</span> : passwordMutation.isSuccess ? <span className={styles.successText}>密码已更新</span> : null}{passwordMutation.isError && identityMutationNeedsLogin(passwordMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
      <p className={styles.note}>验证码由 ZITADEL 官方发送；Shuomi 不保存密码或验证码。</p>
    </Panel>
  </div>;
}

function identityProfileInput(profile: { firstName: string; lastName: string; nickName: string; displayName: string; preferredLanguage: string; gender: string }): AccountIdentityProfileInput {
  return { firstName: profile.firstName, lastName: profile.lastName, nickName: profile.nickName, displayName: profile.displayName, preferredLanguage: profile.preferredLanguage, gender: profile.gender as AccountIdentityProfileInput["gender"] };
}

function IdentityVerificationManagement({ data, returnTo, identityVerificationOutcomeUnknown, onIdentityVerificationOutcomeUnknown }: { data: AccountProfile; returnTo: string; identityVerificationOutcomeUnknown: boolean; onIdentityVerificationOutcomeUnknown?: () => void }) {
  const queryClient = useQueryClient();
  const [emailCode, setEmailCode] = useState(""); const [phoneCode, setPhoneCode] = useState("");
  const emailMutation = useMutation({ mutationFn: () => verifyAccountEmail({ expectedUserId: data.userId, code: emailCode }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityVerificationOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const phoneMutation = useMutation({ mutationFn: () => verifyAccountPhone({ expectedUserId: data.userId, code: phoneCode }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityVerificationOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const emailResend = useMutation({ mutationFn: () => resendAccountEmailVerification({ expectedUserId: data.userId }), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityVerificationOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const phoneResend = useMutation({ mutationFn: () => resendAccountPhoneVerification({ expectedUserId: data.userId }), onError: error => { if (identityOutcomeUnknown(error)) { onIdentityVerificationOutcomeUnknown?.(); reconcileIdentityQueries(queryClient, data.userId); } } });
  const error = (mutation: { isError: boolean; error: unknown }) => mutation.isError ? mutation.error instanceof AccountReadError && mutation.error.code === "IDENTITY_VERIFICATION_FAILED" ? "验证码无效或已过期" : identityMutationErrorText(mutation.error) : null;
  const emailError = error(emailMutation) ?? error(emailResend);
  const phoneError = error(phoneMutation) ?? error(phoneResend);
  const needsEmailLogin = (emailMutation.isError && identityMutationNeedsLogin(emailMutation.error)) || (emailResend.isError && identityMutationNeedsLogin(emailResend.error));
  const needsPhoneLogin = (phoneMutation.isError && identityMutationNeedsLogin(phoneMutation.error)) || (phoneResend.isError && identityMutationNeedsLogin(phoneResend.error));
  return <div className={styles.profileForm}>
    {identityVerificationOutcomeUnknown ? <p className={styles.errorText} role="alert">操作结果待核实，已刷新资料；请确认当前验证状态后，再点击“刷新资料”重新发送。</p> : null}
    {data.email && data.emailVerified !== true ? <form onSubmit={event => { event.preventDefault(); emailMutation.mutate(); }}><label>邮箱验证码<input value={emailCode} onChange={event => setEmailCode(event.target.value)} inputMode="numeric" disabled={emailMutation.isPending || identityVerificationOutcomeUnknown} /></label><div className={styles.formActions}><Button type="submit" disabled={emailMutation.isPending || identityVerificationOutcomeUnknown}>验证邮箱</Button><Button type="button" variant="outline" onClick={() => emailResend.mutate()} disabled={emailResend.isPending || identityVerificationOutcomeUnknown}>重新发送</Button>{emailError ? <span className={styles.errorText}>{emailError}</span> : emailMutation.isSuccess || emailResend.isSuccess ? <span className={styles.successText}>{emailMutation.isSuccess ? "邮箱已验证" : "验证码已发送"}</span> : null}{needsEmailLogin ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form> : null}
    {data.phoneNumber && data.phoneNumberVerified !== true ? <form onSubmit={event => { event.preventDefault(); phoneMutation.mutate(); }}><label>手机验证码<input value={phoneCode} onChange={event => setPhoneCode(event.target.value)} inputMode="numeric" disabled={phoneMutation.isPending || identityVerificationOutcomeUnknown} /></label><div className={styles.formActions}><Button type="submit" disabled={phoneMutation.isPending || identityVerificationOutcomeUnknown}>验证手机</Button><Button type="button" variant="outline" onClick={() => phoneResend.mutate()} disabled={phoneResend.isPending || identityVerificationOutcomeUnknown}>重新发送</Button>{phoneError ? <span className={styles.errorText}>{phoneError}</span> : phoneMutation.isSuccess || phoneResend.isSuccess ? <span className={styles.successText}>{phoneMutation.isSuccess ? "手机已验证" : "验证码已发送"}</span> : null}{needsPhoneLogin ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form> : null}
    {!data.email && !data.phoneNumber ? <p className={styles.note}>当前没有可验证的邮箱或手机号，请先在账户设置中添加。</p> : <p className={styles.note}>验证码校验由 ZITADEL 官方完成；验证结果刷新后以身份服务返回为准。</p>}
  </div>;
}

function identityOutcomeUnknown(error: unknown): boolean { return error instanceof AccountReadError && error.code === "RESULT_UNVERIFIED"; }
function identityMutationNeedsLogin(error: unknown): boolean { return error instanceof AccountReadError && ["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED"].includes(error.code); }
function IdentityLoginRecovery({ returnTo }: { returnTo: string }) { return <Button asChild variant="outline"><Link href={`/login?returnTo=${encodeURIComponent(returnTo)}`} prefetch={false}>重新登录</Link></Button>; }
function identityMutationErrorText(error: unknown): string {
  if (identityOutcomeUnknown(error)) return "操作结果待核实，已刷新资料；请勿重复提交，确认当前状态后再操作";
  if (error instanceof AccountReadError && error.code === "AUTHENTICATION_REQUIRED") return "登录已失效，请重新登录";
  if (error instanceof AccountReadError && error.code === "IDENTITY_CONTEXT_CHANGED") return "登录身份已变化，请重新登录";
  if (error instanceof AccountReadError && error.code === "IDENTITY_PROVIDER_REJECTED") return "身份服务拒绝了本次修改";
  return "操作失败，请稍后重试";
}
function reconcileIdentityQueries(queryClient: ReturnType<typeof useQueryClient>, userId: string): void {
  void queryClient.invalidateQueries({ queryKey: ["account"] });
  void queryClient.invalidateQueries({ queryKey: ["account", "identity-profile", userId] });
}

function BusinessProfilePanel({ profile, organizationId }: { profile?: AccountBusinessProfile | null; organizationId?: string }) {
  const [form, setForm] = useState<AccountBusinessProfileInput>(() => profileInput(profile));
  const mutation = useMutation({ mutationFn: (input: AccountBusinessProfileInput) => updateAccountBusinessProfile({ expectedUserId: profile?.userId ?? "", expectedOrganizationId: organizationId ?? "", input }), onSuccess: saved => setForm(profileInput(saved)) });
  if (!profile) return <Panel title="经营画像" description="业务档案服务暂未接入" className={styles.business}><BusinessGroups><BusinessGroup title="经营角色"><UnavailableChoice title="用户角色" /></BusinessGroup><BusinessGroup title="店铺情况"><UnavailableChoice title="店铺情况" /></BusinessGroup><BusinessGroup title="选择店铺平台、经营站点与店铺类型"><UnavailableChoice title="店铺平台" /><UnavailableChoice title="经营站点" /><UnavailableChoice title="店铺类型" /></BusinessGroup><BusinessGroup title="你是否拥有工厂或供应链？"><UnavailableChoice title="工厂与供应链" /></BusinessGroup><BusinessGroup title="你目前需要什么服务？"><UnavailableChoice title="所需服务" /></BusinessGroup></BusinessGroups><p className={styles.note}>业务档案与企业项目权限分别管理。</p></Panel>;
  const submit = (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (profile) mutation.mutate(form); };
  const disabled = !profile || !organizationId || mutation.isPending;
  const scalar = (key: "userRole" | "shopSituation" | "factorySituation" | "shopType", label: string, options: readonly string[]) => <fieldset className={styles.choiceField} key={key} disabled={disabled}><legend>{label}</legend><div className={styles.choiceOptions} role="group" aria-label={label}>{options.map(option => <button key={option} type="button" className={form[key] === option ? styles.choiceSelected : undefined} aria-pressed={form[key] === option} onClick={() => setForm(current => ({ ...current, [key]: current[key] === option ? "" : option }))}>{option}</button>)}</div><label className={styles.otherChoice}>其他 / 补充<input value={options.includes(form[key]) ? "" : form[key]} onChange={event => setForm(current => ({ ...current, [key]: event.target.value }))} /></label></fieldset>;
  const multiple = (key: "platforms" | "sites" | "services", label: string, options: readonly string[]) => {
    const custom = form[key].filter(value => !options.includes(value));
    return <fieldset className={styles.choiceField} key={key} disabled={disabled}><legend>{label}（可多选）</legend><div className={styles.choiceOptions} role="group" aria-label={`${label}，可多选`}>{options.map(option => <button key={option} type="button" className={form[key].includes(option) ? styles.choiceSelected : undefined} aria-pressed={form[key].includes(option)} onClick={() => setForm(current => ({ ...current, [key]: current[key].includes(option) ? current[key].filter(value => value !== option) : [...current[key], option] }))}>{option}</button>)}</div><label className={styles.otherChoice}>其他选项（逗号分隔）<input value={custom.join(", ")} onChange={event => setForm(current => ({ ...current, [key]: [...current[key].filter(value => options.includes(value)), ...splitList(event.target.value)] }))} /></label></fieldset>;
  };
  return <Panel title="经营画像" description="与首次完善资料使用相同的经营信息；保存后仍由账户中心 owner 持久化。" className={styles.business}><aside className={styles.businessGuide}><strong>经营画像说明</strong><p>这些信息用于匹配当前可用的业务服务，不代表店铺、工厂或平台的外部核验结果。</p></aside><form className={styles.businessForm} onSubmit={submit}>
    <BusinessGroups>
      <BusinessGroup title="经营角色">{scalar("userRole", "你是什么角色？", ["跨境电商创业者", "跨境电商卖家", "工厂或供应商", "企业电商团队", "OPC电商社区或园区", "电商服务商", "其他"])}</BusinessGroup>
      <BusinessGroup title="店铺情况">{scalar("shopSituation", "你目前有没有店铺？", ["暂时没有店铺", "正在准备开店", "已有店铺", "有多个店铺或店群"])}</BusinessGroup>
      <BusinessGroup title="选择店铺平台、经营站点与店铺类型">{multiple("platforms", "店铺平台", ["Amazon", "SHEIN", "Temu", "TikTok Shop", "Walmart", "Shopee", "Lazada", "AliExpress", "Etsy", "独立站"])}{multiple("sites", "经营站点", ["美国站", "加拿大站", "日本站", "英国站", "欧洲站"])}{scalar("shopType", "店铺类型", ["全托店", "半托店", "自营店", "其他"])}</BusinessGroup>
      <BusinessGroup title="你是否拥有工厂或供应链？">{scalar("factorySituation", "工厂与供应链", ["有自有工厂", "有长期合作工厂", "有稳定供应链资源", "暂时没有工厂或供应链资源", "其他"])}</BusinessGroup>
      <BusinessGroup title="你目前需要什么服务？">{multiple("services", "所需服务", ["AI选品与市场分析", "商品采集与刊登", "商品图片与内容生成", "POD商品定制", "供应链与货盘对接", "店铺智能运营", "广告投放与优化", "客服自动化", "订单库存与ERP", "多店铺矩阵管理", "创业培训与陪跑", "店铺联合运营", "品牌出海", "企业功能定制", "OPC电商社区落地", "暂时不确定"])}</BusinessGroup>
    </BusinessGroups>
    <div className={styles.formActions}><Button type="submit" disabled={disabled}>{mutation.isPending ? "保存中…" : "保存修改"}</Button>{mutation.isError ? <span className={styles.errorText}>{mutation.error instanceof AccountReadError ? "保存失败，请稍后重试" : "保存失败"}</span> : mutation.isSuccess ? <span className={styles.successText}>已保存</span> : null}</div>
  </form><p className={styles.note}>身份联系方式仍由登录服务管理；经营画像由账户中心持久化。</p></Panel>;
}
function BusinessGroups({ children }: { children: ReactNode }) { return <div className={styles.businessGroups}>{children}</div>; }
function BusinessGroup({ title, children }: { title: string; children: ReactNode }) { return <section className={styles.businessGroup} aria-label={title}><h3>{title}</h3><div>{children}</div></section>; }
function UnavailableChoice({ title }: { title: string }) { return <div className={styles.businessUnavailable}><span>{title}</span><strong>未提供</strong></div>; }
function profileInput(profile?: AccountBusinessProfile | null): AccountBusinessProfileInput { return { userRole: profile?.userRole ?? "", shopSituation: profile?.shopSituation ?? "", factorySituation: profile?.factorySituation ?? "", platforms: profile?.platforms ?? [], sites: profile?.sites ?? [], shopType: profile?.shopType ?? "", services: profile?.services ?? [] }; }
function splitList(value: string) { return [...new Set(value.split(",").map(item => item.trim()).filter(Boolean))].slice(0, 16); }

export function OrganizationView({ data }: { data: AccountOrganization }) {
  const members = useQuery({ queryKey: ["account-overview", data.userId, data.effectiveOrganizationId, "members"], queryFn: ({ signal }) => getMembers({ expectedUserId: data.userId, expectedOrganizationId: data.effectiveOrganizationId, signal }, 0), gcTime: 0, staleTime: 0, retry: false });
  const commercial = useQuery({ queryKey: ["account-overview", data.userId, data.effectiveOrganizationId, "commercial"], queryFn: ({ signal }) => getCommercialOverview(data.effectiveOrganizationId, signal), gcTime: 0, staleTime: 0, retry: false });
  const allocation = useQuery({ queryKey: ["account-overview", data.userId, data.effectiveOrganizationId, "allocation"], queryFn: ({ signal }) => getMemberTokenAllocations(data.effectiveOrganizationId, signal), gcTime: 0, staleTime: 0, retry: false });
  const audit = useQuery({ queryKey: ["account-overview", data.userId, data.effectiveOrganizationId, "audit"], queryFn: ({ signal }) => getAccountAudit({ expectedUserId: data.userId, expectedOrganizationId: data.effectiveOrganizationId, signal }), gcTime: 0, staleTime: 0, retry: false });
  const metric = (query: { isPending: boolean; isError: boolean }, value: ReactNode, unavailable: string) => query.isPending ? "正在读取" : query.isError ? unavailable : value;
  const activeMembers = members.data?.items.filter(member => member.state === "active").length;
  return <>
    <Card className={styles.identity}><div className={styles.organizationIdentity}><Image src="/console/account/organization-avatar.svg" width={64} height={64} alt="" unoptimized /><div><h2>{provided(data.name)}</h2><p>当前有效企业：{data.effectiveOrganizationId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div></div><div className={styles.identityStatus}><span className={styles.badge}>企业信息 · 只读</span><span className={styles.identityMeta}>当前组织角色：{data.roles.length ? data.roles.join("、") : "未提供"}</span></div></Card>
    <div className={styles.enterpriseMetrics} aria-label="当前企业信息"><article><span>企业成员</span><strong>{metric(members, members.data?.total, "暂不可用")}</strong><small>{activeMembers === undefined ? "有效成员统计未提供" : `当前页有效成员 ${activeMembers} 人`}</small></article><article><span>已绑定店铺</span><strong>未提供</strong><small>当前资源 owner 未返回已绑定店铺数</small></article><article><span>待处理邀请</span><strong>未提供</strong><small>当前成员目录未返回邀请状态</small></article><article><span>已授予权益</span><strong>{metric(commercial, commercial.data?.entitlements.length, "暂不可用")}</strong><small>{commercial.data?.subscription ? `当前订阅：${commercial.data.subscription.plan_name ?? commercial.data.subscription.plan_code}` : commercial.isError ? "权益服务未返回订阅" : "当前无订阅"}</small></article></div>
    <h2 className={styles.sectionTitle}>企业管理</h2><div className={styles.managementGrid}>{[["成员与权限", "查看企业成员；获准管理员可邀请成员、调整角色和移除成员"], ["资源与额度", "查看企业已授予权益和成员 Token 分配"], ["操作记录", "查看账户资料、成员、额度与源账号的已提交事件"]].map(([title, description]) => <Panel key={title} title={title} description={description}>{title === "成员与权限" ? <><Button asChild variant="outline"><Link href="/workbench/account/organization/members" prefetch={false}>管理成员</Link></Button><p className={styles.note}>角色与可执行操作以当前组织授权 owner 为准。</p></> : title === "资源与额度" ? <Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>管理资源</Link></Button> : <><Button asChild variant="outline"><Link href="/workbench/account/organization/audit" prefetch={false}>查看记录</Link></Button><p className={styles.note}>只展示已提交成功的业务事件。</p></>}</Panel>)}</div>
    <Panel title="企业资源" className={styles.resources}><div className={styles.enterpriseResources}><div><span>订阅权益</span><strong>{commercial.isPending ? "正在读取" : commercial.isError ? "暂不可用" : commercial.data.subscription?.plan_name ?? "无订阅"}</strong></div><div><span>AI Token 总额度</span><strong>{allocation.isPending ? "正在读取" : allocation.isError ? "暂不可用" : allocation.data.enterprise.total}</strong></div><div><span>已分配</span><strong>{allocation.isPending ? "正在读取" : allocation.isError ? "暂不可用" : allocation.data.enterprise.allocated}</strong></div><div><span>已消费</span><strong>{allocation.isPending ? "正在读取" : allocation.isError ? "暂不可用" : allocation.data.enterprise.consumed}</strong></div></div><p className={styles.note}>店铺实际数量、AI 点数与数据余额仅在各自 owner 返回后展示，不由套餐或用量推算。</p></Panel>
    <Panel title="成员资源分配" description="AI Token 使用当前企业 entitlement window 的真实 set-target 分配事实。" className={styles.resources}>
      {allocation.isPending ? <p className={styles.note}>正在读取成员分配…</p> : allocation.isError ? <p className={styles.unavailable} role="status">成员 Token 分配暂不可用。</p> : allocation.data.members.length === 0 ? <p className={styles.unavailable} role="status">当前周期暂无成员分配记录。</p> : <div className={styles.allocationPreview}><div className={styles.allocationHead}><span>成员 / 角色</span><span>已分配</span><span>已消费</span><span>剩余</span></div>{allocation.data.members.slice(0, 5).map(member => <div className={styles.allocationLine} key={member.memberId}><span><strong>{member.displayName || member.loginName || member.userId}</strong><small>{member.loginName || member.userId}</small></span><span>{member.allocation.allocated}</span><span>{member.allocation.consumed}</span><span>{member.allocation.remaining}</span></div>)}</div>}
      <Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>查看资源与额度</Link></Button>
    </Panel>
    <Panel title="最近操作记录" description="时间为业务操作时间；记录只供追溯。" className={styles.resources}>
      {audit.isPending ? <p className={styles.note}>正在读取操作记录…</p> : audit.isError ? <p className={styles.unavailable} role="status">操作记录暂不可用。</p> : audit.data.items.length === 0 ? <p className={styles.unavailable} role="status">暂无已提交的操作记录。</p> : <ul className={styles.auditPreview}>{audit.data.items.slice(0, 4).map(item => <li key={`${item.eventType}:${item.relation.reference}:${item.relation.version}`}><time dateTime={item.time}>{new Date(item.time).toLocaleString("zh-CN", { timeZone: "Asia/Singapore", hour12: false })}</time><span>{item.actor}</span><strong>{item.operation} · {item.objectReference}</strong></li>)}</ul>}
      <Button asChild variant="outline"><Link href="/workbench/account/organization/audit" prefetch={false}>查看操作记录</Link></Button>
    </Panel><Provenance data={data} />
  </>;
}
