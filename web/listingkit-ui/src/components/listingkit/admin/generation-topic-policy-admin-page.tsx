"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Search, ShieldAlert, Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
  createListingGenerationTopicPolicy,
  deleteListingGenerationTopicPolicy,
  getListingGenerationTopicPolicies,
  updateListingGenerationTopicPolicyStatus,
  type ListingGenerationTopicPolicy,
  type ListingGenerationTopicPolicyInput,
} from "@/lib/api/admin-generation-topic-policies";

const DEFAULT_FORM: ListingGenerationTopicPolicyInput = {
  platform: "shein",
  topicKey: "",
  remark: "",
  status: 1,
};

const TOPIC_OPTIONS = [
  ["children", "children"],
  ["baby", "baby"],
  ["food", "food"],
  ["meals", "meals"],
  ["knives", "knives"],
] as const;

export function GenerationTopicPolicyAdminPage() {
  const [platform, setPlatform] = useState("shein");
  const [topicKey, setTopicKey] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingGenerationTopicPolicyInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      topic_key: topicKey || undefined,
      status: status || undefined,
    }),
    [platform, topicKey, status],
  );

  const policyQuery = useQuery({
    queryKey: ["listingkit-admin-generation-topic-policies", query],
    queryFn: () => getListingGenerationTopicPolicies(query),
  });

  const items: ListingGenerationTopicPolicy[] = policyQuery.data?.items ?? [];
  const total = policyQuery.data?.total ?? 0;
  const loading = policyQuery.isLoading || policyQuery.isFetching;
  const visibleError =
    error || (policyQuery.error instanceof Error ? policyQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingGenerationTopicPolicy(form);
      setForm(DEFAULT_FORM);
      await policyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(item: ListingGenerationTopicPolicy) {
    setError("");
    try {
      await updateListingGenerationTopicPolicyStatus(
        item.id,
        item.status === 1 ? 0 : 1,
      );
      await policyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingGenerationTopicPolicy(id);
      await policyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">生成禁用主题</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条租户级 SHEIN 生成主题策略。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <TopicSelect
              label="平台"
              value={platform}
              onChange={setPlatform}
              options={[["", "全部"], ["shein", "SHEIN"]]}
            />
            <TopicInput
              label="Topic Key"
              value={topicKey}
              onChange={setTopicKey}
              placeholder="children"
            />
            <TopicSelect
              label="状态"
              value={status}
              onChange={setStatus}
              options={[
                ["", "全部"],
                ["1", "启用"],
                ["0", "禁用"],
              ]}
            />
            <Button
              type="button"
              onClick={() => void policyQuery.refetch()}
              className="mt-5"
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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_390px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-full divide-y divide-zinc-200 text-sm">
              <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <TableRow>
                  <TableHead className="px-4 py-3">平台</TableHead>
                  <TableHead className="px-4 py-3">Topic Key</TableHead>
                  <TableHead className="px-4 py-3">备注</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-zinc-100">
                {loading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={5}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : items.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={5}>
                      暂无生成主题策略
                    </TableCell>
                  </TableRow>
                ) : (
                  items.map((item) => (
                    <TableRow key={item.id} className="align-top">
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {item.platform}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {item.topicKey}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {item.remark || "-"}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Button
                          type="button"
                          onClick={() => void handleToggle(item)}
                          variant="ghost"
                          className="h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={item.status === 1 ? "success" : "neutral"}>
                            {item.status === 1 ? "启用" : "禁用"}
                          </Badge>
                        </Button>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${item.topicKey}`}
                          onClick={() => void handleDelete(item.id)}
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
          onSubmit={handleCreate}
          className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
        >
          <div className="mb-4 flex items-center gap-2">
            <ShieldAlert className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增生成主题策略</h2>
          </div>
          <TopicSelect
            label="平台"
            value={form.platform}
            onChange={(nextPlatform) => setForm({ ...form, platform: nextPlatform })}
            options={[["shein", "SHEIN"]]}
          />
          <TopicSelect
            label="Topic Key"
            value={form.topicKey}
            onChange={(nextTopicKey) => setForm({ ...form, topicKey: nextTopicKey })}
            options={[
              ["", "请选择"],
              ...TOPIC_OPTIONS,
            ]}
          />
          <TopicSelect
            label="状态"
            value={String(form.status ?? 1)}
            onChange={(nextStatus) =>
              setForm({ ...form, status: Number(nextStatus) })
            }
            options={[
              ["1", "启用"],
              ["0", "禁用"],
            ]}
          />
          <TopicInput
            label="备注"
            value={form.remark ?? ""}
            onChange={(remark) => setForm({ ...form, remark })}
          />
          <Button
            type="submit"
            disabled={saving || !form.platform.trim() || !form.topicKey.trim()}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存策略
          </Button>
        </form>
      </section>
    </div>
  );
}

function TopicInput({
  label,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </Label>
  );
}

function TopicSelect({
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
