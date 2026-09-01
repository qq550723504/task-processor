"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useId, useRef, useState } from "react";
import { useForm } from "react-hook-form";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { WorkbenchAPIError, WorkbenchStore } from "@/lib/api/workbench-stores";
import { useCreateWorkbenchStore, useUpdateWorkbenchStore } from "@/lib/query/use-workbench-stores";
import {
  workbenchStoreCreateSchema,
  workbenchStoreUpdateSchema,
  type WorkbenchStoreCreateInput,
  type WorkbenchStoreUpdateInput,
} from "@/lib/validation/workbench-store";
import { useRouter } from "next/navigation";

type Conflict = { latest: WorkbenchStore; changedFields: EditableField[] };
type EditableField = "name" | "region";
type StoreFormProps =
  | { mode: "create"; store?: never; conflict?: never; recoveryState?: never; onConflict?: never; onSaved?: never }
  | {
      mode: "edit";
      store: WorkbenchStore;
      conflict?: Conflict | null;
      recoveryState?: "idle" | "loading" | "failed";
      onConflict?: (
        draft: WorkbenchStoreUpdateInput,
        baseline: WorkbenchStore,
      ) => void;
      onSaved?: (store: WorkbenchStore) => void;
    };

const FIELD_ERROR_TEXT: Record<EditableField | "externalStoreId", string> = {
  name: "店铺名称格式不正确",
  region: "区域格式不正确",
  externalStoreId: "外部店铺 ID 格式不正确",
};

export function StoreForm(props: StoreFormProps) {
  const router = useRouter();
  const context = useWorkbenchContext();
  const organization = context.effectiveOrganization;
  const [mountedOrganizationId] = useState(() => organization?.id ?? "");
  const organizationChanged =
    (organization?.id ?? "") !== mountedOrganizationId;
  const mountedRef = useRef(true);
  const confirmationPendingRef = useRef(false);
  const submissionPendingRef = useRef(false);
  const [submissionPending, setSubmissionPending] = useState(false);
  const editBaselineRef = useRef<WorkbenchStore | null>(
    props.mode === "edit" ? props.store : null,
  );
  const form = useForm<WorkbenchStoreCreateInput | WorkbenchStoreUpdateInput>({
    resolver: zodResolver(
      props.mode === "create" ? workbenchStoreCreateSchema : workbenchStoreUpdateSchema,
    ),
    defaultValues: props.mode === "create"
      ? { name: "", platform: "shein", region: "", externalStoreId: "" }
      : { name: props.store.name, region: props.store.region },
  });
  const create = useCreateWorkbenchStore();
  const update = useUpdateWorkbenchStore();
  const [formError, setFormError] = useState<string | null>(null);
  const isPending = submissionPending || create.isPending || update.isPending;
  const isSwitching = context.isSwitching;
  const createRetryAvailable = props.mode === "create" && create.canRetryLast;

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  useEffect(() => {
    if (
      props.mode !== "edit" ||
      !editBaselineRef.current ||
      props.store.version <= editBaselineRef.current.version
    ) {
      return;
    }
    if (!form.formState.isDirty) {
      editBaselineRef.current = props.store;
      form.reset({ name: props.store.name, region: props.store.region });
    }
  }, [form, form.formState.isDirty, props]);

  useEffect(() => {
    if (!organization || organization.id === mountedOrganizationId) return;
    form.reset(props.mode === "create" ? { name: "", platform: "shein", region: "", externalStoreId: "" } : { name: props.store.name, region: props.store.region });
    router.replace("/workbench/stores");
  }, [form, mountedOrganizationId, organization, props, router]);

  useEffect(() => {
    return context.registerOrganizationSwitchGuard((target) => {
      if (
        !mountedRef.current ||
        organizationChanged ||
        confirmationPendingRef.current
      ) {
        return false;
      }
      if (!form.formState.isDirty) return true;
      confirmationPendingRef.current = true;
      try {
        return Boolean(
          window.confirm(`切换企业将放弃当前草稿：${organization?.name ?? "当前企业"} → ${target.name}。是否继续？`),
        );
      } finally {
        confirmationPendingRef.current = false;
      }
    });
  }, [context, form.formState.isDirty, organization?.name, organizationChanged]);

  const submit = (values: WorkbenchStoreCreateInput | WorkbenchStoreUpdateInput) => {
    if (
      isPending ||
      isSwitching ||
      submissionPendingRef.current ||
      !organization ||
      organizationChanged
    ) {
      return;
    }
    if (props.mode === "create") {
      submissionPendingRef.current = true;
      setSubmissionPending(true);
      setFormError(null);
      create.mutate(values as WorkbenchStoreCreateInput, {
        onSuccess: (store) => { submissionPendingRef.current = false; setSubmissionPending(false); router.push(`/workbench/stores/${store.id}`); },
        onError: (error) => { submissionPendingRef.current = false; setSubmissionPending(false); setFormError(applyServerError(form, error, "create")); },
      });
      return;
    }
    const baseline = editBaselineRef.current;
    if (
      props.conflict ||
      props.recoveryState === "loading" ||
      props.recoveryState === "failed" ||
      !baseline
    ) {
      return;
    }
    submissionPendingRef.current = true;
    setSubmissionPending(true);
    setFormError(null);
    update.mutate(
      { id: baseline.id, version: baseline.version, input: values as WorkbenchStoreUpdateInput },
      {
        onSuccess: (store) => { submissionPendingRef.current = false; setSubmissionPending(false); editBaselineRef.current = store; form.reset({ name: store.name, region: store.region }); props.onSaved?.(store); },
        onError: (error) => {
          if (error.code === "STORE_VERSION_CONFLICT") {
            submissionPendingRef.current = false; setSubmissionPending(false); props.onConflict?.(values as WorkbenchStoreUpdateInput, baseline);
            return;
          }
          submissionPendingRef.current = false; setSubmissionPending(false); setFormError(applyServerError(form, error, "edit"));
        },
      },
    );
  };

  const retryCreate = () => {
    if (isPending || isSwitching || !create.canRetryLast || submissionPendingRef.current) return;
    submissionPendingRef.current = true;
    setSubmissionPending(true);
    setFormError(null);
    void create.retryLast().then(
      (store) => { submissionPendingRef.current = false; setSubmissionPending(false); router.push(`/workbench/stores/${store.id}`); },
      (error: unknown) => { submissionPendingRef.current = false; setSubmissionPending(false); setFormError(applyServerError(form, error, "create")); },
    );
  };
  const retryConflict = () => {
    if (isPending || isSwitching || props.mode !== "edit" || !props.conflict || !editBaselineRef.current || submissionPendingRef.current) return;
    const conflict = props.conflict;
    const draft = workbenchStoreUpdateSchema.safeParse(form.getValues());
    if (!draft.success) return;
    submissionPendingRef.current = true;
    setSubmissionPending(true);
    setFormError(null);
    update.mutate(
      { id: editBaselineRef.current.id, version: conflict.latest.version, input: draft.data },
      {
        onSuccess: (store) => { submissionPendingRef.current = false; setSubmissionPending(false); editBaselineRef.current = store; form.reset({ name: store.name, region: store.region }); props.onSaved?.(store); },
        onError: (error) => {
          submissionPendingRef.current = false; setSubmissionPending(false);
          if (error.code === "STORE_VERSION_CONFLICT") {
            props.onConflict?.(draft.data, conflict.latest);
          }
          else setFormError(applyServerError(form, error, "edit"));
        },
      },
    );
  };

  if (organizationChanged) {
    return (
      <section
        className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6"
        role="status"
      >
        正在切换企业...
      </section>
    );
  }

  const organizationName = organization?.name ?? "当前企业";
  return (
    <section className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <p className="text-sm text-muted-foreground">店铺中心 · {organizationName}</p>
      <h1 className="mt-1 text-2xl font-semibold tracking-tight">
        {props.mode === "create" ? `在${organizationName} 新建店铺` : `编辑${props.store.name}`}
      </h1>
      <p className="mt-2 text-sm text-muted-foreground">
        {props.mode === "create" ? `将为${organizationName}创建 SHEIN 店铺。` : `当前企业：${organizationName}`}
      </p>

      {props.mode === "edit" && props.conflict ? <ConflictNotice conflict={props.conflict} onRetry={retryConflict} pending={isPending} /> : null}
      {props.mode === "edit" && props.recoveryState === "loading" ? <p className="mt-4 rounded-md border p-3 text-sm" role="status">正在获取店铺最新版本，当前草稿已保留。</p> : null}
      {formError ? <p className="mt-4 rounded-md border border-destructive/30 p-3 text-sm text-destructive" role="alert">{formError}</p> : null}
      <form className="mt-6 space-y-5 rounded-xl border bg-card p-5" noValidate onSubmit={(event) => { void form.handleSubmit(submit)(event); }}>
        <TextField control={form} name="name" label="店铺名称" disabled={isPending || isSwitching || createRetryAvailable} />
        <TextField control={form} name="region" label="区域" disabled={isPending || isSwitching || createRetryAvailable} />
        <ReadOnlyField label="平台" value="SHEIN" />
        {props.mode === "create" ? <TextField control={form} name="externalStoreId" label="外部店铺 ID（可选）" disabled={isPending || isSwitching || createRetryAvailable} /> : <ReadOnlyField label="外部店铺 ID" value={props.store.externalStoreId || "未设置"} />}
        {props.mode === "edit" ? <ReadOnlyField label="连接状态" value={connectionLabel(props.store.connectionStatus)} /> : null}
        <Button disabled={isPending || isSwitching || (props.mode === "create" && create.canRetryLast) || (props.mode === "edit" && (Boolean(props.conflict) || props.recoveryState === "loading" || props.recoveryState === "failed"))} type="submit">
          {isPending ? "正在提交..." : props.mode === "create" ? "创建店铺" : "保存更改"}
        </Button>
        {props.mode === "create" && create.canRetryLast ? <Button disabled={isPending || isSwitching} onClick={retryCreate} type="button" variant="outline">重试创建</Button> : null}
        <Button aria-label="连接能力尚未开放" disabled type="button" variant="outline">连接能力尚未开放</Button>
      </form>
    </section>
  );
}

function TextField({ control, name, label, disabled }: { control: ReturnType<typeof useForm<WorkbenchStoreCreateInput | WorkbenchStoreUpdateInput>>; name: "name" | "region" | "externalStoreId"; label: string; disabled: boolean }) {
  const id = useId();
  const error = (control.formState.errors as Record<string, { message?: unknown }>)[name]?.message;
  const displayError = error === "Invalid input" ? (name === "name" ? "请填写店铺名称" : name === "region" ? "请填写区域" : "外部店铺 ID 格式不正确") : error;
  return <div className="grid gap-2"><Label htmlFor={id}>{label}</Label><Input aria-describedby={error ? `${id}-error` : undefined} aria-invalid={Boolean(error)} disabled={disabled} id={id} {...control.register(name as "name")} />{error ? <p className="text-sm text-destructive" id={`${id}-error`} role="alert">{String(displayError)}</p> : null}</div>;
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  const id = useId();
  return <div className="grid gap-2"><Label htmlFor={id}>{label}</Label><Input disabled id={id} value={value} /></div>;
}

function ConflictNotice({ conflict, onRetry, pending }: { conflict: Conflict; onRetry: () => void; pending: boolean }) {
  const fields = conflict.changedFields.map(editableFieldLabel).join("、") || "店铺信息";
  return <section className="mt-5 rounded-xl border border-amber-500/40 bg-amber-50 p-4 text-sm dark:bg-amber-950/20" role="alert"><p>最新版本中{fields}已被其他人修改。你的草稿仍被保留，请确认后再保存。</p><Button className="mt-3" disabled={pending} onClick={onRetry} type="button" variant="outline">使用最新版本重新保存</Button></section>;
}

function editableFieldLabel(field: EditableField) { return field === "name" ? "名称" : "区域"; }
function connectionLabel(status: WorkbenchStore["connectionStatus"]) { return ({ disconnected: "未连接", connected: "已连接", expired: "授权已过期", unavailable: "暂时无法检查" })[status]; }

function applyServerError(form: ReturnType<typeof useForm<WorkbenchStoreCreateInput | WorkbenchStoreUpdateInput>>, error: unknown, mode: "create" | "edit") {
  const apiError = error as Partial<WorkbenchAPIError>;
  const errors = Array.isArray(apiError.fieldErrors) ? apiError.fieldErrors : [];
  let unknownField = false;
  for (const item of errors) {
    if (item.field === "name" || item.field === "region" || (mode === "create" && item.field === "externalStoreId")) form.setError(item.field, { type: "server", message: FIELD_ERROR_TEXT[item.field] });
    else unknownField = true;
  }
  const message = unknownField ? "表单包含无法处理的字段错误" : apiError.code === "STORE_LIMIT_REACHED" || apiError.code === "SUBSCRIPTION_REQUIRED" ? "当前企业的店铺额度不可用，请联系管理员或升级套餐。" : apiError.code === "PERMISSION_DENIED" || apiError.code === "ORGANIZATION_ACCESS_DENIED" || apiError.code === "ORGANIZATION_ACCESS_REVOKED" || apiError.code === "ORGANIZATION_CONTEXT_CHANGED" ? "当前企业访问状态已变化，请返回店铺列表后重试。" : apiError.code === "STORE_ALREADY_EXISTS" ? "该店铺标识已存在，请检查后重试。" : errors.length === 0 ? "暂时无法提交店铺，请稍后重试。" : null;
  return message;
}
