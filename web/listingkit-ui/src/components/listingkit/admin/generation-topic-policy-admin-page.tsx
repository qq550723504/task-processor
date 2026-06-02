"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  ChevronRight,
  Plus,
  RefreshCw,
  Search,
  ShieldAlert,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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
import {
  createListingGenerationTopicOverride,
  deleteListingGenerationTopicOverride,
  getListingGenerationTopicCatalog,
  updateListingGenerationTopicOverride,
  updateListingGenerationTopicOverrideStatus,
  type ListingGenerationTopicCatalogItem,
  type ListingGenerationTopicOverrideInput,
} from "@/lib/api/admin-generation-topic-overrides";

const DEFAULT_FORM: ListingGenerationTopicPolicyInput = {
  platform: "shein",
  topicKey: "",
  remark: "",
  status: 1,
};

export function GenerationTopicPolicyAdminPage() {
  const [platform, setPlatform] = useState("shein");
  const [topicKey, setTopicKey] = useState("");
  const [status, setStatus] = useState("");
  const [catalogSearch, setCatalogSearch] = useState("");
  const [overrideFilter, setOverrideFilter] = useState("all");
  const [selectedTopicKey, setSelectedTopicKey] = useState("");
  const [showCreatePolicyDialog, setShowCreatePolicyDialog] = useState(false);
  const [form, setForm] = useState<ListingGenerationTopicPolicyInput>(DEFAULT_FORM);
  const [overrideDrafts, setOverrideDrafts] = useState<Record<string, OverrideDraft>>(
    {},
  );
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
  const catalogQuery = useQuery({
    queryKey: ["listingkit-admin-generation-topic-catalog", platform],
    queryFn: () => getListingGenerationTopicCatalog({ platform }),
  });

  const items: ListingGenerationTopicPolicy[] = policyQuery.data?.items ?? [];
  const catalogItems: ListingGenerationTopicCatalogItem[] =
    catalogQuery.data?.items ?? [];
  const total = policyQuery.data?.total ?? 0;
  const loading = policyQuery.isLoading || policyQuery.isFetching;
  const catalogLoading = catalogQuery.isLoading || catalogQuery.isFetching;
  const topicOptions = useMemo<ReadonlyArray<readonly [string, string]>>(
    () => catalogItems.map((item) => [item.key, item.key] as const),
    [catalogItems],
  );
  const filteredCatalogItems = useMemo(
    () =>
      catalogItems.filter((item) => {
        const normalizedSearch = catalogSearch.trim().toLowerCase();
        if (
          normalizedSearch &&
          !item.key.toLowerCase().includes(normalizedSearch)
        ) {
          return false;
        }
        switch (overrideFilter) {
          case "with_override":
            return Boolean(item.tenantOverride);
          case "without_override":
            return !item.tenantOverride;
          case "override_enabled":
            return item.tenantOverride?.status === 1;
          case "override_disabled": {
            const tenantOverride = item.tenantOverride;
            if (!tenantOverride) {
              return false;
            }
            return tenantOverride.status !== 1;
          }
          default:
            return true;
        }
      }),
    [catalogItems, catalogSearch, overrideFilter],
  );
  const sortedCatalogItems = useMemo(
    () =>
      [...filteredCatalogItems].sort((left, right) => {
        const leftRank = getTopicSortRank(left);
        const rightRank = getTopicSortRank(right);
        if (leftRank !== rightRank) {
          return leftRank - rightRank;
        }
        if (left.priority !== right.priority) {
          return left.priority - right.priority;
        }
        return left.key.localeCompare(right.key);
      }),
    [filteredCatalogItems],
  );
  const catalogStats = useMemo(
    () => ({
      enabled: sortedCatalogItems.filter((item) => item.tenantOverride?.status === 1)
        .length,
      disabled: sortedCatalogItems.filter(
        (item) => Boolean(item.tenantOverride) && item.tenantOverride?.status !== 1,
      ).length,
      unconfigured: sortedCatalogItems.filter((item) => !item.tenantOverride).length,
    }),
    [sortedCatalogItems],
  );
  const selectedCatalogItem = useMemo(() => {
    if (!sortedCatalogItems.length) {
      return null;
    }
    return (
      sortedCatalogItems.find((item) => item.key === selectedTopicKey) ??
      sortedCatalogItems[0]
    );
  }, [sortedCatalogItems, selectedTopicKey]);
  const visibleError =
    error ||
    (policyQuery.error instanceof Error ? policyQuery.error.message : "") ||
    (catalogQuery.error instanceof Error ? catalogQuery.error.message : "");

  useEffect(() => {
    if (!sortedCatalogItems.length) {
      if (selectedTopicKey) {
        setSelectedTopicKey("");
      }
      return;
    }
    if (
      !selectedTopicKey ||
      !sortedCatalogItems.some((item) => item.key === selectedTopicKey)
    ) {
      setSelectedTopicKey(sortedCatalogItems[0].key);
    }
  }, [sortedCatalogItems, selectedTopicKey]);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingGenerationTopicPolicy(form);
      setForm(DEFAULT_FORM);
      setShowCreatePolicyDialog(false);
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

  async function handleSaveOverride(item: ListingGenerationTopicCatalogItem) {
    setSaving(true);
    setError("");
    try {
      const draft = resolveOverrideDraft(item, overrideDrafts[item.key]);
      const input: ListingGenerationTopicOverrideInput = {
        platform: "shein",
        topicKey: item.key,
        additionalPromptDirectives: parseMultilineList(draft.directivesText),
        additionalLexiconByLanguage: parseLexiconJson(draft.lexiconText),
        remark: draft.remark || undefined,
        status: draft.status,
      };
      if (item.tenantOverride?.id) {
        await updateListingGenerationTopicOverride(item.tenantOverride.id, input);
      } else {
        await createListingGenerationTopicOverride(input);
      }
      await catalogQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggleOverride(item: ListingGenerationTopicCatalogItem) {
    if (!item.tenantOverride?.id) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      await updateListingGenerationTopicOverrideStatus(
        item.tenantOverride.id,
        item.tenantOverride.status === 1 ? 0 : 1,
        item.tenantOverride.remark,
      );
      await catalogQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteOverride(item: ListingGenerationTopicCatalogItem) {
    if (!item.tenantOverride?.id) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      await deleteListingGenerationTopicOverride(item.tenantOverride.id);
      await catalogQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
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

        <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
          <div className="flex items-start gap-3">
            <ShieldAlert className="mt-0.5 size-4 text-zinc-500" />
            <div className="min-w-0">
              <h2 className="text-base font-semibold text-zinc-950">策略管理</h2>
              <p className="mt-1 text-sm text-zinc-500">
                新增策略改为按需打开，避免主界面长期被大表单占用。
              </p>
            </div>
          </div>
          <Button
            type="button"
            className="mt-4 w-full"
            onClick={() => setShowCreatePolicyDialog(true)}
          >
            <Plus className="size-4" />
            新增策略
          </Button>
        </div>
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-4">
          <h2 className="text-lg font-semibold text-zinc-950">Topic Catalog</h2>
          <p className="mt-1 text-sm text-zinc-500">
            先从左侧筛选并选择一个 topic，再在右侧查看默认定义和编辑租户 override。
          </p>
        </div>
        <div className="grid gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
          <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-4">
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
              <TopicInput
                label="筛选 Topic"
                value={catalogSearch}
                placeholder="children"
                onChange={setCatalogSearch}
              />
              <TopicSelect
                label="Override 状态"
                value={overrideFilter}
                onChange={setOverrideFilter}
                options={[
                  ["all", "全部"],
                  ["with_override", "有 override"],
                  ["without_override", "无 override"],
                  ["override_enabled", "override 启用"],
                  ["override_disabled", "override 禁用"],
                ]}
              />
            </div>
            <div className="mt-3 flex items-center justify-between text-xs text-zinc-500">
              <span>共 {sortedCatalogItems.length} 个 topic</span>
              <Button
                type="button"
                variant="ghost"
                className="h-auto px-0 text-xs"
                onClick={() => void catalogQuery.refetch()}
              >
                {catalogLoading ? (
                  <RefreshCw className="size-3 animate-spin" />
                ) : (
                  <RefreshCw className="size-3" />
                )}
                刷新
              </Button>
            </div>
            <div className="mt-3 grid grid-cols-3 gap-2">
              <StatChip label="启用中" value={catalogStats.enabled} tone="success" />
              <StatChip label="已禁用" value={catalogStats.disabled} tone="warning" />
              <StatChip
                label="未配置"
                value={catalogStats.unconfigured}
                tone="neutral"
              />
            </div>
            <div className="mt-3 max-h-[720px] space-y-2 overflow-y-auto pr-1">
              {catalogLoading ? (
                <div className="rounded-md border border-dashed border-zinc-300 bg-white px-3 py-6 text-sm text-zinc-500">
                  加载 catalog 中...
                </div>
              ) : sortedCatalogItems.length === 0 ? (
                <div className="rounded-md border border-dashed border-zinc-300 bg-white px-3 py-6 text-sm text-zinc-500">
                  暂无匹配的 topic
                </div>
              ) : (
                sortedCatalogItems.map((item) => {
                  const active = item.key === selectedCatalogItem?.key;
                  const statusTone = getTopicStatusTone(item);
                  return (
                    <button
                      key={item.key}
                      data-topic-key={item.key}
                      type="button"
                      onClick={() => setSelectedTopicKey(item.key)}
                      className={[
                        "w-full rounded-lg border px-3 py-3 text-left transition-colors",
                        active
                          ? "border-zinc-900 bg-white shadow-sm"
                          : "border-zinc-200 bg-white hover:border-zinc-300 hover:bg-zinc-50",
                        statusTone.container,
                      ].join(" ")}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-sm font-semibold text-zinc-950">
                              {item.key}
                            </span>
                            <Badge variant="outline">P{item.priority}</Badge>
                          </div>
                          <p className="mt-1 text-xs text-zinc-500">
                            默认 {countWords(item.lexiconByLanguage)} 个词
                          </p>
                        </div>
                        <ChevronRight
                          className={[
                            "mt-0.5 size-4 shrink-0 text-zinc-400 transition-transform",
                            active ? "translate-x-0.5 text-zinc-700" : "",
                          ].join(" ")}
                        />
                      </div>
                      <div className="mt-2 flex flex-wrap gap-2">
                        <Badge variant={statusTone.badgeVariant}>
                          {item.tenantOverride
                            ? item.tenantOverride.status === 1
                              ? "override 启用"
                              : "override 禁用"
                            : "无 override"}
                        </Badge>
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          <div className="min-w-0">
            {!selectedCatalogItem ? (
              <div className="rounded-lg border border-dashed border-zinc-300 bg-white px-6 py-12 text-sm text-zinc-500">
                请选择左侧 topic 查看详情。
              </div>
            ) : (
              <TopicEditorPanel
                item={selectedCatalogItem}
                draft={resolveOverrideDraft(
                  selectedCatalogItem,
                  overrideDrafts[selectedCatalogItem.key],
                )}
                saving={saving}
                onChangeDraft={(nextDraft) =>
                  setOverrideDrafts((current) => ({
                    ...current,
                    [selectedCatalogItem.key]: nextDraft,
                  }))
                }
                onSave={() => void handleSaveOverride(selectedCatalogItem)}
                onToggle={() => void handleToggleOverride(selectedCatalogItem)}
                onDelete={() => void handleDeleteOverride(selectedCatalogItem)}
              />
            )}
          </div>
        </div>
      </section>

      {showCreatePolicyDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/40 px-4 py-8">
          <div
            role="dialog"
            aria-modal="true"
            className="w-full max-w-lg rounded-xl border border-zinc-200 bg-white p-5 shadow-xl"
          >
            <form onSubmit={handleCreate}>
              <div className="mb-4 flex items-center gap-2">
                <ShieldAlert className="size-4 text-zinc-500" />
                <h2 className="text-base font-semibold text-zinc-950">
                  新增生成主题策略
                </h2>
              </div>
              <TopicSelect
                label="平台"
                value={form.platform}
                onChange={(nextPlatform) =>
                  setForm({ ...form, platform: nextPlatform })
                }
                options={[["shein", "SHEIN"]]}
              />
              <TopicSelect
                label="Topic Key"
                value={form.topicKey}
                onChange={(nextTopicKey) =>
                  setForm({ ...form, topicKey: nextTopicKey })
                }
                options={[
                  ["", "请选择"],
                  ...topicOptions,
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
              <div className="mt-5 grid gap-2 sm:grid-cols-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setShowCreatePolicyDialog(false)}
                >
                  取消
                </Button>
                <Button
                  type="submit"
                  disabled={saving || !form.platform.trim() || !form.topicKey.trim()}
                >
                  {saving ? (
                    <RefreshCw className="size-4 animate-spin" />
                  ) : (
                    <Plus className="size-4" />
                  )}
                  保存策略
                </Button>
              </div>
            </form>
          </div>
        </div>
      ) : null}
    </div>
  );
}

type OverrideDraft = {
  directivesText: string;
  lexiconText: string;
  remark: string;
  status: number;
};

function resolveOverrideDraft(
  item: ListingGenerationTopicCatalogItem,
  draft?: OverrideDraft,
): OverrideDraft {
  if (draft) {
    return draft;
  }
  return {
    directivesText: (item.tenantOverride?.additionalPromptDirectives ?? []).join("\n"),
    lexiconText: JSON.stringify(
      item.tenantOverride?.additionalLexiconByLanguage ?? {},
      null,
      2,
    ),
    remark: item.tenantOverride?.remark ?? "",
    status: item.tenantOverride?.status ?? 1,
  };
}

function parseMultilineList(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseLexiconJson(value: string): Record<string, string[]> {
  const trimmed = value.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed) as Record<string, unknown>;
  const normalized: Record<string, string[]> = {};
  for (const [language, words] of Object.entries(parsed)) {
    if (!Array.isArray(words)) {
      continue;
    }
    normalized[language] = words
      .map((word) => String(word).trim())
      .filter(Boolean);
  }
  return normalized;
}

function countWords(lexiconByLanguage: Record<string, string[]>) {
  return Object.values(lexiconByLanguage).reduce(
    (total, words) => total + words.length,
    0,
  );
}

function getTopicSortRank(item: ListingGenerationTopicCatalogItem) {
  if (item.tenantOverride?.status === 1) {
    return 0;
  }
  if (item.tenantOverride) {
    return 1;
  }
  return 2;
}

function getTopicStatusTone(item: ListingGenerationTopicCatalogItem) {
  if (item.tenantOverride?.status === 1) {
    return {
      container: "border-l-4 border-l-emerald-500",
      badgeVariant: "success" as const,
    };
  }
  if (item.tenantOverride) {
    return {
      container: "border-l-4 border-l-amber-500",
      badgeVariant: "neutral" as const,
    };
  }
  return {
    container: "border-l-4 border-l-zinc-200",
    badgeVariant: "outline" as const,
  };
}

function TopicEditorPanel({
  item,
  draft,
  saving,
  onChangeDraft,
  onSave,
  onToggle,
  onDelete,
}: {
  item: ListingGenerationTopicCatalogItem;
  draft: OverrideDraft;
  saving: boolean;
  onChangeDraft: (draft: OverrideDraft) => void;
  onSave: () => void;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const guidance = getTopicGuidance(item.key);
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="flex flex-col gap-3 border-b border-zinc-100 pb-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-lg font-semibold text-zinc-950">{item.key}</h3>
            <Badge variant="outline">priority {item.priority}</Badge>
            <Badge
              variant={item.tenantOverride?.status === 1 ? "success" : "neutral"}
            >
              {item.tenantOverride
                ? item.tenantOverride.status === 1
                  ? "override 启用"
                  : "override 禁用"
                : "无 override"}
            </Badge>
          </div>
          <p className="mt-2 text-sm text-zinc-500">
            查看默认定义、当前生效定义，并维护这个 topic 的租户级 override。
          </p>
        </div>
        <div className="grid gap-2 sm:grid-cols-3 lg:min-w-[420px]">
          <Button type="button" variant="secondary" onClick={onSave} disabled={saving}>
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存 override
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={onToggle}
            disabled={saving || !item.tenantOverride?.id}
          >
            切换状态
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={onDelete}
            disabled={saving || !item.tenantOverride?.id}
          >
            <Trash2 className="size-4" />
            删除 override
          </Button>
        </div>
      </div>

      <div className="mt-4 grid gap-4 2xl:grid-cols-3">
        <DefinitionBlock
          title="默认词包"
          directives={item.promptDirectives}
          lexiconByLanguage={item.lexiconByLanguage}
        />
        <DefinitionBlock
          title="生效词包"
          directives={item.effectiveDefinition.promptDirectives}
          lexiconByLanguage={item.effectiveDefinition.lexiconByLanguage}
        />
        <div className="space-y-3 rounded-md border border-zinc-200 bg-zinc-50 p-4">
          <div className="rounded-md border border-sky-200 bg-sky-50 p-3">
            <h4 className="text-sm font-semibold text-zinc-950">推荐用途</h4>
            <p className="mt-2 text-sm text-zinc-700">{guidance.summary}</p>
            <p className="mt-2 text-xs text-zinc-500">{guidance.scenario}</p>
          </div>
          <h4 className="text-sm font-semibold text-zinc-950">租户 Override</h4>
          <Label className="block text-xs font-medium text-zinc-500">
            额外 Prompt Directives
            <Textarea
              className="mt-1 min-h-28 bg-white"
              value={draft.directivesText}
              onChange={(event) =>
                onChangeDraft({ ...draft, directivesText: event.target.value })
              }
              placeholder="每行一条 directive"
            />
          </Label>
          <Label className="block text-xs font-medium text-zinc-500">
            额外词包 JSON
            <Textarea
              className="mt-1 min-h-32 bg-white font-mono text-xs"
              value={draft.lexiconText}
              onChange={(event) =>
                onChangeDraft({ ...draft, lexiconText: event.target.value })
              }
              placeholder={'{"en":["toddler"],"zh":["幼童"]}'}
            />
          </Label>
          <TopicInput
            label="备注"
            value={draft.remark}
            onChange={(remark) => onChangeDraft({ ...draft, remark })}
          />
        </div>
      </div>
    </div>
  );
}

function getTopicGuidance(topicKey: string) {
  const guidanceMap: Record<string, { summary: string; scenario: string }> = {
    children: {
      summary:
        "适合禁止出现儿童、孩童、kids、children 这类面向儿童用户的表达。",
      scenario:
        "常见场景：不希望标题或描述里出现“儿童款”“kids use”“for children”这类表述。",
    },
    baby: {
      summary:
        "适合禁止出现婴儿、新生儿、宝宝、infant、newborn 这类更低龄表达。",
      scenario:
        "常见场景：不希望文案触发婴幼儿用品相关审核，或被写成 baby/newborn 使用场景。",
    },
    food: {
      summary:
        "适合禁止出现食品、食物、零食、edible、snack 这类食物属性表达。",
      scenario:
        "常见场景：产品本身不是食品或食品接触品，但 AI 容易写出 snack、edible、food storage 一类文案。",
    },
    meals: {
      summary:
        "适合禁止出现早餐、午餐、晚餐、meal、dinner 这类餐食场景表达。",
      scenario:
        "常见场景：不希望商品被描述成早餐盘、午餐便当、晚餐搭配用品等 meal 场景。",
    },
    knives: {
      summary:
        "适合禁止出现刀具、刀刃、knife、blade 这类锐器或刀具表达。",
      scenario:
        "常见场景：不希望商品被写成切割、刀具、锋利边缘或 blade/knife 相关用途。",
    },
  };
  return (
    guidanceMap[topicKey] ?? {
      summary: "这个 topic 用于限制一类特定主题在 AI 文案中出现。",
      scenario: "建议先启用 policy，再根据租户特殊要求补充 override 词包和 prompt 规则。",
    }
  );
}

function StatChip({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "success" | "warning" | "neutral";
}) {
  const toneClass =
    tone === "success"
      ? "border-emerald-200 bg-emerald-50 text-emerald-700"
      : tone === "warning"
        ? "border-amber-200 bg-amber-50 text-amber-700"
        : "border-zinc-200 bg-white text-zinc-700";
  return (
    <div className={["rounded-md border px-3 py-2", toneClass].join(" ")}>
      <p className="text-[11px] font-medium">{label}</p>
      <p className="mt-1 text-sm font-semibold">
        {label} {value}
      </p>
    </div>
  );
}

function DefinitionBlock({
  title,
  directives = [],
  lexiconByLanguage,
}: {
  title: string;
  directives?: string[];
  lexiconByLanguage: Record<string, string[]>;
}) {
  return (
    <div className="rounded-md border border-zinc-200 bg-white p-3">
      <h4 className="text-sm font-semibold text-zinc-950">{title}</h4>
      {directives.length > 0 ? (
        <>
          <p className="mt-2 text-xs font-medium text-zinc-500">Prompt Directives</p>
          <ul className="mt-1 list-disc pl-5 text-sm text-zinc-700">
            {directives.map((directive) => (
              <li key={directive}>{directive}</li>
            ))}
          </ul>
        </>
      ) : null}
      <div className="mt-3 space-y-2">
        {Object.entries(lexiconByLanguage).map(([language, words]) => (
          <div key={language}>
            <p className="text-xs font-medium uppercase text-zinc-500">{language}</p>
            <p className="text-sm text-zinc-700">{words.join(", ") || "-"}</p>
          </div>
        ))}
      </div>
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
  options: ReadonlyArray<readonly [string, string]>;
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
