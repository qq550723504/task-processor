"use client";

import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import type { WorkbenchStore } from "@/lib/api/workbench-stores";
import {
  useDeleteWorkbenchStore,
  useDisableWorkbenchStore,
  useEnableWorkbenchStore,
  workbenchStoreKeys,
} from "@/lib/query/use-workbench-stores";

type Props = {
  store: WorkbenchStore;
  onStoreUpdated?: (store: WorkbenchStore) => void;
  onRefreshStore?: () => Promise<WorkbenchStore | null | undefined>;
  onDeleted?: () => void;
};

const updateRoles = new Set(["listingkit_operator", "listingkit_admin", "platform_admin"]);
const deleteRoles = new Set(["listingkit_admin", "platform_admin"]);

export function StoreLifecycleActions({ store, onStoreUpdated, onRefreshStore, onDeleted }: Props) {
  const context = useWorkbenchContext();
  const queryClient = useQueryClient();
  const enable = useEnableWorkbenchStore();
  const disable = useDisableWorkbenchStore();
  const remove = useDeleteWorkbenchStore();
  const [dialogTarget, setDialogTarget] = useState<string | null>(null);
  const [state, setState] = useState<"idle" | "pending" | "conflict" | "refreshing" | "revoked">("idle");
  const actionRef = useRef(false);
  const organization = context.effectiveOrganization;
  const organizationId = organization?.id ?? "";
  const organizationName = organization?.name ?? "当前企业";
  const phrase = `删除 ${organizationName} 的店铺 ${store.name}`;
  const confirmationTarget = `${organizationId}:${store.id}:${store.name}`;
  const canUpdate = context.roles.some((role) => updateRoles.has(role));
  const canDelete = context.roles.some((role) => deleteRoles.has(role));
  const isBusy = state === "pending" || state === "refreshing" || enable.isPending || disable.isPending || remove.isPending;

  const resetAction = () => {
    actionRef.current = false;
    setState("idle");
  };
  const revoke = () => {
    actionRef.current = false;
    queryClient.removeQueries({ queryKey: workbenchStoreKeys.root(organizationId) });
    context.retry();
    setState("revoked");
  };
  const handleActionError = (error: { code?: string }) => {
    if (error.code === "ORGANIZATION_ACCESS_REVOKED") {
      revoke();
      return;
    }
    if (error.code === "STORE_VERSION_CONFLICT") {
      actionRef.current = false;
      setState("conflict");
      return;
    }
    resetAction();
  };
  const runStateAction = (action: "enable" | "disable") => {
    if (actionRef.current || isBusy || state !== "idle") return;
    actionRef.current = true;
    setState("pending");
    const mutation = action === "enable" ? enable : disable;
    mutation.mutate(
      { id: store.id, version: store.version },
      {
        onSuccess: (next) => {
          actionRef.current = false;
          setState("idle");
          onStoreUpdated?.(next);
        },
        onError: handleActionError,
      },
    );
  };
  const refreshAfterConflict = () => {
    if (!onRefreshStore || actionRef.current || state !== "conflict") return;
    actionRef.current = true;
    setState("refreshing");
    void onRefreshStore().then(
      (latest) => {
        actionRef.current = false;
        if (!latest) {
          setState("conflict");
          return;
        }
        onStoreUpdated?.(latest);
        setState("idle");
      },
      () => {
        actionRef.current = false;
        setState("conflict");
      },
    );
  };
  const finishDelete = () => {
    actionRef.current = false;
    setDialogTarget(null);
    setState("idle");
    onDeleted?.();
  };
  const deleteErrorHandler = (error: { code?: string }, onInterrupted: () => void) => {
    if (error.code === "ORGANIZATION_ACCESS_REVOKED") {
      revoke();
      return;
    }
    actionRef.current = false;
    setState("idle");
    onInterrupted();
  };
  const submitDelete = (onInterrupted: () => void) => {
    if (actionRef.current || isBusy) return;
    actionRef.current = true;
    setState("pending");
    remove.mutate(
      { id: store.id, version: store.version },
      { onSuccess: finishDelete, onError: (error) => deleteErrorHandler(error, onInterrupted) },
    );
  };
  const retryDelete = (onInterrupted: () => void) => {
    if (actionRef.current || isBusy || !remove.canRetryLast) return;
    actionRef.current = true;
    setState("pending");
    void Promise.resolve(remove.retryLast()).then(finishDelete, (error) => deleteErrorHandler(error, onInterrupted));
  };

  if (state === "revoked") {
    return <p className="rounded-md border border-destructive/30 p-3 text-sm text-destructive" role="alert">当前企业访问已被撤销，正在重新验证企业访问。</p>;
  }
  if (store.lifecycleStatus === "deleting") {
    return <div className="space-y-2"><p className="text-sm text-muted-foreground">删除正在进行中，暂不能编辑或更改店铺状态。</p>{remove.canRetryLast ? <Button disabled={isBusy} onClick={() => retryDelete(() => undefined)} size="sm" variant="outline">重试删除</Button> : <p className="text-sm text-muted-foreground">此删除请求没有可安全复用的重试凭据，请刷新页面或联系支持。</p>}</div>;
  }
  if (store.lifecycleStatus !== "active" && store.lifecycleStatus !== "disabled") return null;
  if (!canUpdate && !canDelete) return null;
  if (state === "conflict" || state === "refreshing") {
    return <div className="space-y-2" role="alert"><p>店铺信息已变化，请刷新店铺信息后再操作。</p>{onRefreshStore ? <Button disabled={state === "refreshing"} onClick={refreshAfterConflict} size="sm" variant="outline">刷新店铺信息</Button> : null}</div>;
  }
  return (
    <div className="flex flex-wrap gap-2">
      {canUpdate ? <Button disabled={isBusy} onClick={() => runStateAction(store.lifecycleStatus === "active" ? "disable" : "enable")} size="sm" variant="outline">{store.lifecycleStatus === "active" ? "停用店铺" : "重新启用店铺"}</Button> : null}
      {canDelete ? <Button disabled={isBusy} onClick={() => setDialogTarget(confirmationTarget)} size="sm" variant="destructive">删除店铺</Button> : null}
      {dialogTarget === confirmationTarget ? <DeleteConfirmation key={confirmationTarget} onCancel={() => setDialogTarget(null)} onConfirm={submitDelete} pending={isBusy} phrase={phrase} retryAvailable={remove.canRetryLast} onRetry={retryDelete} /> : null}
    </div>
  );
}

function DeleteConfirmation({ onCancel, onConfirm, pending, phrase, retryAvailable, onRetry }: { onCancel: () => void; onConfirm: (onInterrupted: () => void) => void; pending: boolean; phrase: string; retryAvailable: boolean; onRetry: (onInterrupted: () => void) => void }) {
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState(false);
  const markInterrupted = () => setError(true);
  return <section aria-describedby="store-delete-description" aria-labelledby="store-delete-title" className="w-full rounded-lg border border-destructive/30 p-4" role="alertdialog"><h2 className="font-semibold" id="store-delete-title">确认删除店铺</h2><p className="mt-2 text-sm text-muted-foreground" id="store-delete-description">此操作不会在界面中提供恢复。请输入：<strong className="block break-all">{phrase}</strong></p><label className="mt-3 grid gap-1 text-sm font-medium" htmlFor="store-delete-confirmation">确认删除文本<input className="h-10 rounded-md border bg-background px-3" id="store-delete-confirmation" onChange={(event) => setConfirmation(event.target.value)} value={confirmation} /></label>{error ? <p className="mt-3 text-sm text-destructive" role="alert">删除请求未完成，请仅在可用时重试。</p> : null}<div className="mt-4 flex flex-wrap gap-2"><Button disabled={pending} onClick={onCancel} variant="outline">取消</Button><Button disabled={pending || confirmation !== phrase} onClick={() => onConfirm(markInterrupted)} variant="destructive">确认删除</Button>{error && retryAvailable ? <Button disabled={pending} onClick={() => onRetry(markInterrupted)} variant="outline">重试删除</Button> : null}</div></section>;
}
