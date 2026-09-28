"use client";
import Link from "next/link";
import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  getMemberStores,
  getMemberStoreGrant,
  setMemberStoreGrant,
  MemberResourceError,
  type MemberResourceScope,
  type MemberResourceEntry,
  type MemberGrantInput,
} from "@/lib/api/member-resources";
import { listWorkbenchStores } from "@/lib/api/workbench-stores";
import { ResourceDialog } from "./resource-dialog";
import { name, live, failure } from "./member-resource-view";
import styles from "./resources.module.css";
type GrantCommand = { storeId: string; input: MemberGrantInput; key: string };
export function MemberStores({
  member,
  scope,
  canManage,
  onClose,
  onChanged,
}: {
  member: MemberResourceEntry;
  scope: MemberResourceScope;
  canManage: boolean;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [availablePage, setAvailablePage] = useState(1);
  const [storeId, setStoreId] = useState("");
  const [message, setMessage] = useState("");
  const [operation, setOperation] = useState<{
    command: GrantCommand;
    unknown: boolean;
  } | null>(null);
  const busy = useRef(false);
  const prefix = [
    "workbench",
    scope.expectedUserId,
    scope.expectedOrganizationId,
    "member-stores",
    member.memberId,
  ];
  const assigned = useQuery({
    queryKey: [...prefix, page],
    queryFn: ({ signal }) =>
      getMemberStores(scope, member.memberId, page, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const available = useQuery({
    queryKey: [...prefix, "available", availablePage],
    queryFn: ({ signal }) =>
      listWorkbenchStores(
        { page: availablePage, pageSize: 20 },
        scope.expectedOrganizationId,
        signal,
      ),
    enabled: canManage && live(member),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const grant = useQuery({
    queryKey: [...prefix, "grant", storeId],
    queryFn: ({ signal }) =>
      getMemberStoreGrant(scope, member.memberId, storeId, signal),
    enabled: !!storeId && canManage,
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  async function write(command: GrantCommand) {
    if (busy.current) return;
    busy.current = true;
    setOperation({ command, unknown: false });
    setMessage("");
    try {
      await setMemberStoreGrant(
        scope,
        member.memberId,
        command.storeId,
        command.input,
        command.key,
      );
      setOperation(null);
      await Promise.all([
        client.invalidateQueries({ queryKey: prefix }),
        onChanged(),
      ]);
      setMessage("店铺授权已保存。");
    } catch (error) {
      if (error instanceof MemberResourceError && error.outcome === "unknown") {
        setOperation({ command, unknown: true });
        setMessage("操作结果未确认，请核验原操作。");
      } else {
        setOperation(null);
        setMessage(failure(error));
      }
    } finally {
      busy.current = false;
    }
  }
  const list =
    !assigned.isFetching && !assigned.isError ? assigned.data : undefined;
  return (
    <ResourceDialog
      title="成员店铺授权"
      onClose={onClose}
      locked={operation !== null}
    >
      <p>{name(member)} · 仅列出当前明确授权的店铺。</p>
      {list ? (
        <>
          <ul className={styles.assignedStores}>
            {list.items.map((store) => (
              <li key={store.id}>
                <Link href={`/workbench/stores/${store.id}`} prefetch={false}>
                  {store.name}
                </Link>
                <small>
                  {store.serviceExpiresAt
                    ? `服务到期 ${store.serviceExpiresAt}`
                    : "尚未开通服务"}
                </small>
                {canManage ? (
                  <Button
                    variant="outline"
                    disabled={operation !== null}
                    onClick={() =>
                      void write({
                        storeId: store.id,
                        input: {
                          active: false,
                          expectedVersion: store.grantVersion,
                        },
                        key: crypto.randomUUID(),
                      })
                    }
                  >
                    撤销 {store.name} 授权
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>
          {!list.items.length ? <p>该成员当前没有已授权店铺。</p> : null}
          <div className={styles.rowActions}>
            <Button
              variant="outline"
              disabled={page === 1 || operation !== null}
              onClick={() => setPage((v) => v - 1)}
            >
              上一页
            </Button>
            <span>
              第 {page} 页 · 共 {list.total} 家
            </span>
            <Button
              variant="outline"
              disabled={
                BigInt(page * 20) >= BigInt(list.total) || operation !== null
              }
              onClick={() => setPage((v) => v + 1)}
            >
              下一页
            </Button>
          </div>
        </>
      ) : (
        <p
          role={assigned.isPending || assigned.isFetching ? "status" : "alert"}
        >
          {assigned.isPending || assigned.isFetching
            ? "正在读取授权店铺…"
            : "本次未取得店铺授权，请刷新后重试。"}
        </p>
      )}
      {canManage && live(member) ? (
        <section className={styles.allocationSection}>
          <h3>授权具体店铺</h3>
          <label className={styles.dialogField}>
            选择店铺
            <select
              value={storeId}
              disabled={operation !== null || available.isFetching}
              onChange={(e) => setStoreId(e.target.value)}
            >
              <option value="">请选择</option>
              {!available.isFetching && !available.isError
                ? available.data?.items.map((store) => (
                    <option key={store.id} value={store.id}>
                      {store.name}
                    </option>
                  ))
                : null}
            </select>
          </label>
          <div className={styles.rowActions}>
            <Button
              variant="outline"
              disabled={availablePage === 1 || operation !== null}
              onClick={() => {
                setStoreId("");
                setAvailablePage((v) => v - 1);
              }}
            >
              前一组
            </Button>
            <span>第 {availablePage} 组</span>
            <Button
              variant="outline"
              disabled={
                !available.data ||
                availablePage * 20 >= available.data.pagination.total ||
                operation !== null
              }
              onClick={() => {
                setStoreId("");
                setAvailablePage((v) => v + 1);
              }}
            >
              后一组
            </Button>
          </div>
          <Button
            disabled={
              !storeId ||
              grant.isPending ||
              grant.isFetching ||
              grant.isError ||
              !grant.data ||
              grant.data.active ||
              operation !== null
            }
            onClick={() =>
              grant.data &&
              void write({
                storeId,
                input: { active: true, expectedVersion: grant.data.version },
                key: crypto.randomUUID(),
              })
            }
          >
            {grant.data?.active ? "已授权" : "确认授权"}
          </Button>
          <p>授权允许成员管理这一家店铺，不会开通服务或代替 SHEIN 官方连接。</p>
        </section>
      ) : null}
      {message ? <p role="alert">{message}</p> : null}
      {operation?.unknown ? (
        <Button onClick={() => void write(operation.command)}>
          核验原操作
        </Button>
      ) : null}
      <div className={styles.dialogFooter}>
        <Button
          variant="outline"
          disabled={operation !== null}
          onClick={onClose}
        >
          关闭授权窗口
        </Button>
        <Button
          variant="outline"
          disabled={operation !== null && !operation.unknown}
          onClick={() => void client.invalidateQueries({ queryKey: prefix })}
        >
          刷新店铺授权
        </Button>
      </div>
    </ResourceDialog>
  );
}
