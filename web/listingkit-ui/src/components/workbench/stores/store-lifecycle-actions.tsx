"use client";

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import type { WorkbenchStore } from "@/lib/api/workbench-stores";
import { canCreateWorkbenchStore, canDeleteWorkbenchStore, canUpdateWorkbenchStore } from "@/lib/workbench/permissions";
import {
	useDeleteWorkbenchStore,
	useDisableWorkbenchStore,
	useEnableWorkbenchStore,
	useResumeWorkbenchStore,
	workbenchStoreKeys,
} from "@/lib/query/use-workbench-stores";

type Props = {
  store: WorkbenchStore;
  onStoreUpdated?: (store: WorkbenchStore) => void;
  onRefreshStore?: () => Promise<WorkbenchStore | null | undefined>;
  onDeleted?: () => void;
};

type ConflictSource = "lifecycle" | "delete";
type ActionState =
  | { kind: "idle"; scopeKey: string }
  | { kind: "pending"; scopeKey: string }
  | {
      kind: "conflict" | "refreshing";
      scopeKey: string;
      source: ConflictSource;
      baselineVersion: number;
    }
  | {
      kind: "delete-interrupted";
      scopeKey: string;
      baselineVersion: number;
      retryable: boolean;
      refreshing: boolean;
    }
  | { kind: "lifecycle-error"; scopeKey: string; message: string }
  | { kind: "revoked"; scopeKey: string };

type DeleteTerminal = "retryable" | "semantic" | null;

export function StoreLifecycleActions({
  store,
  onStoreUpdated,
  onRefreshStore,
  onDeleted,
}: Props) {
  const context = useWorkbenchContext();
  const organization = context.effectiveOrganization;
  const organizationId = organization?.id ?? "";
  const organizationName = organization?.name ?? "当前企业";
  const canUpdate = canUpdateWorkbenchStore(context.roles);
  const canDelete = Boolean(
    organizationId && canDeleteWorkbenchStore(context.roles),
  );
  const scopeKey = [organizationId, store.id].join("\u0000");
  const confirmationScopeKey = [
    scopeKey,
    organizationName,
    [...context.roles].sort().join(","),
    store.name,
    store.version,
  ].join("\u0000");
  return (
    <ScopedStoreLifecycleActions
      canDelete={canDelete}
      canCreate={canCreateWorkbenchStore(context.roles)}
      canUpdate={canUpdate}
      confirmationScopeKey={confirmationScopeKey}
      context={context}
      key={scopeKey}
      onDeleted={onDeleted}
      onRefreshStore={onRefreshStore}
      onStoreUpdated={onStoreUpdated}
      organizationId={organizationId}
      organizationName={organizationName}
      scopeKey={scopeKey}
      store={store}
    />
  );
}

function ScopedStoreLifecycleActions({
  canDelete,
  canCreate,
  canUpdate,
  confirmationScopeKey,
  context,
  organizationId,
  organizationName,
  scopeKey,
  store,
  onStoreUpdated,
  onRefreshStore,
  onDeleted,
}: Props & {
  canDelete: boolean;
  canCreate: boolean;
  canUpdate: boolean;
  confirmationScopeKey: string;
  context: ReturnType<typeof useWorkbenchContext>;
  organizationId: string;
  organizationName: string;
  scopeKey: string;
}) {
  const queryClient = useQueryClient();
  const enable = useEnableWorkbenchStore();
  const disable = useDisableWorkbenchStore();
  const remove = useDeleteWorkbenchStore();
  const resumeCreate = useResumeWorkbenchStore();
  const phrase = `删除 ${organizationName} 的店铺 ${store.name}`;
  const [action, setAction] = useState<ActionState>({
    kind: "idle",
    scopeKey,
  });
  const [deleteCompletionToken, setDeleteCompletionToken] = useState(0);
  const actionRef = useRef(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const currentAction = action;
  const isBusy =
    currentAction.kind === "pending" ||
    currentAction.kind === "refreshing" ||
    (currentAction.kind === "delete-interrupted" && currentAction.refreshing) ||
    enable.isPending ||
    disable.isPending ||
    remove.isPending ||
    resumeCreate.isPending;

  const setIdle = (operationScope = scopeKey) => {
    actionRef.current = false;
    if (mountedRef.current) {
      setAction({ kind: "idle", scopeKey: operationScope });
    }
  };
  const revoke = (operationScope: string, operationOrganizationId: string) => {
    actionRef.current = false;
    queryClient.removeQueries({
      queryKey: workbenchStoreKeys.root(operationOrganizationId),
    });
    context.retry();
    if (mountedRef.current) {
      setAction({ kind: "revoked", scopeKey: operationScope });
    }
  };
  const beginConflict = (
    source: ConflictSource,
    baselineVersion: number,
    operationScope: string,
  ) => {
    actionRef.current = false;
    if (!mountedRef.current) return;
    setAction({
      kind: "conflict",
      scopeKey: operationScope,
      source,
      baselineVersion,
    });
  };
  const handleLifecycleError = (
    error: { code?: string },
    baselineVersion: number,
    operationScope: string,
    operationOrganizationId: string,
  ) => {
    if (isOrganizationAccessError(error.code)) {
      revoke(operationScope, operationOrganizationId);
      return;
    }
    if (error.code === "STORE_VERSION_CONFLICT") {
      beginConflict("lifecycle", baselineVersion, operationScope);
      return;
    }
    actionRef.current = false;
    if (!mountedRef.current) return;
    setAction({
      kind: "lifecycle-error",
      scopeKey: operationScope,
      message: lifecycleFailureMessage(error.code),
    });
  };
  const runStateAction = (nextAction: "enable" | "disable") => {
    if (
      actionRef.current ||
      isBusy ||
      currentAction.kind !== "idle" ||
      !canUpdate
    ) {
      return;
    }
    const operationScope = scopeKey;
    const operationOrganizationId = organizationId;
    const baselineVersion = store.version;
    actionRef.current = true;
    setAction({ kind: "pending", scopeKey: operationScope });
    const mutation = nextAction === "enable" ? enable : disable;
    mutation.mutate(
      { id: store.id, version: baselineVersion },
      {
        onSuccess: (next) => {
          setIdle(operationScope);
          if (mountedRef.current && organizationId === operationOrganizationId) {
            onStoreUpdated?.(next);
          }
        },
        onError: (error) =>
          handleLifecycleError(
            error,
            baselineVersion,
            operationScope,
            operationOrganizationId,
          ),
      },
    );
  };
  const resumeCreateAction = () => {
    if (
      actionRef.current ||
      isBusy ||
      currentAction.kind !== "idle" ||
      !canCreate
    ) {
      return;
    }
    const operationScope = scopeKey;
    const operationOrganizationId = organizationId;
    const baselineVersion = store.version;
    actionRef.current = true;
    setAction({ kind: "pending", scopeKey: operationScope });
    resumeCreate.mutate(
      { id: store.id, version: baselineVersion },
      {
        onSuccess: (next) => {
          setIdle(operationScope);
          if (mountedRef.current && organizationId === operationOrganizationId) {
            onStoreUpdated?.(next);
          }
        },
        onError: (error) =>
          handleLifecycleError(
            error,
            baselineVersion,
            operationScope,
            operationOrganizationId,
          ),
      },
    );
  };
  const refreshAfterConflict = () => {
    if (
      !onRefreshStore ||
      actionRef.current ||
      currentAction.kind !== "conflict"
    ) {
      return;
    }
    const conflict = currentAction;
    const operationScope = scopeKey;
    const operationOrganizationId = organizationId;
    actionRef.current = true;
    setAction({ ...conflict, kind: "refreshing" });
    void onRefreshStore().then(
      (latest) => {
        actionRef.current = false;
        if (!mountedRef.current || organizationId !== operationOrganizationId) return;
        const minimumVersion = Math.max(
          conflict.baselineVersion,
          store.version,
        );
        if (!latest || latest.version <= minimumVersion) {
          setAction({ ...conflict, kind: "conflict" });
          return;
        }
        setAction({ kind: "idle", scopeKey: operationScope });
        onStoreUpdated?.(latest);
      },
      () => {
        actionRef.current = false;
        if (!mountedRef.current || organizationId !== operationOrganizationId) return;
        setAction({ ...conflict, kind: "conflict" });
      },
    );
  };
  const finishDelete = (operationScope: string, operationOrganizationId: string) => {
    actionRef.current = false;
    if (!mountedRef.current || organizationId !== operationOrganizationId) return;
    setDeleteCompletionToken((token) => token + 1);
    setAction({ kind: "idle", scopeKey: operationScope });
    onDeleted?.();
  };
  const finishInterruptedRefresh = (
    latest: WorkbenchStore | null | undefined,
    operationScope: string,
    operationOrganizationId: string,
    baselineVersion: number,
  ) => {
    if (!mountedRef.current || organizationId !== operationOrganizationId) return;
    const minimumVersion = Math.max(
      baselineVersion,
      store.version,
    );
    setAction({
      kind: "delete-interrupted",
      scopeKey: operationScope,
      baselineVersion,
      retryable: true,
      refreshing: false,
    });
    if (
      latest &&
      latest.version > minimumVersion &&
      latest.lifecycleStatus === "deleting"
    ) {
      onStoreUpdated?.(latest);
    }
  };
  const handleDeleteError = (
    error: { code?: string; status?: number },
    baselineVersion: number,
    operationScope: string,
    operationOrganizationId: string,
  ) => {
    if (isOrganizationAccessError(error.code)) {
      revoke(operationScope, operationOrganizationId);
      return;
    }
    if (error.code === "STORE_NOT_FOUND") {
      finishDelete(operationScope, operationOrganizationId);
      return;
    }
    if (error.code === "STORE_VERSION_CONFLICT") {
      beginConflict("delete", baselineVersion, operationScope);
      return;
    }
    actionRef.current = false;
    if (!mountedRef.current) return;
    const retryable = error.status === 0 || (error.status ?? 0) >= 500;
    if (!retryable) {
      setAction({
        kind: "delete-interrupted",
        scopeKey: operationScope,
        baselineVersion,
        retryable: false,
        refreshing: false,
      });
      return;
    }
    setAction({
      kind: "delete-interrupted",
      scopeKey: operationScope,
      baselineVersion,
      retryable: true,
      refreshing: Boolean(onRefreshStore),
    });
    if (!onRefreshStore) return;
    void onRefreshStore().then(
      (latest) =>
        finishInterruptedRefresh(
          latest,
          operationScope,
          operationOrganizationId,
          baselineVersion,
        ),
      () =>
        finishInterruptedRefresh(
          null,
          operationScope,
          operationOrganizationId,
          baselineVersion,
        ),
    );
  };
  const submitDelete = () => {
    if (
      actionRef.current ||
      isBusy ||
      currentAction.kind !== "idle" ||
      !canDelete
    ) {
      return;
    }
    const operationScope = scopeKey;
    const operationOrganizationId = organizationId;
    const baselineVersion = store.version;
    actionRef.current = true;
    setAction({ kind: "pending", scopeKey: operationScope });
    remove.mutate(
      { id: store.id, version: baselineVersion },
      {
        onSuccess: () => finishDelete(operationScope, operationOrganizationId),
        onError: (error) =>
          handleDeleteError(
            error,
            baselineVersion,
            operationScope,
            operationOrganizationId,
          ),
      },
    );
  };
  const retryDelete = () => {
    const deletingStore = store.lifecycleStatus === "deleting";
    const interruptedRetry =
      currentAction.kind === "delete-interrupted" &&
      currentAction.retryable &&
      !currentAction.refreshing;
    if (
      actionRef.current ||
      isBusy ||
      !canDelete ||
      (!deletingStore && (!interruptedRetry || !remove.canRetryLast))
    ) {
      return;
    }
    const operationScope = scopeKey;
    const operationOrganizationId = organizationId;
    const baselineVersion = store.version;
      actionRef.current = true;
    setAction({ kind: "pending", scopeKey: operationScope });
    const recovery = remove.canRetryLast
      ? remove.retryLast()
      : remove.resume({ id: store.id, version: baselineVersion });
    void Promise.resolve(recovery).then(
      () => finishDelete(operationScope, operationOrganizationId),
      (error) =>
        handleDeleteError(
          error,
          baselineVersion,
          operationScope,
          operationOrganizationId,
        ),
    );
  };

  if (currentAction.kind === "revoked") {
    return (
      <p
        className="rounded-md border border-destructive/30 p-3 text-sm text-destructive"
        role="alert"
      >
        当前企业访问已不可用，正在重新验证企业访问。
      </p>
    );
  }
  if (currentAction.kind === "lifecycle-error") {
    return (
      <p
        className="rounded-md border border-destructive/30 p-3 text-sm text-destructive"
        role="alert"
      >
        {currentAction.message}
      </p>
    );
  }
  if (
    currentAction.kind === "conflict" ||
    currentAction.kind === "refreshing"
  ) {
    return (
      <div className="space-y-2" role="alert">
        <p>店铺信息已变化，请刷新店铺信息后再操作。</p>
        {onRefreshStore ? (
          <Button
            disabled={currentAction.kind === "refreshing"}
            onClick={refreshAfterConflict}
            size="sm"
            variant="outline"
          >
            刷新店铺信息
          </Button>
        ) : null}
      </div>
    );
  }
  if (store.lifecycleStatus === "provisioning") {
    return (
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">
          店铺创建尚未完成，暂不能编辑或更改店铺状态。
        </p>
        {canCreate && currentAction.kind === "idle" ? (
          <Button
            disabled={isBusy}
            onClick={resumeCreateAction}
            size="sm"
            variant="outline"
          >
            恢复创建
          </Button>
        ) : null}
      </div>
    );
  }
  if (store.lifecycleStatus === "deleting") {
    const deleteRecoveryAvailable =
      currentAction.kind === "idle" ||
      (currentAction.kind === "delete-interrupted" &&
        currentAction.retryable &&
        !currentAction.refreshing);
    return (
      <div className="space-y-2">
        <p className="text-sm text-muted-foreground">
          删除正在进行中，暂不能编辑或更改店铺状态。
        </p>
        {canDelete && deleteRecoveryAvailable ? (
          <Button
            disabled={isBusy}
            onClick={retryDelete}
            size="sm"
            variant="outline"
          >
            {remove.canRetryLast ? "重试删除" : "恢复删除"}
          </Button>
        ) : (
          <p className="text-sm text-muted-foreground">
            此删除请求没有可安全复用的重试凭据，请刷新页面或联系支持。
          </p>
        )}
      </div>
    );
  }
  if (
    store.lifecycleStatus !== "active" &&
    store.lifecycleStatus !== "disabled"
  ) {
    return null;
  }
  if (!canUpdate && !canDelete) return null;
  const deleteTerminal: DeleteTerminal =
    currentAction.kind === "delete-interrupted"
      ? currentAction.retryable
        ? "retryable"
        : "semantic"
      : null;
  const retryAvailable = Boolean(
    currentAction.kind === "delete-interrupted" &&
      currentAction.retryable &&
      !currentAction.refreshing &&
      canDelete &&
      remove.canRetryLast,
  );
  return (
    <StoreLifecycleControls
      action={currentAction}
      canDelete={canDelete}
      canUpdate={canUpdate}
      isBusy={isBusy}
      key={`${confirmationScopeKey}:${deleteCompletionToken}`}
      onConfirmDelete={submitDelete}
      onRetryDelete={retryDelete}
      onStateAction={runStateAction}
      phrase={phrase}
      retryAvailable={retryAvailable}
      store={store}
      terminal={deleteTerminal}
    />
  );
}

function isOrganizationAccessError(code?: string) {
  return (
    code === "ORGANIZATION_ACCESS_REVOKED" ||
    code === "ORGANIZATION_ACCESS_DENIED" ||
    code === "ORGANIZATION_CONTEXT_CHANGED"
  );
}

function lifecycleFailureMessage(code?: string) {
  if (code === "PERMISSION_DENIED") {
    return "店铺操作未获授权，请刷新店铺信息后重试。";
  }
  if (code === "STORE_NOT_FOUND") {
    return "店铺已不存在，请刷新店铺列表。";
  }
  return "店铺操作未完成，请刷新店铺信息后重试。";
}

function StoreLifecycleControls({
  action,
  canDelete,
  canUpdate,
  isBusy,
  onConfirmDelete,
  onRetryDelete,
  onStateAction,
  phrase,
  retryAvailable,
  store,
  terminal,
}: {
  action: ActionState;
  canDelete: boolean;
  canUpdate: boolean;
  isBusy: boolean;
  onConfirmDelete: () => void;
  onRetryDelete: () => void;
  onStateAction: (action: "enable" | "disable") => void;
  phrase: string;
  retryAvailable: boolean;
  store: WorkbenchStore;
  terminal: DeleteTerminal;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  if (action.kind === "delete-interrupted" && !dialogOpen) {
    return (
      <div className="space-y-2" role="alert">
        <p className="text-sm text-destructive">
          {action.retryable
            ? "删除请求未完成，不能重新提交普通删除。"
            : "删除请求未完成，当前请求不能重试。"}
        </p>
        {retryAvailable ? (
          <Button
            disabled={isBusy}
            onClick={onRetryDelete}
            size="sm"
            variant="outline"
          >
            重试删除
          </Button>
        ) : null}
      </div>
    );
  }
  return (
    <div className="flex flex-wrap gap-2">
      {canUpdate ? (
        <Button
          disabled={isBusy || action.kind !== "idle"}
          onClick={() =>
            onStateAction(
              store.lifecycleStatus === "active" ? "disable" : "enable",
            )
          }
          size="sm"
          variant="outline"
        >
          {store.lifecycleStatus === "active" ? "停用店铺" : "重新启用店铺"}
        </Button>
      ) : null}
      {canDelete ? (
        <Button
          disabled={isBusy || action.kind !== "idle"}
          onClick={() => {
            if (canDelete && action.kind === "idle") setDialogOpen(true);
          }}
          size="sm"
          variant="destructive"
        >
          删除店铺
        </Button>
      ) : null}
      {canDelete && dialogOpen ? (
        <DeleteConfirmation
          onCancel={() => setDialogOpen(false)}
          onConfirm={onConfirmDelete}
          onRetry={onRetryDelete}
          pending={isBusy}
          phrase={phrase}
          retryAvailable={retryAvailable}
          terminal={terminal}
        />
      ) : null}
    </div>
  );
}

function DeleteConfirmation({
  onCancel,
  onConfirm,
  pending,
  phrase,
  retryAvailable,
  onRetry,
  terminal,
}: {
  onCancel: () => void;
  onConfirm: () => void;
  pending: boolean;
  phrase: string;
  retryAvailable: boolean;
  onRetry: () => void;
  terminal: DeleteTerminal;
}) {
  const [confirmation, setConfirmation] = useState("");
  const [submitted, setSubmitted] = useState(false);
  return (
    <section
      aria-describedby="store-delete-description"
      aria-labelledby="store-delete-title"
      className="w-full rounded-lg border border-destructive/30 p-4"
      role="alertdialog"
    >
      <h2 className="font-semibold" id="store-delete-title">
        确认删除店铺
      </h2>
      <p
        className="mt-2 text-sm text-muted-foreground"
        id="store-delete-description"
      >
        此操作不会在界面中提供恢复。请输入：
        <strong className="block break-all">{phrase}</strong>
      </p>
      <label
        className="mt-3 grid gap-1 text-sm font-medium"
        htmlFor="store-delete-confirmation"
      >
        确认删除文本
        <input
          className="h-10 rounded-md border bg-background px-3"
          disabled={pending || submitted}
          id="store-delete-confirmation"
          onChange={(event) => setConfirmation(event.target.value)}
          value={confirmation}
        />
      </label>
      {terminal ? (
        <p className="mt-3 text-sm text-destructive" role="alert">
          {terminal === "retryable"
            ? "删除请求未完成，请仅在可用时重试。"
            : "删除请求未完成，当前请求不能重试。"}
        </p>
      ) : null}
      <div className="mt-4 flex flex-wrap gap-2">
        <Button disabled={pending} onClick={onCancel} variant="outline">
          取消
        </Button>
        <Button
          disabled={pending || submitted || confirmation !== phrase}
          onClick={() => {
            if (submitted || confirmation !== phrase) return;
            setSubmitted(true);
            onConfirm();
          }}
          variant="destructive"
        >
          确认删除
        </Button>
        {retryAvailable ? (
          <Button disabled={pending} onClick={onRetry} variant="outline">
            重试删除
          </Button>
        ) : null}
      </div>
    </section>
  );
}
