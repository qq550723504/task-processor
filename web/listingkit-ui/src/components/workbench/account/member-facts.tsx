"use client";
import { useQuery } from "@tanstack/react-query";
import { getMemberSummary, MemberScope } from "@/lib/api/members";
import { getInvitationSummary } from "@/lib/api/invitations";
import { getAccountAudit } from "@/lib/api/account-audit";
import styles from "./members.module.css";
export function MemberStats({ scope }: { scope: MemberScope }) {
  const summary = useQuery({
      queryKey: [
        "member-summary",
        scope.expectedUserId,
        scope.expectedOrganizationId,
      ],
      queryFn: ({ signal }) => getMemberSummary({ ...scope, signal }),
      gcTime: 0,
      staleTime: 0,
      retry: false,
    }),
    invites = useQuery({
      queryKey: [
        "invitation-summary",
        scope.expectedUserId,
        scope.expectedOrganizationId,
      ],
      queryFn: ({ signal }) => getInvitationSummary({ ...scope, signal }),
      gcTime: 0,
      staleTime: 0,
      retry: false,
    });
  const value = (
    query: { isPending: boolean; isFetching: boolean; isError: boolean },
    n?: number,
  ) =>
    query.isPending || query.isFetching
      ? "正在读取"
      : query.isError
        ? "暂不可用"
        : n;
  return (
    <section className={styles.memberMetrics} aria-label="成员目录摘要">
      <article>
        <span>正式成员</span>
        <strong>{value(summary, summary.data?.active)}</strong>
        <small>当前企业全部有效授权</small>
      </article>
      <article>
        <span>管理员</span>
        <strong>{value(summary, summary.data?.administrators)}</strong>
        <small>当前企业管理员角色授权</small>
      </article>
      <article>
        <span>邀请中</span>
        <strong>{value(invites, invites.data?.pending)}</strong>
        <small>尚未结束且未过期的正式邀请</small>
      </article>
      <article>
        <span>已停用</span>
        <strong>{value(summary, summary.data?.inactive)}</strong>
        <small>当前企业停用的授权</small>
      </article>
    </section>
  );
}
const names: Record<string, string> = {
  "product_sourcing.write": "链接采集商品",
  "local_agent.write": "使用商品智能助理",
  "listingkit.image_agent.read": "查看商品图片处理",
  "listingkit.image_agent.write": "生成商品图片",
  "workbench.store.read": "查看店铺",
  "workbench.store.create": "添加店铺",
  "workbench.store.update": "编辑店铺",
  "workbench.store.lifecycle": "店铺服务操作",
  "workbench.store.delete": "删除店铺",
  "workbench.source_account.read": "查看源账号",
  "workbench.source_account.manage": "管理源账号",
  "workbench.organization_member.read": "查看成员",
  "workbench.organization_member.manage": "管理成员",
  "workbench.commercial.read": "查看套餐与权益",
  "workbench.commercial.purchase": "购买套餐",
  "workbench.commercial.wallet_topup": "钱包充值",
};
export function PermissionScope({ permissions }: { permissions: string[] }) {
  return permissions.length ? (
    <details>
      <summary>{permissions.length} 项权限</summary>
      <ul>
        {permissions.map((permission) => (
          <li key={permission}>{names[permission] ?? "其他已授予权限"}</li>
        ))}
      </ul>
    </details>
  ) : (
    <span>无已授予权限</span>
  );
}
export function RecentMemberActivity({
  scope,
  userId,
}: {
  scope: MemberScope;
  userId: string;
}) {
  const query = useQuery({
    queryKey: [
      "member-activity",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      userId,
    ],
    queryFn: ({ signal }) =>
      getAccountAudit({ ...scope, actor: userId, limit: 1, signal }),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  return (
    <span title="仅统计本企业账户操作记录，不代表全站登录或使用时间">
      {query.isPending || query.isFetching ? (
        "正在读取"
      ) : query.isError ? (
        "暂不可用"
      ) : query.data.items.length ? (
        <time dateTime={query.data.items[0].time}>
          {new Date(query.data.items[0].time).toLocaleString("zh-CN")}
        </time>
      ) : (
        "暂无已记录操作"
      )}
    </span>
  );
}
