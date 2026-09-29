"use client";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  getMemberResources,
  type MemberResourceEntry,
} from "@/lib/api/member-resources";
import { roles, name, live } from "./member-resource-view";
import { TransferDialog } from "./resource-transfer-dialog";
import { MemberStores } from "./member-store-dialog";
import styles from "./resources.module.css";
type Props = {
  userId: string;
  organizationId: string;
  sequence: number;
  canManage: boolean;
};
export function MemberResources(props: Props) {
  return (
    <ScopedMemberResources
      key={JSON.stringify([
        props.userId,
        props.organizationId,
        props.canManage,
      ])}
      {...props}
    />
  );
}
function ScopedMemberResources({
  userId,
  organizationId,
  sequence,
  canManage,
}: Props) {
  const client = useQueryClient();
  const scope = {
    expectedUserId: userId,
    expectedOrganizationId: organizationId,
  };
  const queryKey = ["workbench", userId, organizationId, "member-resources"];
  const response = useQuery({
    queryKey: [...queryKey, sequence],
    queryFn: ({ signal }) => getMemberResources(scope, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const [search, setSearch] = useState("");
  const [state, setState] = useState("all");
  const [type, setType] = useState("all");
  const [selected, setSelected] = useState<{
    member: MemberResourceEntry;
    action: "allocate" | "reclaim" | "stores";
  } | null>(null);
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey }),
      client.invalidateQueries({
        queryKey: ["workbench", "commercial-resources", userId, organizationId],
      }),
      client.invalidateQueries({
        queryKey: ["workbench", userId, organizationId, "account-store-count"],
      }),
    ]);
  };
  const available =
    !response.isPending &&
    !response.isFetching &&
    !response.isError &&
    response.data?.organizationId === organizationId;
  const members = available
    ? response.data!.members.filter(
        (member) =>
          (state === "all" ||
            (state === "active" ? live(member) : !live(member))) &&
          [name(member), member.loginName, member.memberId].some((value) =>
            value.toLowerCase().includes(search.toLowerCase()),
          ),
      )
    : [];
  return (
    <Card role="region" aria-label="成员资源目录" className={styles.panel}>
      <div className={styles.heading}>
        <div>
          <h2>成员资源目录</h2>
          <p>
            店铺按具体授权列出；续费期数和数据条数为实际余额。已离开成员的剩余资源保留，可由管理员回收。
          </p>
        </div>
        <Button variant="outline" onClick={() => void refresh()}>
          刷新成员资源
        </Button>
      </div>
      <div
        className={styles.memberFilters}
        role="group"
        aria-label="成员资源筛选"
      >
        <label>
          搜索成员
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="姓名、账号或成员编号"
          />
        </label>
        <label>
          资源类型
          <select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="all">全部资源</option>
            <option value="periods">续费期数</option>
            <option value="data">数据条数</option>
          </select>
        </label>
        <label>
          成员状态
          <select value={state} onChange={(e) => setState(e.target.value)}>
            <option value="all">全部成员</option>
            <option value="active">有效成员</option>
            <option value="inactive">已离开或停用</option>
          </select>
        </label>
      </div>
      {!available ? (
        <p
          role={response.isPending || response.isFetching ? "status" : "alert"}
        >
          {response.isPending || response.isFetching
            ? "正在读取成员资源…"
            : "本次未取得成员资源，请确认企业与权限后刷新。"}
        </p>
      ) : (
        <>
          <p className={styles.readOnly}>
            观察时间：
            {response.data!.observedAt.replace("T", " ").replace("Z", " UTC")}
          </p>
          <div
            className={styles.memberTableWrap}
            tabIndex={0}
            role="region"
            aria-label="成员资源表格，可横向滚动"
          >
            <table className={styles.memberTable}>
              <thead>
                <tr>
                  <th>成员</th>
                  <th>角色 / 状态</th>
                  <th>店铺</th>
                  {type !== "data" ? <th>续费期数</th> : null}
                  {type !== "periods" ? <th>数据额度</th> : null}
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {members.map((member) => (
                  <tr key={member.memberId}>
                    <td>
                      <strong>{name(member)}</strong>
                      <small>{member.loginName || member.memberId}</small>
                    </td>
                    <td>
                      {member.roles
                        .map((role) => roles[role] ?? role)
                        .join("、") || "未授予角色"}
                      <small>
                        {live(member) ? "有效成员" : "已离开或停用"}
                      </small>
                    </td>
                    <td>
                      {member.storeCount === null ? (
                        "店铺服务暂不可用"
                      ) : (
                        <Button
                          variant="outline"
                          onClick={() =>
                            setSelected({ member, action: "stores" })
                          }
                        >
                          {member.storeCount} 家
                        </Button>
                      )}
                    </td>
                    {type !== "data" ? (
                      <td>
                        <strong>{member.periods.free} 期</strong>
                        <small>
                          预留 {member.periods.reserved} · 已消费{" "}
                          {member.periods.consumed}
                        </small>
                      </td>
                    ) : null}
                    {type !== "periods" ? (
                      <td>
                        <strong>{member.dataRows.free} 条</strong>
                        <small>
                          预留 {member.dataRows.reserved} · 已消费{" "}
                          {member.dataRows.consumed}
                        </small>
                      </td>
                    ) : null}
                    <td>
                      {canManage ? (
                        <div className={styles.rowActions}>
                          <Button
                            disabled={!live(member)}
                            onClick={() =>
                              setSelected({ member, action: "allocate" })
                            }
                          >
                            分配资源
                          </Button>
                          <Button
                            variant="outline"
                            disabled={
                              member.periods.free === "0" &&
                              member.dataRows.free === "0"
                            }
                            onClick={() =>
                              setSelected({ member, action: "reclaim" })
                            }
                          >
                            回收资源
                          </Button>
                          {member.storeCount !== null ? (
                            <Button
                              variant="outline"
                              onClick={() =>
                                setSelected({ member, action: "stores" })
                              }
                            >
                              店铺授权
                            </Button>
                          ) : null}
                        </div>
                      ) : (
                        "只读"
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {members.length === 0 ? <p>当前筛选下没有成员资源记录。</p> : null}
          </div>
        </>
      )}
      {selected ? (
        selected.action === "stores" ? (
          <MemberStores
            member={selected.member}
            scope={scope}
            canManage={canManage}
            onClose={() => setSelected(null)}
            onChanged={refresh}
          />
        ) : (
          <TransferDialog
            member={selected.member}
            action={selected.action}
            scope={scope}
            onClose={() => setSelected(null)}
            onChanged={refresh}
          />
        )
      ) : null}
    </Card>
  );
}
