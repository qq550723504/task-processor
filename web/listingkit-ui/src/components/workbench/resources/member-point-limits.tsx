"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { getMemberAIPointLimits, MemberPointLimitError, setMemberAIPointLimit } from "@/lib/api/member-ai-point-limits";
import styles from "./resources.module.css";

type Props = { userId: string; organizationId: string; canManage: boolean; sequence: number };
type Command = { memberId: string; target: string; version: string; key: string };
const roles: Record<string, string> = { listingkit_admin: "管理员", listingkit_operator: "运营", listingkit_viewer: "只读成员" };

export function MemberPointLimits(props: Props) {
  return <ScopedPointLimits key={JSON.stringify([props.userId, props.organizationId, props.canManage])} {...props} />;
}

function ScopedPointLimits({ userId, organizationId, canManage, sequence }: Props) {
  const client = useQueryClient();
  const queryKey = ["workbench", userId, organizationId, "member-ai-point-limits"];
  const scope = { expectedUserId: userId, expectedOrganizationId: organizationId };
  const response = useQuery({ queryKey: [...queryKey, sequence], queryFn: ({ signal }) => getMemberAIPointLimits(scope, signal), gcTime: 0, staleTime: 0, retry: false });
  const [targets, setTargets] = useState<Record<string, string>>({});
  const [operation, setOperation] = useState<{ command: Command; unknown: boolean } | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const busy = useRef(false);
  async function write(command: Command) {
    if (busy.current || !canManage) return;
    busy.current = true; setOperation({ command, unknown: false }); setMessage(null);
    try {
      await setMemberAIPointLimit(scope, command.memberId, command.target, command.version, command.key);
      setOperation(null); setTargets(current => { const next = { ...current }; delete next[command.memberId]; return next; });
      await client.invalidateQueries({ queryKey });
    } catch (error) {
      if (error instanceof MemberPointLimitError && error.outcome === "unknown") {
        setOperation({ command, unknown: true });
        setMessage("保存结果未确认；不会自动重试或创建新操作。可核验原操作。");
      } else {
        setOperation(null);
        setMessage(error instanceof MemberPointLimitError && ["CONFLICT", "CONSUMED_FLOOR"].includes(error.code) ? "上限或已用额度已变化，请刷新后重新设置。" : "本次无法保存上限，请确认当前权限后刷新。");
      }
    } finally { busy.current = false; }
  }
  function save(memberId: string, target: string, version: string) {
    if (operation || !/^(0|[1-9][0-9]*)$/.test(target) || BigInt(target) > BigInt("9223372036854775807")) {
      setMessage("请输入有效的非负整数上限；未分配不自动填入额度。"); return;
    }
    void write({ memberId, target, version, key: crypto.randomUUID() });
  }
  const available = !response.isPending && !response.isFetching && !response.isError && response.data?.organizationId === organizationId;
  return <Card role="region" aria-label="成员 AI 点数月度上限" className={`${styles.panel} ${styles.pointPanel}`}>
    <div className={styles.heading}><div><h2>成员 AI 点数月度上限</h2><p>UTC 自然月，每月 1 日 00:00 切换；这是消费上限，不是成员余额。企业余额不按月清零，原月未知预留仍保留。</p></div><Button variant="outline" disabled={operation !== null && !operation.unknown} onClick={() => void client.invalidateQueries({ queryKey })}>刷新点数上限</Button></div>
    {message ? <p role="alert">{message}</p> : null}
    {!available ? <p role={response.isPending || response.isFetching ? "status" : "alert"}>{response.isPending || response.isFetching ? "正在读取当前成员点数上限…" : "无法读取成员点数上限；本次未确认数据，请刷新或检查权限。"}</p> :
      <div className={styles.memberTableWrap} tabIndex={0} role="region" aria-label="成员点数上限表格，可横向滚动"><table className={`${styles.memberTable} ${styles.pointTable}`}><thead><tr><th>成员 / 角色</th><th>AI 点数 / 月</th><th>当前月使用情况</th><th>操作</th></tr></thead><tbody>
        {response.data!.members.map(member => {
          const name = member.displayName || member.loginName || member.memberId;
          const target = targets[member.memberId] ?? (member.configured ? member.monthlyLimit : "");
          return <tr key={member.memberId}><td><strong>{name}</strong><small>{member.roles.map(role => roles[role] ?? role).join("、") || "角色未提供"}</small></td><td>{member.configured ? `${member.monthlyLimit} / 月` : "未分配，不能消费"}</td><td><span>已消费 {member.consumed} · 预留 {member.reserved} · 剩余 {member.remaining}</span><small>{member.monthStart.slice(0, 10)} — {member.monthEnd.slice(0, 10)} UTC</small></td><td>{canManage ? <div className={styles.allocationEdit}><label htmlFor={`point-limit-${member.memberId}`}>{name} AI 点数月度上限</label><input id={`point-limit-${member.memberId}`} inputMode="numeric" maxLength={19} placeholder="未分配" disabled={operation !== null} value={target} onChange={event => setTargets(current => ({ ...current, [member.memberId]: event.target.value }))} />{operation?.unknown && operation.command.memberId === member.memberId ? <Button onClick={() => void write(operation.command)}>核验原操作</Button> : <Button disabled={operation !== null || target === ""} onClick={() => save(member.memberId, target, member.version)}>{operation?.command.memberId === member.memberId ? "保存中…" : "保存上限"}</Button>}</div> : <span className={styles.readOnly}>只读</span>}</td></tr>;
        })}
        {!response.data!.members.length ? <tr><td colSpan={4} className={styles.emptyCell}>当前没有可配置的有效成员。</td></tr> : null}
      </tbody></table></div>}
  </Card>;
}
