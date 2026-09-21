"use client";

import Image from "next/image";
import Link from "next/link";
import { useMutation } from "@tanstack/react-query";
import { useState, type FormEvent, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AccountReadError, updateAccountBusinessProfile, type AccountBusinessProfile, type AccountBusinessProfileInput, type AccountOrganization, type AccountProfile } from "@/lib/api/account";
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

export function ProfileView({ data, business }: { data: AccountProfile; business?: AccountBusinessProfile | null }) {
  return <><Card className={styles.identity}><div><h2>{data.displayName?.trim() || "当前账户"}</h2><p>账户 ID：{data.userId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div><span className={styles.badge}>账户资料 · 只读</span></Card>
    {![data.displayName, data.email, data.phoneNumber].some(value => value?.trim()) ? <ConsoleState kind="empty" title="暂未提供个人资料">登录服务本次未提供显示名称、手机号码或电子邮箱。你仍可查看账户标识并刷新资料。</ConsoleState> : null}
    <div className={styles.profileGrid}><div className={styles.profileMain}>
      <Panel title="账户设置" description="个人身份资料由登录服务提供"><Fields items={[["用户名", provided(data.displayName)], ["手机号码", provided(data.phoneNumber)], ["邮箱地址", provided(data.email)], ["所在地区", "未提供"], ["注册时间", "未提供"]]} /></Panel>
      <Panel title="认证信息" description="认证状态由登录服务与当前企业授权共同决定" className={styles.status}><Fields items={[["手机验证", verification(data.phoneNumberVerified)], ["邮箱验证", verification(data.emailVerified)], ["企业授权", "已读取当前企业授权"]]} /></Panel>
    </div><BusinessProfilePanel profile={business} /></div><Provenance data={data} /></>;
}

function BusinessProfilePanel({ profile }: { profile?: AccountBusinessProfile | null }) {
  const [form, setForm] = useState<AccountBusinessProfileInput>(() => profileInput(profile));
  const mutation = useMutation({ mutationFn: (input: AccountBusinessProfileInput) => updateAccountBusinessProfile({ expectedUserId: profile?.userId ?? "", input }), onSuccess: saved => setForm(profileInput(saved)) });
  if (!profile) return <Panel title="经营画像" description="业务档案服务暂未接入" className={styles.business}><Fields items={[["用户角色", "未提供"], ["店铺情况", "未提供"], ["工厂情况", "未提供"], ["经营平台", "未提供"], ["经营站点", "未提供"], ["店铺类型", "未提供"], ["所需服务", "未提供"]]} /><p className={styles.note}>业务档案与企业项目权限分别管理。</p></Panel>;
  const submit = (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (profile) mutation.mutate(form); };
  const field = (key: "userRole" | "shopSituation" | "factorySituation" | "shopType", label: string) => <label key={key}>{label}<input value={form[key]} onChange={event => setForm(current => ({ ...current, [key]: event.target.value }))} disabled={!profile || mutation.isPending} /></label>;
  return <Panel title="经营画像" description="用于账户识别与业务服务匹配" className={styles.business}><form className={styles.profileForm} onSubmit={submit}>{field("userRole", "用户角色")}{field("shopSituation", "店铺情况")}{field("factorySituation", "工厂情况")}{field("shopType", "店铺类型")}<label>经营平台<input value={form.platforms.join(", ")} onChange={event => setForm(current => ({ ...current, platforms: splitList(event.target.value) }))} disabled={!profile || mutation.isPending} /></label><label>经营站点<input value={form.sites.join(", ")} onChange={event => setForm(current => ({ ...current, sites: splitList(event.target.value) }))} disabled={!profile || mutation.isPending} /></label><label>所需服务<input value={form.services.join(", ")} onChange={event => setForm(current => ({ ...current, services: splitList(event.target.value) }))} disabled={!profile || mutation.isPending} /></label><div className={styles.formActions}><Button type="submit" disabled={!profile || mutation.isPending}>{mutation.isPending ? "保存中…" : "保存经营画像"}</Button>{mutation.isError ? <span className={styles.errorText}>{mutation.error instanceof AccountReadError ? "保存失败，请稍后重试" : "保存失败"}</span> : mutation.isSuccess ? <span className={styles.successText}>已保存</span> : null}</div></form><p className={styles.note}>身份联系方式仍由登录服务管理；经营画像由账户中心持久化。</p></Panel>;
}
function profileInput(profile?: AccountBusinessProfile | null): AccountBusinessProfileInput { return { userRole: profile?.userRole ?? "", shopSituation: profile?.shopSituation ?? "", factorySituation: profile?.factorySituation ?? "", platforms: profile?.platforms ?? [], sites: profile?.sites ?? [], shopType: profile?.shopType ?? "", services: profile?.services ?? [] }; }
function splitList(value: string) { return [...new Set(value.split(",").map(item => item.trim()).filter(Boolean))].slice(0, 16); }

export function OrganizationView({ data }: { data: AccountOrganization }) {
  return <><Card className={styles.identity}><div className={styles.organizationIdentity}><Image src="/console/account/organization-avatar.svg" width={64} height={64} alt="" unoptimized /><div><h2>{provided(data.name)}</h2><p>当前有效企业：{data.effectiveOrganizationId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div></div><span className={styles.badge}>企业信息 · 只读</span></Card>
    <div className={styles.enterpriseSummary}><p>当前账号的项目权限：{data.roles.length ? data.roles.map(role => <span key={role} className={styles.role}>{role}</span>) : "未提供"}</p><p>企业认证：未提供 · 管理员：{data.roles.includes("org_admin") ? "当前账号" : "未提供"}</p></div>
    <h2 className={styles.sectionTitle}>企业管理</h2><div className={styles.managementGrid}>{[["成员与权限", "查看企业成员；获准管理员可邀请成员、调整角色和移除成员"], ["资源与额度", "企业资源与额度信息"], ["操作记录", "源账号登记、启用和停用的已提交成功记录"]].map(([title, description]) => <Panel key={title} title={title} description={description}>{title === "成员与权限" ? <><Button asChild variant="outline"><Link href="/workbench/account/organization/members" prefetch={false}>查看成员与权限</Link></Button><p className={styles.note}>可用操作以当前企业权限为准。</p></> : title === "资源与额度" ? <Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>查看资源与额度</Link></Button> : <><Button asChild variant="outline"><Link href="/workbench/account/organization/audit" prefetch={false}>查看操作记录</Link></Button><p className={styles.note}>当前仅覆盖源账号；成员、权限、续费及失败尝试尚未纳入。</p></>}</Panel>)}</div>
    <Panel title="企业资源" className={styles.resources}><p className={styles.note}>企业额度与已记录用量进入资源与额度查看；成员 Token 分配使用当前企业 entitlement window。</p></Panel><Panel title="成员资源分配" description="AI Token 使用 set-target 分配并进入账户审计"><Button asChild variant="outline"><Link href="/workbench/account/organization/resources" prefetch={false}>管理成员额度</Link></Button></Panel><Provenance data={data} /></>;
}
