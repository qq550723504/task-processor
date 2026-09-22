"use client";

import Image from "next/image";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AccountReadError, updateAccountBusinessProfile, type AccountBusinessProfile, type AccountBusinessProfileInput, type AccountOrganization, type AccountProfile } from "@/lib/api/account";
import { getAccountIdentityProfile, resendAccountEmailVerification, resendAccountPhoneVerification, setAccountEmail, setAccountPhone, updateAccountIdentityProfile, updateAccountPassword, verifyAccountEmail, verifyAccountPhone, type AccountIdentityProfileInput } from "@/lib/api/account-identity";
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

export function ProfileView({ data, business, organization, organizationId, section = "summary" }: { data: AccountProfile; business?: AccountBusinessProfile | null; organization?: AccountOrganization | null; organizationId?: string; section?: ProfileSection }) {
  const identity = <ProfileIdentity data={data} />;
  if (section === "settings") return <>{identity}<AccountSettingsPanel data={data} editable returnTo="/workbench/account/profile/settings" /><Provenance data={data} /></>;
  if (section === "business") return <>{identity}<BusinessProfilePanel profile={business} organizationId={organizationId} /><Provenance data={data} /></>;
  if (section === "verification") return <>{identity}<VerificationPanel data={data} interactive organization={organization} returnTo="/workbench/account/profile/verification" /><Provenance data={data} /></>;
  return <>{identity}
    {![data.displayName, data.email, data.phoneNumber].some(value => value?.trim()) ? <ConsoleState kind="empty" title="暂未提供个人资料">登录服务本次未提供显示名称、手机号码或电子邮箱。你仍可查看账户标识并刷新资料。</ConsoleState> : null}
    <div className={styles.profileGrid}><div className={styles.profileMain}>
      <AccountSettingsPanel data={data} />
      <VerificationPanel data={data} />
    </div><BusinessProfilePanel profile={business} organizationId={organizationId} /></div><Provenance data={data} /></>;
}

function ProfileIdentity({ data }: { data: AccountProfile }) {
  return <Card className={styles.identity}><div><h2>{data.displayName?.trim() || "当前账户"}</h2><p>账户 ID：{data.userId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div><span className={styles.badge}>账户资料 · 当前用户</span></Card>;
}

function AccountSettingsPanel({ data, editable = false, returnTo }: { data: AccountProfile; editable?: boolean; returnTo?: string }) {
  return <Panel title="账户设置" description="个人身份资料由 ZITADEL 官方 Auth API 管理"><Fields items={[["显示名称", provided(data.displayName)], ["手机号码", provided(data.phoneNumber)], ["邮箱地址", provided(data.email)], ["所在地区", "未提供"], ["注册时间", "未提供"]]} />{editable ? <><IdentityProfileManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/settings"} /><IdentityContactManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/settings"} /></> : <p className={styles.note}>进入账户设置后，可在 Shuomi 内修改个人资料和联系方式；验证仍由 ZITADEL 官方流程完成。</p>}</Panel>;
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

function VerificationPanel({ data, organization, interactive = false, returnTo }: { data: AccountProfile; organization?: AccountOrganization | null; interactive?: boolean; returnTo?: string }) {
  return <Panel title="认证信息" description="认证状态由 ZITADEL 官方 Auth API 返回" className={styles.status}><Fields items={[["手机验证", verification(data.phoneNumberVerified)], ["邮箱验证", verification(data.emailVerified)], ["企业授权", organization ? "已读取当前企业授权" : "未读取当前企业授权"]]} />{interactive ? <IdentityVerificationManagement data={data} returnTo={returnTo ?? "/workbench/account/profile/verification"} /> : null}</Panel>;
}

function IdentityContactManagement({ data, returnTo }: { data: AccountProfile; returnTo: string }) {
  const queryClient = useQueryClient();
  const [email, setEmail] = useState(data.email ?? "");
  const [phone, setPhone] = useState(data.phoneNumber ?? "");
  const emailMutation = useMutation({ mutationFn: () => setAccountEmail({ expectedUserId: data.userId, email }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const phoneMutation = useMutation({ mutationFn: () => setAccountPhone({ expectedUserId: data.userId, phone }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const [oldPassword, setOldPassword] = useState(""); const [newPassword, setNewPassword] = useState("");
  const passwordMutation = useMutation({ mutationFn: (input: { oldPassword: string; newPassword: string }) => updateAccountPassword({ expectedUserId: data.userId, ...input }), onSettled: () => { setOldPassword(""); setNewPassword(""); } });
  const errorText = (mutation: { isError: boolean; error: unknown }) => mutation.isError ? identityMutationErrorText(mutation.error) : null;
  return <div className={styles.profileForm}>
    <form onSubmit={event => { event.preventDefault(); emailMutation.mutate(); }}><label>邮箱地址<input type="email" value={email} onChange={event => setEmail(event.target.value)} disabled={emailMutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={emailMutation.isPending}>{emailMutation.isPending ? "提交中…" : "修改邮箱"}</Button>{errorText(emailMutation) ? <span className={styles.errorText}>{errorText(emailMutation)}</span> : emailMutation.isSuccess ? <span className={styles.successText}>已发送验证邮件</span> : null}{emailMutation.isError && identityMutationNeedsLogin(emailMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
    <form onSubmit={event => { event.preventDefault(); phoneMutation.mutate(); }}><label>手机号码<input type="tel" value={phone} onChange={event => setPhone(event.target.value)} disabled={phoneMutation.isPending} placeholder="+8613800000000" /></label><div className={styles.formActions}><Button type="submit" disabled={phoneMutation.isPending}>{phoneMutation.isPending ? "提交中…" : "修改手机号"}</Button>{errorText(phoneMutation) ? <span className={styles.errorText}>{errorText(phoneMutation)}</span> : phoneMutation.isSuccess ? <span className={styles.successText}>已发送短信验证码</span> : null}{phoneMutation.isError && identityMutationNeedsLogin(phoneMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
    <form onSubmit={event => { event.preventDefault(); passwordMutation.mutate({ oldPassword, newPassword }); }}><label>当前密码<input type="password" value={oldPassword} onChange={event => setOldPassword(event.target.value)} autoComplete="current-password" disabled={passwordMutation.isPending} /></label><label>新密码<input type="password" value={newPassword} onChange={event => setNewPassword(event.target.value)} autoComplete="new-password" disabled={passwordMutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={passwordMutation.isPending}>{passwordMutation.isPending ? "提交中…" : "修改密码"}</Button>{errorText(passwordMutation) ? <span className={styles.errorText}>{errorText(passwordMutation)}</span> : passwordMutation.isSuccess ? <span className={styles.successText}>密码已更新</span> : null}{passwordMutation.isError && identityMutationNeedsLogin(passwordMutation.error) ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form>
    <p className={styles.note}>修改邮箱或手机号后，验证码由 ZITADEL 官方发送；Shuomi 不保存密码或验证码。</p>
  </div>;
}

function identityProfileInput(profile: { firstName: string; lastName: string; nickName: string; displayName: string; preferredLanguage: string; gender: string }): AccountIdentityProfileInput {
  return { firstName: profile.firstName, lastName: profile.lastName, nickName: profile.nickName, displayName: profile.displayName, preferredLanguage: profile.preferredLanguage, gender: profile.gender as AccountIdentityProfileInput["gender"] };
}

function IdentityVerificationManagement({ data, returnTo }: { data: AccountProfile; returnTo: string }) {
  const queryClient = useQueryClient();
  const [emailCode, setEmailCode] = useState(""); const [phoneCode, setPhoneCode] = useState("");
  const emailMutation = useMutation({ mutationFn: () => verifyAccountEmail({ expectedUserId: data.userId, code: emailCode }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const phoneMutation = useMutation({ mutationFn: () => verifyAccountPhone({ expectedUserId: data.userId, code: phoneCode }), onSuccess: () => reconcileIdentityQueries(queryClient, data.userId), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const emailResend = useMutation({ mutationFn: () => resendAccountEmailVerification({ expectedUserId: data.userId }), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const phoneResend = useMutation({ mutationFn: () => resendAccountPhoneVerification({ expectedUserId: data.userId }), onError: error => { if (identityOutcomeUnknown(error)) reconcileIdentityQueries(queryClient, data.userId); } });
  const error = (mutation: { isError: boolean; error: unknown }) => mutation.isError ? mutation.error instanceof AccountReadError && mutation.error.code === "IDENTITY_VERIFICATION_FAILED" ? "验证码无效或已过期" : identityMutationErrorText(mutation.error) : null;
  const emailError = error(emailMutation) ?? error(emailResend);
  const phoneError = error(phoneMutation) ?? error(phoneResend);
  const needsEmailLogin = (emailMutation.isError && identityMutationNeedsLogin(emailMutation.error)) || (emailResend.isError && identityMutationNeedsLogin(emailResend.error));
  const needsPhoneLogin = (phoneMutation.isError && identityMutationNeedsLogin(phoneMutation.error)) || (phoneResend.isError && identityMutationNeedsLogin(phoneResend.error));
  return <div className={styles.profileForm}>
    {data.email && data.emailVerified !== true ? <form onSubmit={event => { event.preventDefault(); emailMutation.mutate(); }}><label>邮箱验证码<input value={emailCode} onChange={event => setEmailCode(event.target.value)} inputMode="numeric" disabled={emailMutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={emailMutation.isPending}>验证邮箱</Button><Button type="button" variant="outline" onClick={() => emailResend.mutate()} disabled={emailResend.isPending}>重新发送</Button>{emailError ? <span className={styles.errorText}>{emailError}</span> : emailMutation.isSuccess || emailResend.isSuccess ? <span className={styles.successText}>{emailMutation.isSuccess ? "邮箱已验证" : "验证码已发送"}</span> : null}{needsEmailLogin ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form> : null}
    {data.phoneNumber && data.phoneNumberVerified !== true ? <form onSubmit={event => { event.preventDefault(); phoneMutation.mutate(); }}><label>手机验证码<input value={phoneCode} onChange={event => setPhoneCode(event.target.value)} inputMode="numeric" disabled={phoneMutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={phoneMutation.isPending}>验证手机</Button><Button type="button" variant="outline" onClick={() => phoneResend.mutate()} disabled={phoneResend.isPending}>重新发送</Button>{phoneError ? <span className={styles.errorText}>{phoneError}</span> : phoneMutation.isSuccess || phoneResend.isSuccess ? <span className={styles.successText}>{phoneMutation.isSuccess ? "手机已验证" : "验证码已发送"}</span> : null}{needsPhoneLogin ? <IdentityLoginRecovery returnTo={returnTo} /> : null}</div></form> : null}
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
  if (!profile) return <Panel title="经营画像" description="业务档案服务暂未接入" className={styles.business}><Fields items={[["用户角色", "未提供"], ["店铺情况", "未提供"], ["工厂情况", "未提供"], ["经营平台", "未提供"], ["经营站点", "未提供"], ["店铺类型", "未提供"], ["所需服务", "未提供"]]} /><p className={styles.note}>业务档案与企业项目权限分别管理。</p></Panel>;
  const submit = (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (profile) mutation.mutate(form); };
  const field = (key: "userRole" | "shopSituation" | "factorySituation" | "shopType", label: string) => <label key={key}>{label}<input value={form[key]} onChange={event => setForm(current => ({ ...current, [key]: event.target.value }))} disabled={!profile || !organizationId || mutation.isPending} /></label>;
  return <Panel title="经营画像" description="用于账户识别与业务服务匹配" className={styles.business}><form className={styles.profileForm} onSubmit={submit}>{field("userRole", "用户角色")}{field("shopSituation", "店铺情况")}{field("factorySituation", "工厂情况")}{field("shopType", "店铺类型")}<label>经营平台<input value={form.platforms.join(", ")} onChange={event => setForm(current => ({ ...current, platforms: splitList(event.target.value) }))} disabled={!profile || !organizationId || mutation.isPending} /></label><label>经营站点<input value={form.sites.join(", ")} onChange={event => setForm(current => ({ ...current, sites: splitList(event.target.value) }))} disabled={!profile || !organizationId || mutation.isPending} /></label><label>所需服务<input value={form.services.join(", ")} onChange={event => setForm(current => ({ ...current, services: splitList(event.target.value) }))} disabled={!profile || !organizationId || mutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={!profile || !organizationId || mutation.isPending}>{mutation.isPending ? "保存中…" : "保存经营画像"}</Button>{mutation.isError ? <span className={styles.errorText}>{mutation.error instanceof AccountReadError ? "保存失败，请稍后重试" : "保存失败"}</span> : mutation.isSuccess ? <span className={styles.successText}>已保存</span> : null}</div></form><p className={styles.note}>身份联系方式仍由登录服务管理；经营画像由账户中心持久化。</p></Panel>;
}
function profileInput(profile?: AccountBusinessProfile | null): AccountBusinessProfileInput { return { userRole: profile?.userRole ?? "", shopSituation: profile?.shopSituation ?? "", factorySituation: profile?.factorySituation ?? "", platforms: profile?.platforms ?? [], sites: profile?.sites ?? [], shopType: profile?.shopType ?? "", services: profile?.services ?? [] }; }
function splitList(value: string) { return [...new Set(value.split(",").map(item => item.trim()).filter(Boolean))].slice(0, 16); }

export function OrganizationView({ data }: { data: AccountOrganization }) {
  return <><Card className={styles.identity}><div className={styles.organizationIdentity}><Image src="/console/account/organization-avatar.svg" width={64} height={64} alt="" unoptimized /><div><h2>{provided(data.name)}</h2><p>当前有效企业：{data.effectiveOrganizationId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div></div><span className={styles.badge}>企业信息 · 只读</span></Card>
    <div className={styles.enterpriseSummary}><p>当前账号的项目权限：{data.roles.length ? data.roles.map(role => <span key={role} className={styles.role}>{role}</span>) : "未提供"}</p><p>企业认证：未提供 · 管理员：{data.roles.includes("org_admin") ? "当前账号" : "未提供"}</p></div>
    <h2 className={styles.sectionTitle}>企业管理</h2><div className={styles.managementGrid}>{[["成员与权限", "查看企业成员；获准管理员可邀请成员、调整角色和移除成员"], ["资源与额度", "企业资源与额度信息"], ["操作记录", "查看源账号、成员额度、经营画像和成员权限的已提交成功记录"]].map(([title, description]) => <Panel key={title} title={title} description={description}>{title === "成员与权限" ? <><Button asChild variant="outline"><Link href="/workbench/account/organization/members" prefetch={false}>查看成员与权限</Link></Button><p className={styles.note}>可用操作以当前企业权限为准。</p></> : title === "资源与额度" ? <Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>查看资源与额度</Link></Button> : <><Button asChild variant="outline"><Link href="/workbench/account/organization/audit" prefetch={false}>查看操作记录</Link></Button><p className={styles.note}>只展示已提交成功的业务事件；失败尝试及未提交的 provider 操作不纳入。</p></>}</Panel>)}</div>
    <Panel title="企业资源" className={styles.resources}><p className={styles.note}>企业额度与已记录用量进入资源与额度查看；成员 Token 分配使用当前企业 entitlement window。</p></Panel><Panel title="成员资源分配" description="AI Token 使用 set-target 分配并进入账户审计"><Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>管理成员额度</Link></Button></Panel><Provenance data={data} /></>;
}
