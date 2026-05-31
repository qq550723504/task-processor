"use client";

import { useMemo, useState } from "react";
import { FileText, Save, ToggleLeft, ToggleRight } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { ListingKitSettingsSection } from "@/components/listingkit/settings/listingkit-settings-section";
import {
  usePromptTemplateCatalog,
  usePromptTemplates,
  useSetPromptTemplateStatus,
  useUpsertPromptTemplate,
} from "@/lib/query/use-prompt-management";
import type { PromptTemplate, PromptTemplateSchema } from "@/lib/types/prompt-management";

type PromptForm = {
  key: string;
  content: string;
  version: string;
  enabled: boolean;
};

const emptyForm: PromptForm = {
  key: "",
  content: "",
  version: "",
  enabled: true,
};

type PromptCatalogItem = PromptTemplateSchema & {
  tenantTemplate?: PromptTemplate;
};

function promptCoverageState(item: PromptCatalogItem) {
  if (!item.tenantTemplate) {
    return "default";
  }
  if (item.tenantTemplate.enabled ?? true) {
    return "overridden_enabled";
  }
  return "overridden_disabled";
}

function formatScopeSummary(items: PromptCatalogItem[]) {
  const seen = new Map<string, string>();
  for (const item of items) {
    for (const scope of item.supported_scopes ?? []) {
      if (!seen.has(scope.id)) {
        seen.set(scope.id, scope.label);
      }
    }
  }
  const labels = [...seen.values()];
  if (labels.length === 0) {
    return "作用域未定义";
  }
  return labels.join(" / ");
}

export function PromptTemplatesPanel() {
  const catalog = usePromptTemplateCatalog();
  const prompts = usePromptTemplates();
  const upsert = useUpsertPromptTemplate();
  const setStatus = useSetPromptTemplateStatus();
  const [groupFilter, setGroupFilter] = useState("all");
  const [coverageFilter, setCoverageFilter] = useState("all");
  const [defaultContentFilter, setDefaultContentFilter] = useState("all");
  const [variableFilter, setVariableFilter] = useState("all");
  const [searchQuery, setSearchQuery] = useState("");
  const items = useMemo(() => {
    const tenantTemplates = new Map((prompts.data?.items ?? []).map((item) => [item.key, item]));
    const catalogItems = (catalog.data?.items ?? []).map((item) => ({
      ...item,
      tenantTemplate: tenantTemplates.get(item.key),
    }));
    return [...catalogItems].sort((a, b) => {
      const categoryOrder = a.category_label.localeCompare(b.category_label);
      if (categoryOrder !== 0) {
        return categoryOrder;
      }
      return a.key.localeCompare(b.key);
    });
  }, [catalog.data?.items, prompts.data?.items]);
  const availableGroups = useMemo(() => {
    const seen = new Map<string, string>();
    for (const item of items) {
      if (!seen.has(item.group)) {
        seen.set(item.group, item.group_label);
      }
    }
    return [...seen.entries()].map(([value, label]) => ({ value, label }));
  }, [items]);
  const filteredItems = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();
    return items.filter((item) => {
      if (groupFilter !== "all" && item.group !== groupFilter) {
        return false;
      }
      if (coverageFilter !== "all" && promptCoverageState(item) !== coverageFilter) {
        return false;
      }
      if (defaultContentFilter === "present" && !item.has_default_content) {
        return false;
      }
      if (defaultContentFilter === "missing" && item.has_default_content) {
        return false;
      }
      const variableCount = item.variables?.length ?? 0;
      if (variableFilter === "with_variables" && variableCount === 0) {
        return false;
      }
      if (variableFilter === "without_variables" && variableCount > 0) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }
      const haystacks = [item.label, item.key, item.category_label, item.group_label];
      return haystacks.some((value) => value.toLowerCase().includes(normalizedQuery));
    });
  }, [coverageFilter, defaultContentFilter, groupFilter, items, searchQuery, variableFilter]);
  const groupedItems = useMemo(() => {
    const groups = new Map<string, Map<string, PromptCatalogItem[]>>();
    for (const item of filteredItems) {
      const categoryGroups = groups.get(item.group_label) ?? new Map<string, PromptCatalogItem[]>();
      const current = categoryGroups.get(item.category_label) ?? [];
      current.push(item);
      categoryGroups.set(item.category_label, current);
      groups.set(item.group_label, categoryGroups);
    }
    return [...groups.entries()].map(([groupLabel, categoryGroups]) => ({
      groupLabel,
      categories: [...categoryGroups.entries()],
    }));
  }, [filteredItems]);
  const [selectedKey, setSelectedKey] = useState("");
  const [draft, setDraft] = useState<PromptForm>(emptyForm);
  const selectedItem = useMemo(
    () => items.find((item) => item.key === (selectedKey || draft.key)) ?? null,
    [draft.key, items, selectedKey],
  );
  const scopeSummary = useMemo(() => formatScopeSummary(items), [items]);

  const selectPrompt = (item: PromptCatalogItem) => {
    setSelectedKey(item.key);
    setDraft({
      key: item.key,
      content: item.tenantTemplate?.content ?? "",
      version: item.tenantTemplate?.version ?? "",
      enabled: item.tenantTemplate?.enabled ?? true,
    });
  };

  const selectPromptByKey = (key: string) => {
    if (!key) {
      setSelectedKey("");
      setDraft(emptyForm);
      return;
    }
    const item = items.find((candidate) => candidate.key === key);
    if (!item) {
      return;
    }
    selectPrompt(item);
  };

  const updateDraft = <Key extends keyof PromptForm>(key: Key, value: PromptForm[Key]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const save = () => {
    upsert.mutate({
      key: draft.key.trim(),
      content: draft.content,
      version: draft.version.trim(),
      enabled: draft.enabled,
    });
  };

  const toggleEnabled = (item: PromptCatalogItem) => {
    const enabled = item.tenantTemplate?.enabled ?? true;
    setStatus.mutate({ key: item.key, enabled: !enabled });
  };

  return (
    <ListingKitSettingsSection
      id="prompts"
      eyebrow="Prompt 管理"
      title="当前客户提示词模板"
      description={`当前 catalog 支持的作用域：${scopeSummary}。禁用或缺失的模板会被后端视为不可用，不会回落到代码模板。`}
      actions={
        <Button
          onClick={() => {
            setSelectedKey("");
            setDraft(emptyForm);
          }}
        >
          新建租户覆盖
        </Button>
      }
    >
      <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(280px,0.9fr)_minmax(360px,1.1fr)]">
        <div className="min-h-[340px] overflow-hidden rounded-2xl border border-zinc-200 bg-zinc-50/70">
          <div className="flex items-center justify-between border-b border-zinc-200 bg-white px-3 py-2">
            <span className="text-sm font-semibold text-zinc-900">模板目录</span>
            <span className="text-xs text-zinc-500">
              {filteredItems.length} / {items.length} 个
            </span>
          </div>
          <div className="grid gap-2 border-b border-zinc-200 bg-white px-3 py-3 md:grid-cols-2 xl:grid-cols-5">
            <Input
              id="prompt-template-search"
              name="prompt-template-search"
              aria-label="搜索提示词模板"
              placeholder="搜索 key 或名称"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
            />
            <Select
              id="prompt-template-group-filter"
              name="prompt-template-group-filter"
              aria-label="用途分组"
              value={groupFilter}
              onChange={(event) => setGroupFilter(event.target.value)}
            >
              <option value="all">全部用途</option>
              {availableGroups.map((group) => (
                <option key={group.value} value={group.value}>
                  {group.label}
                </option>
              ))}
            </Select>
            <Select
              id="prompt-template-coverage-filter"
              name="prompt-template-coverage-filter"
              aria-label="覆盖状态"
              value={coverageFilter}
              onChange={(event) => setCoverageFilter(event.target.value)}
            >
              <option value="all">全部状态</option>
              <option value="default">仅默认模板</option>
              <option value="overridden_enabled">仅已覆盖并启用</option>
              <option value="overridden_disabled">仅已覆盖但禁用</option>
            </Select>
            <Select
              id="prompt-template-default-content-filter"
              name="prompt-template-default-content-filter"
              aria-label="默认模板"
              value={defaultContentFilter}
              onChange={(event) => setDefaultContentFilter(event.target.value)}
            >
              <option value="all">默认模板：全部</option>
              <option value="present">默认模板：存在</option>
              <option value="missing">默认模板：缺失</option>
            </Select>
            <Select
              id="prompt-template-variable-filter"
              name="prompt-template-variable-filter"
              aria-label="模板变量"
              value={variableFilter}
              onChange={(event) => setVariableFilter(event.target.value)}
            >
              <option value="all">变量：全部</option>
              <option value="with_variables">变量：有变量</option>
              <option value="without_variables">变量：无变量</option>
            </Select>
          </div>
          <div className="max-h-[520px] overflow-auto p-2">
            {catalog.isLoading || prompts.isLoading ? (
              <div className="px-3 py-6 text-sm text-zinc-500">加载中...</div>
            ) : null}
            {!catalog.isLoading && !prompts.isLoading && filteredItems.length === 0 ? (
              <div className="px-3 py-6 text-sm text-zinc-500">暂无提示词模板</div>
            ) : null}
            {groupedItems.map(({ groupLabel, categories }) => (
              <div key={groupLabel} className="mb-5">
                <div className="px-2 pb-2 text-xs font-semibold tracking-[0.08em] text-zinc-700">
                  {groupLabel}
                </div>
                {categories.map(([categoryLabel, categoryItems]) => (
                  <div key={`${groupLabel}-${categoryLabel}`} className="mb-4">
                    <div className="px-2 pb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                      {categoryLabel}
                    </div>
                    {categoryItems.map((item) => {
                      const enabled = item.tenantTemplate?.enabled ?? true;
                      const coverageState = promptCoverageState(item);
                      const active = selectedKey === item.key;
                      return (
                        <div
                          key={item.key}
                          className={[
                            "mb-2 grid grid-cols-[1fr_auto] gap-2 rounded-xl border p-2",
                            active
                              ? "border-zinc-900 bg-white shadow-sm"
                              : "border-zinc-200 bg-white/70",
                          ].join(" ")}
                        >
                          <Button
                            variant="ghost"
                            className="h-auto min-w-0 justify-start p-0 text-left hover:bg-transparent"
                            type="button"
                            onClick={() => selectPrompt(item)}
                          >
                            <span className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
                              <FileText className="h-4 w-4 shrink-0 text-teal-700" />
                              <span className="truncate">{item.label}</span>
                            </span>
                            <span className="mt-1 block truncate text-xs text-zinc-500">{item.key}</span>
                            <span className="mt-2 flex flex-wrap gap-2">
                              {coverageState === "default" ? (
                                <Badge variant="secondary">使用默认模板</Badge>
                              ) : null}
                              {coverageState === "overridden_enabled" ? (
                                <Badge variant="secondary">租户已覆盖</Badge>
                              ) : null}
                              {coverageState === "overridden_disabled" ? (
                                <Badge variant="destructive">租户已覆盖（已禁用）</Badge>
                              ) : null}
                              {item.tenantTemplate?.version ? (
                                <Badge variant="outline">版本 {item.tenantTemplate.version}</Badge>
                              ) : null}
                              <Badge variant="outline">{item.variables?.length ?? 0} 个变量</Badge>
                            </span>
                          </Button>
                          <Button
                            aria-label={enabled ? "禁用提示词" : "启用提示词"}
                            variant="outline"
                            size="icon"
                            className="size-8 text-zinc-600"
                            type="button"
                            onClick={() => toggleEnabled(item)}
                          >
                            {enabled ? <ToggleRight className="h-4 w-4" /> : <ToggleLeft className="h-4 w-4" />}
                          </Button>
                        </div>
                      );
                    })}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>

        <div className="rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4">
          {!selectedItem ? (
            <div className="flex min-h-[420px] items-center justify-center rounded-2xl border border-dashed border-zinc-300 bg-white/70 p-8 text-center">
              <div className="max-w-sm">
                <div className="text-sm font-semibold text-zinc-900">选择一个模板开始编辑</div>
                <div className="mt-2 text-sm leading-6 text-zinc-600">
                  从左侧目录选择 catalog 模板后，再为当前作用域创建或更新覆盖内容。
                </div>
              </div>
            </div>
          ) : (
            <>
              <div className="mb-4 rounded-xl border border-zinc-200 bg-white p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-semibold text-zinc-900">{selectedItem.label}</div>
                  <Badge>{selectedItem.group_label}</Badge>
                  <Badge variant="secondary">{selectedItem.category_label}</Badge>
                  {(selectedItem.supported_scopes ?? []).map((scope) => (
                    <Badge key={scope.id} variant="outline">
                      {scope.label}
                    </Badge>
                  ))}
                  {selectedItem.has_default_content ? (
                    <Badge variant="outline">有默认模板</Badge>
                  ) : (
                    <Badge variant="outline">无默认模板</Badge>
                  )}
                </div>
                <div className="mt-2 text-xs text-zinc-600">{selectedItem.description}</div>
                {selectedItem.variables?.length ? (
                  <div className="mt-3">
                    <div className="mb-2 text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                      模板变量
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2">
                      {selectedItem.variables.map((variable) => (
                        <div
                          key={variable.key}
                          className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2"
                        >
                          <div className="text-xs font-semibold text-zinc-900">
                            {variable.label || variable.key}
                          </div>
                          <div className="mt-1 font-mono text-[11px] text-zinc-500">
                            {variable.key}
                          </div>
                          {variable.description ? (
                            <div className="mt-1 text-[11px] leading-5 text-zinc-600">
                              {variable.description}
                            </div>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
              <div className="grid gap-3 lg:grid-cols-[1fr_160px_auto]">
                <Label className="space-y-1">
                  <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                    Prompt Key
                  </span>
                  <Select
                    aria-label="Prompt Key"
                    value={draft.key}
                    onChange={(event) => selectPromptByKey(event.target.value)}
                  >
                    <option value="">请选择模板</option>
                    {items.map((item) => (
                      <option key={item.key} value={item.key}>
                        {item.category_label} / {item.label}
                      </option>
                    ))}
                  </Select>
                </Label>
                <Label className="space-y-1">
                  <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                    版本
                  </span>
                  <Input
                    aria-label="版本"
                    value={draft.version}
                    onChange={(event) => updateDraft("version", event.target.value)}
                  />
                </Label>
                <Label className="mt-5 flex h-10 items-center gap-2 rounded-md border border-input bg-background px-3 text-sm text-foreground">
                  <Checkbox
                    checked={draft.enabled}
                    onChange={(event) => updateDraft("enabled", event.target.checked)}
                  />
                  启用
                </Label>
              </div>
              <Label className="mt-3 block space-y-1">
                <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                  Prompt 内容
                </span>
                <Textarea
                  aria-label="Prompt 内容"
                  className="min-h-[300px] resize-y font-mono leading-6"
                  value={draft.content}
                  onChange={(event) => updateDraft("content", event.target.value)}
                />
              </Label>
              <div className="mt-3 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
                <div className="text-xs text-zinc-500">
                  {draft.key
                    ? `保存后会按 ${selectedItem?.supported_scopes?.map((scope) => scope.label).join(" / ") || scopeSummary} 作用域生效`
                    : "先从左侧目录或下拉框选择已有模板，再创建作用域覆盖"}
                </div>
                <Button className="gap-2" disabled={upsert.isPending || !draft.key.trim()} onClick={save}>
                  <Save className="h-4 w-4" />
                  {upsert.isPending ? "保存中..." : "保存提示词"}
                </Button>
              </div>
            </>
          )}
          {catalog.isError || prompts.isError ? (
            <Alert className="mt-3" variant="destructive">
              <AlertDescription>提示词目录读取失败。</AlertDescription>
            </Alert>
          ) : null}
          {upsert.error || setStatus.error ? (
            <Alert className="mt-3" variant="destructive">
              <AlertDescription>提示词保存失败。</AlertDescription>
            </Alert>
          ) : null}
        </div>
      </div>
    </ListingKitSettingsSection>
  );
}
