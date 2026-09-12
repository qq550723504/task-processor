import Image from "next/image";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import type { AccountOrganization, AccountProfile } from "@/lib/api/account";
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
export function ProfileView({ data }: { data: AccountProfile }) {
  return <><Card className={styles.identity}><div><h2>{data.displayName?.trim() || "当前账户"}</h2><p>账户 ID：{data.userId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div><span className={styles.badge}>账户资料 · 只读</span></Card>
    {![data.displayName, data.email, data.phoneNumber].some(value => value?.trim()) ? <ConsoleState kind="empty" title="暂未提供个人资料">登录服务本次未提供显示名称、手机号码或电子邮箱。你仍可查看账户标识并刷新资料。</ConsoleState> : null}
    <div className={styles.profileGrid}><div className={styles.profileMain}>
      <Panel title="账户设置" description="个人身份资料由登录服务提供"><Fields items={[["显示名称", provided(data.displayName)], ["手机号码", provided(data.phoneNumber)], ["电子邮箱", provided(data.email)], ["所在地区", "未提供"], ["注册时间", "未提供"]]} /></Panel>
      <Panel title="账号状态" className={styles.status}><Fields items={[["手机验证", verification(data.phoneNumberVerified)], ["邮箱验证", verification(data.emailVerified)], ["登录密码", "未提供"]]} /></Panel>
    </div><Panel title="经营画像" description="业务档案服务暂未接入" className={styles.business}><Fields items={[["用户角色", "未提供"], ["店铺情况", "未提供"], ["工厂情况", "未提供"], ["经营平台", "未提供"], ["经营站点", "未提供"], ["店铺类型", "未提供"], ["所需服务", "未提供"]]} /><p className={styles.note}>业务档案与企业项目权限分别管理。</p></Panel></div><Provenance data={data} /></>;
}
export function OrganizationView({ data }: { data: AccountOrganization }) {
  return <><Card className={styles.identity}><div className={styles.organizationIdentity}><Image src="/console/account/organization-avatar.svg" width={64} height={64} alt="" unoptimized /><div><h2>{provided(data.name)}</h2><p>当前有效企业：{data.effectiveOrganizationId}</p><p>归属企业（Home）：{data.homeOrganizationId}</p></div></div><span className={styles.badge}>企业信息 · 只读</span></Card>
    <div className={styles.enterpriseSummary}><p>当前账号的项目权限：{data.roles.length ? data.roles.map(role => <span key={role} className={styles.role}>{role}</span>) : "未提供"}</p><p>企业认证：未提供 · 管理员：未提供</p></div>
    <h2 className={styles.sectionTitle}>企业管理</h2><div className={styles.managementGrid}>{[["成员与权限", "成员管理与权限配置"], ["资源与额度", "企业资源与额度信息"], ["操作记录", "企业操作与审计记录"]].map(([title, description]) => <Panel key={title} title={title} description={description}><p className={styles.note}>暂未接入</p></Panel>)}</div>
    <Panel title="企业资源" className={styles.resources}><p className={styles.note}>资源服务暂未接入，余额与用量未提供。</p></Panel>
    <Panel title="成员资源分配" description="分配数据暂未接入"><div className={styles.unavailable}>成员、资源配额与使用情况未提供</div></Panel><Provenance data={data} /></>;
}
