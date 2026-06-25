"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import type {
  SDSRetirementItem,
  SDSRetirementRunDetail,
  SDSRetirementSelectionUpdate,
} from "@/lib/types/sds-retirement";

type RetirementSiteSelection = {
  site_abbr?: string;
  store_type?: number;
};

type Props = {
  detail: SDSRetirementRunDetail;
  isExecuting: boolean;
  onSelectionChange: (items: SDSRetirementSelectionUpdate[]) => void;
  onConfirm: () => void;
};

function parseSiteSelection(value?: string): RetirementSiteSelection[] {
  if (!value?.trim()) {
    return [];
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.flatMap((entry) => {
      if (!entry || typeof entry !== "object") {
        return [];
      }
      const candidate = entry as RetirementSiteSelection;
      return [
        {
          site_abbr:
            typeof candidate.site_abbr === "string" ? candidate.site_abbr : undefined,
          store_type:
            typeof candidate.store_type === "number" ? candidate.store_type : undefined,
        },
      ];
    });
  } catch {
    return [];
  }
}

function buildSiteSelectionValue(items: RetirementSiteSelection[]) {
  if (items.length === 0) {
    return "";
  }
  return JSON.stringify(items);
}

function buildSiteKey(site: RetirementSiteSelection) {
  return `${site.site_abbr ?? ""}:${site.store_type ?? ""}`;
}

function mergeSiteSelections(...groups: RetirementSiteSelection[][]) {
  const merged = new Map<string, RetirementSiteSelection>();
  groups.flat().forEach((site) => {
    merged.set(buildSiteKey(site), site);
  });
  return Array.from(merged.values());
}

function buildCandidateSitesByItem(items: SDSRetirementItem[]) {
  return Object.fromEntries(
    items.map((item) => [item.id, parseSiteSelection(item.site_selection)]),
  );
}

function itemCanChange(item: SDSRetirementItem) {
  return item.status === "pending" || item.status === "selected";
}

function itemStatusLabel(item: SDSRetirementItem) {
  switch (item.status) {
    case "pending":
      return "待确认";
    case "selected":
      return "已选中";
    case "running":
      return "执行中";
    case "succeeded":
      return "已下架";
    case "succeeded_already_off_shelf":
      return "已是下架";
    case "failed":
      return "执行失败";
    case "skipped":
      return "已跳过";
    default:
      return item.status;
  }
}

export function SDSRetirementPanel({
  detail,
  isExecuting,
  onSelectionChange,
  onConfirm,
}: Props) {
  const [acknowledged, setAcknowledged] = useState(false);
  const [candidateSiteState, setCandidateSiteState] = useState<{
    runId: string;
    byItemId: Record<string, RetirementSiteSelection[]>;
  }>(() => ({
    runId: detail.run.id,
    byItemId: buildCandidateSitesByItem(detail.items),
  }));

  const candidateSitesByItem = useMemo(() => {
    const nextCandidates = buildCandidateSitesByItem(detail.items);
    if (candidateSiteState.runId !== detail.run.id) {
      return nextCandidates;
    }
    return Object.fromEntries(
      detail.items.map((item) => [
        item.id,
        mergeSiteSelections(
          candidateSiteState.byItemId[item.id] ?? [],
          nextCandidates[item.id] ?? [],
        ),
      ]),
    );
  }, [candidateSiteState, detail.items, detail.run.id]);

  const selectedCount = useMemo(
    () =>
      detail.items.filter(
        (item) =>
          item.selected &&
          itemCanChange(item) &&
          (!item.site_selection || parseSiteSelection(item.site_selection).length > 0),
      ).length,
    [detail.items],
  );

  const canConfirm =
    acknowledged && selectedCount > 0 && detail.run.status === "ready" && !isExecuting;

  function getCandidateSites(item: SDSRetirementItem) {
    return candidateSitesByItem[item.id] ?? parseSiteSelection(item.site_selection);
  }

  function toggleItem(item: SDSRetirementItem, selected: boolean) {
    onSelectionChange([
      {
        item_id: item.id,
        selected,
        site_selection: item.site_selection,
      },
    ]);
  }

  function toggleSite(
    item: SDSRetirementItem,
    site: RetirementSiteSelection,
    selected: boolean,
  ) {
    setCandidateSiteState({
      runId: detail.run.id,
      byItemId: candidateSitesByItem,
    });
    const currentSites = parseSiteSelection(item.site_selection);
    const nextSites = selected
      ? [
          ...currentSites,
          site,
        ].filter((candidate, index, all) => {
          const key = `${candidate.site_abbr ?? ""}:${candidate.store_type ?? ""}`;
          return all.findIndex((entry) => `${entry.site_abbr ?? ""}:${entry.store_type ?? ""}` === key) === index;
        })
      : currentSites.filter(
          (candidate) =>
            candidate.site_abbr !== site.site_abbr || candidate.store_type !== site.store_type,
        );
    onSelectionChange([
      {
        item_id: item.id,
        selected: nextSites.length > 0,
        site_selection: buildSiteSelectionValue(nextSites),
      },
    ]);
  }

  return (
    <Card className="min-w-0 space-y-3 p-4">
      <div className="space-y-1">
        <h2 className="text-base font-semibold text-foreground">SDS 底版下架处置</h2>
        <p className="text-sm text-muted-foreground">
          {detail.run.reason || detail.reason || "请确认需要下架的 SHEIN 商品。"}
        </p>
      </div>

      <div className="space-y-2">
        {detail.items.map((item) => {
          const siteSelections = getCandidateSites(item);
          const selectedSites = parseSiteSelection(item.site_selection);
          const editable = itemCanChange(item) && !isExecuting;
          return (
            <div
              className="min-w-0 rounded-md border border-border px-3 py-2"
              key={item.id}
            >
              <label className="flex min-w-0 items-start gap-3">
                <Checkbox
                  aria-label={item.skc_name || item.id}
                  checked={item.selected}
                  disabled={!editable}
                  onChange={(event) => toggleItem(item, event.currentTarget.checked)}
                />
                <span className="min-w-0 flex-1 space-y-1">
                  <span className="block truncate text-sm font-medium text-foreground">
                    {item.skc_name || item.id}
                  </span>
                  <span className="block text-xs text-muted-foreground">
                    {item.spu_name || "-"} · {itemStatusLabel(item)}
                  </span>
                  {item.error ? (
                    <span className="block text-xs text-rose-700">{item.error}</span>
                  ) : null}
                </span>
              </label>
              {siteSelections.length > 0 ? (
                <div className="mt-2 flex flex-wrap gap-2 pl-7">
                  {siteSelections.map((site) => {
                    const label = site.site_abbr?.trim() || "站点";
                    return (
                      <label
                        className="inline-flex max-w-full items-center gap-2 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground"
                        key={`${item.id}:${label}:${site.store_type ?? ""}`}
                      >
                        <Checkbox
                          aria-label={`${item.skc_name || item.id} ${label}`}
                          checked={selectedSites.some(
                            (candidate) => buildSiteKey(candidate) === buildSiteKey(site),
                          )}
                          disabled={!editable}
                          onChange={(event) =>
                            toggleSite(item, site, event.currentTarget.checked)
                          }
                        />
                        <span className="truncate">{label}</span>
                      </label>
                    );
                  })}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>

      <label className="flex items-start gap-2 text-sm text-foreground">
        <Checkbox
          aria-label="我确认所选 SHEIN 商品将执行下架操作"
          checked={acknowledged}
          disabled={isExecuting}
          onChange={(event) => setAcknowledged(event.currentTarget.checked)}
        />
        <span className="min-w-0">我确认所选 SHEIN 商品将执行下架操作</span>
      </label>

      <Button
        className="w-full sm:w-auto"
        disabled={!canConfirm}
        onClick={onConfirm}
        type="button"
        variant="danger"
      >
        {isExecuting ? "下架中..." : `确认下架 ${selectedCount} 个商品`}
      </Button>
    </Card>
  );
}
