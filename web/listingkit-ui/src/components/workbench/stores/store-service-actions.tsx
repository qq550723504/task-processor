"use client";
import { useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { ResourceDialog } from "../resources/resource-dialog";
import {
  servicePendingSchema,
  useResourcePending,
} from "../resources/resource-pending";
import { getCommercialResources } from "@/lib/api/commercial-billing";
import {
  getMemberResources,
  type MemberResourceScope,
} from "@/lib/api/member-resources";
import {
  activateWorkbenchStoreService,
  renewWorkbenchStoreService,
  reactivateWorkbenchStoreService,
  WorkbenchAPIError,
  type WorkbenchStore,
} from "@/lib/api/workbench-stores";
import styles from "../resources/resources.module.css";
type Command = {
  action: "activate" | "renew" | "reactivate";
  periods: number;
  version: number;
  key: string;
};
export function StoreServiceActions({
  store,
  scope,
  administrator,
  canWrite,
  connected,
  onChanged,
}: {
  store: WorkbenchStore;
  scope: MemberResourceScope;
  administrator: boolean;
  canWrite: boolean;
  connected: boolean;
  onChanged: () => Promise<unknown>;
}) {
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [periods, setPeriods] = useState("1");
  const pending = useResourcePending(
    scope,
    ["service", store.id],
    servicePendingSchema,
  );
  const [currentOperation, setOperation] = useState<{
    command: Command;
    unknown: boolean;
  } | null>(null);
  const operation =
    currentOperation ??
    (pending.command ? { command: pending.command, unknown: true } : null);
  const [message, setMessage] = useState("");
  const busy = useRef(false);
  const balance = useQuery({
    queryKey: [
      "workbench",
      "commercial-resources",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      "store-service",
    ],
    queryFn: ({ signal }) =>
      getCommercialResources(
        scope.expectedUserId,
        scope.expectedOrganizationId,
        signal,
      ),
    enabled: open && administrator,
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  const members = useQuery({
    queryKey: [
      "workbench",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      "member-resources",
      "store-service",
    ],
    queryFn: ({ signal }) => getMemberResources(scope, signal),
    enabled: open && !administrator,
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  const resource =
    !balance.isFetching && !balance.isError
      ? balance.data?.resources.find(
          (v) => v.resource_type === "store_renewal_period",
        )
      : undefined;
  const own =
    !members.isFetching && !members.isError
      ? members.data?.members.find(
          (member) => member.userId === scope.expectedUserId,
        )
      : undefined;
  const available = administrator
    ? resource?.state === "recorded"
      ? resource.available
      : resource?.state === "not_recorded"
        ? "0"
        : null
    : (own?.periods.free ?? null);
  const action =
    store.serviceStatus === "active"
      ? "renew"
      : store.serviceStatus === "expired" || store.serviceStatus === "suspended"
        ? "reactivate"
        : "activate";
  const label =
    action === "activate"
      ? "开通服务"
      : action === "renew"
        ? "续费服务"
        : "恢复服务";
  const valid =
    pending.ready &&
    /^[1-9][0-9]?$/.test(periods) &&
    Number(periods) <= 12 &&
    available !== null &&
    BigInt(periods) <= BigInt(available);
  async function write(command: Command) {
    if (busy.current) return;
    const recovering = pending.command !== null;
    try {
      pending.persist(command);
    } catch {
      setMessage("无法保存原操作，请先恢复浏览器存储；本次没有发送新命令。");
      return;
    }
    busy.current = true;
    setOperation({ command, unknown: false });
    setMessage("");
    try {
      const args = [
        store.id,
        command.version,
        command.key,
        scope.expectedOrganizationId,
        scope.expectedUserId,
      ] as const;
      const result =
        command.action === "activate"
          ? await activateWorkbenchStoreService(...args)
          : command.action === "renew"
            ? await renewWorkbenchStoreService(
                store.id,
                command.periods,
                ...(args.slice(1) as [number, string, string, string]),
              )
            : await reactivateWorkbenchStoreService(
                store.id,
                command.periods,
                ...(args.slice(1) as [number, string, string, string]),
              );
      pending.clear(command);
      setOperation(null);
      setMessage(
        `服务已更新：到期 ${result.serviceExpiresAt}，本次扣 ${result.quantity} 期，原资金池余额 ${result.resourceBalanceAfter} 期。`,
      );
      await Promise.all([
        onChanged(),
        client.invalidateQueries({
          queryKey: [
            "workbench",
            "commercial-resources",
            scope.expectedUserId,
            scope.expectedOrganizationId,
          ],
        }),
        client.invalidateQueries({
          queryKey: [
            "workbench",
            scope.expectedUserId,
            scope.expectedOrganizationId,
            "member-resources",
          ],
        }),
      ]);
      setOpen(false);
    } catch (error) {
      const unknown =
        error instanceof WorkbenchAPIError &&
        (error.status === 0 ||
          error.status >= 500 ||
          error.code === "INVALID_WORKBENCH_RESPONSE");
      if (recovering || unknown) {
        setOperation({ command, unknown: true });
        setMessage("续费结果未确认；预留保持，不能另建操作。请核验原操作。");
      } else {
        try {
          pending.clear(command);
          setOperation(null);
        } catch {
          setOperation({ command, unknown: true });
        }
        setMessage(
          error instanceof WorkbenchAPIError &&
            error.code === "RESOURCE_INSUFFICIENT_BALANCE"
            ? "可用续费期数不足。"
            : error instanceof WorkbenchAPIError &&
                error.code === "STORE_CONNECTION_NOT_CONNECTED"
              ? "开通前必须完成真实 SHEIN 官方连接。"
              : "服务操作未完成，请刷新店铺并确认权限后重试。",
        );
      }
    } finally {
      busy.current = false;
    }
  }
  return (
    <section className="mt-4 space-y-3" aria-label="店铺服务续费">
      <p className="text-sm text-muted-foreground">
        {administrator
          ? "管理员代续费从企业未分配期数扣减。"
          : "成员续费从本人已分配期数扣减。"}
        1 期为 30 天，首次开通需要真实平台连接。
      </p>
      {canWrite &&
      store.recordStatus !== "deleting" ? (
        <Button
          disabled={action === "activate" && !connected}
          onClick={() => {
            setOpen(true);
            setMessage("");
          }}
        >
          {label}
        </Button>
      ) : null}
      {message && !open ? <p role="status">{message}</p> : null}
      {open ? (
        <ResourceDialog
          title={label}
          onClose={() => setOpen(false)}
          locked={operation !== null}
        >
          <p>
            {store.name} · {administrator ? "企业未分配期数" : "本人已分配期数"}
          </p>
          <p>可用 {available ?? "正在读取"} 期</p>
          <label className={styles.dialogField}>
            本次续费期数
            <input
              value={action === "activate" ? "1" : periods}
              disabled={action === "activate" || operation !== null}
              inputMode="numeric"
              maxLength={2}
              onChange={(e) => setPeriods(e.target.value)}
            />
            <small>30 天 / 期；单次最多 12 期。</small>
          </label>
          <p>
            已有效的服务从当前到期时间延长；到期后从本次开通时间计算。续费不会自动授权平台发布。
          </p>
          {message ? <p role="alert">{message}</p> : null}
          {pending.error ? (
            <p role="alert">
              原操作记录不可读取，已暂停新操作。请恢复浏览器存储后重试。
            </p>
          ) : null}
          <div className={styles.dialogFooter}>
            <Button
              variant="outline"
              disabled={operation !== null}
              onClick={() => setOpen(false)}
            >
              取消
            </Button>
            {operation?.unknown ? (
              <Button onClick={() => void write(operation.command)}>
                核验原操作
              </Button>
            ) : (
              <Button
                disabled={!valid || operation !== null}
                onClick={() =>
                  void write({
                    action,
                    periods: action === "activate" ? 1 : Number(periods),
                    version: store.version,
                    key: crypto.randomUUID(),
                  })
                }
              >
                确认{label}
              </Button>
            )}
          </div>
        </ResourceDialog>
      ) : null}
    </section>
  );
}
