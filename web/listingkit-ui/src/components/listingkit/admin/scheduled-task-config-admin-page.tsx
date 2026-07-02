"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  AdminStoreSelect,
  formatAdminStoreName,
  useAdminSimpleStores,
} from "@/components/listingkit/admin/admin-store-select";
import {
  deleteListingScheduledTaskConfig,
  getListingScheduledTaskConfigs,
  updateListingScheduledTaskConfigStatus,
  upsertListingScheduledTaskConfig,
  type ListingScheduledTaskConfig,
  type ListingScheduledTaskConfigInput,
} from "@/lib/api/admin-scheduled-task-configs";
import { useQuery } from "@tanstack/react-query";
import { Clock, Plus, Power, RefreshCw, Search, Trash2 } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

const DEFAULT_FORM: ListingScheduledTaskConfigInput = {
  storeId: 0,
  platform: "shein",
  taskType: "inventory",
  enabled: true,
  intervalSeconds: 3600,
  remark: "",
};

const TASK_TYPE_OPTIONS: Array<[string, string]> = [
  ["inventory", "库存同步"],
  ["productSync", "产品同步"],
  ["activity", "活动报名"],
  ["pricing", "自动核价"],
];

export function ScheduledTaskConfigAdminPage() {
  const [platform, setPlatform] = useState("");
  const [taskType, setTaskType] = useState("");
  const [enabled, setEnabled] = useState("");
  const [form, setForm] =
    useState<ListingScheduledTaskConfigInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      taskType: taskType || undefined,
      enabled: enabled === "" ? undefined : enabled === "true",
    }),
    [enabled, platform, taskType],
  );

  const configQuery = useQuery({
    queryKey: ["listingkit-admin-scheduled-task-configs", query],
    queryFn: () => getListingScheduledTaskConfigs(query),
  });
  const storesQuery = useAdminSimpleStores();

  const configs = configQuery.data?.items ?? [];
  const stores = storesQuery.data ?? [];
  const loading = configQuery.isLoading || configQuery.isFetching;
  const total = configQuery.data?.total ?? 0;
  const visibleError =
    error ||
    (configQuery.error instanceof Error ? configQuery.error.message : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  function platformForStore(storeId: number) {
    return stores.find((store) => store.id === storeId)?.platform?.trim() || DEFAULT_FORM.platform;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const selectedStorePlatform = platformForStore(form.storeId);
      const saved = await upsertListingScheduledTaskConfig({
        ...form,
        platform: selectedStorePlatform,
        intervalSeconds: Math.max(60, Number(form.intervalSeconds) || 3600),
      });
      const nextPlatform = saved.platform || form.platform || DEFAULT_FORM.platform;
      const nextTaskType = saved.taskType || form.taskType || DEFAULT_FORM.taskType;
      setPlatform(nextPlatform);
      setTaskType(nextTaskType);
      setEnabled("");
      setForm({
        ...DEFAULT_FORM,
        platform: nextPlatform,
        taskType: nextTaskType,
      });
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(config: ListingScheduledTaskConfig) {
    setError("");
    try {
      await updateListingScheduledTaskConfigStatus(
        config.id,
        !config.enabled,
      );
      await configQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingScheduledTaskConfig(id);
      await configQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">定时任务配置</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条配置，按租户隔离。
            </p>
          </div>
          <form
            className="flex flex-col gap-2 sm:flex-row sm:flex-wrap"
            onSubmit={(event) => event.preventDefault()}
          >
            <TaskSelect
              label="平台"
              value={platform}
              onChange={setPlatform}
              options={[
                ["", "全部"],
                ["shein", "SHEIN"],
              ]}
            />
            <TaskSelect
              label="任务"
              value={taskType}
              onChange={setTaskType}
              options={[["", "全部"], ...TASK_TYPE_OPTIONS]}
            />
            <TaskSelect
              label="状态"
              value={enabled}
              onChange={setEnabled}
              options={[
                ["", "全部"],
                ["true", "启用"],
                ["false", "停用"],
              ]}
            />
            <Button
              type="button"
              onClick={() => void configQuery.refetch()}
              className="w-full sm:mt-5 sm:w-auto"
              variant="secondary"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              查询
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_390px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-[46rem] divide-y divide-zinc-200 text-sm">
              <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <TableRow>
                  <TableHead className="px-4 py-3">店铺</TableHead>
                  <TableHead className="px-4 py-3">平台</TableHead>
                  <TableHead className="px-4 py-3">任务</TableHead>
                  <TableHead className="px-4 py-3">间隔</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3">备注</TableHead>
                  <TableHead className="px-4 py-3 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-zinc-100">
                {loading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={7}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : configs.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={7}>
                      暂无定时任务配置
                    </TableCell>
                  </TableRow>
                ) : (
                  configs.map((config) => (
                    <TableRow key={config.id} className="align-top">
                      <TableCell className="px-4 py-3 font-medium text-zinc-950">
                        {formatAdminStoreName(stores, config.storeId)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {config.platform}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {taskTypeLabel(config.taskType)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatInterval(config.intervalSeconds)}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Button
                          type="button"
                          onClick={() => void handleToggle(config)}
                          variant="ghost"
                          className="h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={config.enabled ? "success" : "neutral"}>
                            {config.enabled ? "启用" : "停用"}
                          </Badge>
                        </Button>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {config.remark || "-"}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${taskTypeLabel(config.taskType)} ${config.storeId}`}
                          onClick={() => void handleDelete(config.id)}
                          size="icon"
                          variant="ghost"
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>

        <form
          onSubmit={handleSubmit}
          className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
        >
          <div className="mb-4 flex items-center gap-2">
            <Clock className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">配置任务</h2>
          </div>
          <AdminStoreSelect
            value={form.storeId}
            onChange={(storeId) =>
              setForm({ ...form, storeId, platform: platformForStore(storeId) })
            }
            stores={stores}
            emptyLabel="请选择店铺"
          />
          <TaskSelect
            label="任务"
            value={form.taskType}
            onChange={(nextTaskType) =>
              setForm({ ...form, taskType: nextTaskType || "inventory" })
            }
            options={TASK_TYPE_OPTIONS}
          />
          <TaskInput
            label="间隔秒数"
            type="number"
            value={String(form.intervalSeconds)}
            onChange={(value) =>
              setForm({ ...form, intervalSeconds: Number(value) || 0 })
            }
          />
          <Label className="mb-3 flex items-center gap-2 text-xs font-medium text-zinc-700">
            <Checkbox
              checked={form.enabled}
              onChange={(event) =>
                setForm({ ...form, enabled: event.target.checked })
              }
            />
            启用
          </Label>
          <TaskInput
            label="备注"
            value={form.remark ?? ""}
            onChange={(remark) => setForm({ ...form, remark })}
          />
          <Button
            type="submit"
            disabled={saving || form.storeId <= 0}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : form.enabled ? (
              <Power className="size-4" />
            ) : (
              <Plus className="size-4" />
            )}
            保存配置
          </Button>
        </form>
      </section>
    </div>
  );
}

function TaskSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<[string, string]>;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue} value={optionValue}>
            {labelText}
          </option>
        ))}
      </Select>
    </Label>
  );
}

function TaskInput({
  label,
  type = "text",
  value,
  onChange,
}: {
  label: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </Label>
  );
}

function taskTypeLabel(taskType: string) {
  return TASK_TYPE_OPTIONS.find(([value]) => value === taskType)?.[1] ?? taskType;
}

function formatInterval(seconds: number) {
  if (seconds % 3600 === 0) {
    return `${seconds / 3600} 小时`;
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60} 分钟`;
  }
  return `${seconds} 秒`;
}
