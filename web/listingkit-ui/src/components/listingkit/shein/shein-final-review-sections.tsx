import type { Dispatch, SetStateAction } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  imageRoleLabel,
  imageRoleTone,
  money,
  type ReviewSummaryItem,
  summaryStatusLabel,
  summaryTone,
} from "@/components/listingkit/shein/shein-final-review-helpers";
import type {
  SheinFinalReviewImage,
  SheinPreviewPayload,
  SheinResolvedSaleAttribute,
  SheinSizeAttribute,
} from "@/lib/types/listingkit";

export function FailureGuidance({
  title,
  detail,
  impact,
  nextStep,
}: {
  title: string;
  detail: string;
  impact: string;
  nextStep: string;
}) {
  return (
    <div className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
      <div className="space-y-3">
        <p className="font-semibold text-rose-900">{title}</p>
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
            发生了什么
          </p>
          <p className="leading-6">{detail}</p>
        </div>
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
            可能影响
          </p>
          <p className="leading-6">{impact}</p>
        </div>
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
            下一步怎么做
          </p>
          <p className="leading-6">{nextStep}</p>
        </div>
      </div>
    </div>
  );
}

export function ReviewSummaryGrid({
  items,
  onSelectItem,
}: {
  items: ReviewSummaryItem[];
  onSelectItem?: (item: ReviewSummaryItem) => void;
}) {
  return (
    <div className="space-y-3">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
          检查项概览
        </p>
        <p className="mt-1 text-sm leading-6 text-zinc-600">
          只显示客户提交前需要确认的关键项。
        </p>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        {items.map((item) => (
          <div
            className={`rounded-2xl border p-3 ${summaryTone(item.status)}`}
            key={item.key}
          >
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-semibold">{item.title}</p>
                  <Badge
                    className="rounded-full bg-white/75 px-2 py-1 text-[10px]"
                    variant="neutral"
                  >
                    {summaryStatusLabel(item.status)}
                  </Badge>
                </div>
                <p className="mt-1 text-sm leading-6">{item.message}</p>
              </div>
              {item.status !== "done" && onSelectItem ? (
                <Button
                  className="h-8 shrink-0 px-3 text-xs"
                  variant="secondary"
                  onClick={() => onSelectItem(item)}
                >
                  {item.actionLabel ?? "去修复"}
                </Button>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function ImageSubmitSummary({
  finalCount,
  galleryCount,
  imageBlocked,
  mainCount,
  sizeMapCount,
  skcCount,
  swatchCount,
  onSelectImages,
}: {
  finalCount: number;
  galleryCount: number;
  imageBlocked: boolean;
  mainCount: number;
  sizeMapCount: number;
  skcCount: number;
  swatchCount: number;
  onSelectImages?: () => void;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
            图片提交摘要
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            最终图片 {finalCount} 张 · 主图 {mainCount} 张 · 色块图{" "}
            {swatchCount} 张 · SKC 图 {skcCount} 张 · 尺寸图 {sizeMapCount} 张 · 图库{" "}
            {galleryCount} 张
          </p>
        </div>
        {imageBlocked && onSelectImages ? (
          <Button
            className="h-8 w-full px-3 text-xs sm:w-auto"
            variant="secondary"
            onClick={onSelectImages}
          >
            去检查图片
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function ImageStructureDetails({
  finalImages,
  imageBlocked,
  mainCount,
  swatchCount,
  finalCount,
}: {
  finalImages: SheinFinalReviewImage[];
  imageBlocked: boolean;
  mainCount: number;
  swatchCount: number;
  finalCount: number;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
            图片结构明细
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-600">
            这里按提交前角色展示最终图片。单变体可直接使用首图作为色块和 SKC
            图来源；色块来源会在提交时生成纯色色块图后上传 SHEIN。
          </p>
        </div>
        {imageBlocked ? (
          <Badge className="rounded-full px-3 py-1 text-xs" variant="warning">
            图片资料需处理
          </Badge>
        ) : (
          <Badge className="rounded-full px-3 py-1 text-xs" variant="success">
            图片结构完整
          </Badge>
        )}
      </div>
      <div className="mt-3 grid gap-2 sm:grid-cols-2 2xl:grid-cols-3">
        {finalImages.map((image, index) => (
          <div
            className="min-w-0 rounded-2xl border border-zinc-100 bg-zinc-50 p-3"
            key={`${image.url}-${index}`}
          >
            <div className="flex items-center justify-between gap-2">
              <Badge
                className={`rounded-full px-2 py-1 text-[10px] ${imageRoleTone(image)}`}
                variant="neutral"
              >
                {imageRoleLabel(image)}
              </Badge>
              <span className="text-[11px] text-zinc-500">
                排序 {image.sort ?? index + 1}
              </span>
            </div>
            <p className="mt-2 truncate text-xs text-zinc-600" title={image.url}>
              {image.url}
            </p>
          </div>
        ))}
        {finalImages.length === 0 ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
            还没有最终提交图片，请先回到图片区域确认。
          </div>
        ) : null}
      </div>
      <div className="mt-3 grid gap-2 text-sm sm:grid-cols-3">
        {mainCount === 0 ? (
          <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
            缺主图
          </div>
        ) : null}
        {imageBlocked && swatchCount === 0 ? (
          <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
            缺色块来源图
          </div>
        ) : null}
        {finalCount === 0 ? (
          <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
            缺最终提交图片
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function FinalReviewOverviewCards({
  finalReview,
}: {
  finalReview?: SheinPreviewPayload["final_review"];
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <div className="rounded-2xl border border-zinc-100 bg-zinc-50 p-3">
        <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
          商品
        </div>
        <div className="mt-1 text-sm font-semibold text-zinc-950">
          {finalReview?.title || "未命名 SHEIN 商品"}
        </div>
        <div className="mt-1 text-xs text-zinc-500">
          {finalReview?.category_path?.join(" > ") || "未匹配类目"}
        </div>
      </div>
      <div className="rounded-2xl border border-zinc-100 bg-zinc-50 p-3">
        <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
          图片
        </div>
        <div className="mt-1 text-sm font-semibold text-zinc-950">
          最终提交 {finalReview?.images?.length ?? 0} 张
        </div>
        <div className="mt-1 text-xs text-zinc-500">
          主图、色块图和图库需在 SHEIN data images 中确认。
        </div>
      </div>
      <div className="rounded-2xl border border-zinc-100 bg-zinc-50 p-3">
        <div className="text-[10px] font-semibold uppercase tracking-[0.22em] text-zinc-500">
          SKU
        </div>
        <div className="mt-1 text-sm font-semibold text-zinc-950">
          {finalReview?.skus?.length ?? 0} SKUs
        </div>
        <div className="mt-1 text-xs text-zinc-500">
          价格来自 SDS 人民币成本换算，可在下方覆盖。
        </div>
      </div>
    </div>
  );
}

export function StoreResolutionCard({
  resolution,
}: {
  resolution?: SheinPreviewPayload["store_resolution"];
}) {
  if (!resolution?.store_id) {
    return null;
  }
  return (
    <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
            店铺解析
          </p>
          <p className="mt-1 text-sm font-semibold text-zinc-950">
            SHEIN 店铺 {resolution.store_id}
            {resolution.site ? ` · ${resolution.site}` : ""}
          </p>
          {resolution.reason ? (
            <p className="mt-1 text-sm leading-6 text-zinc-600">{resolution.reason}</p>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          {resolution.manual_override ? (
            <Badge className="rounded-full px-2 py-1 text-[10px]" variant="success">
              手工指定
            </Badge>
            ) : null}
        </div>
      </div>
      {resolution.matched_profile_id || resolution.resolved_at ? (
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
          {resolution.matched_profile_id ? (
            <span>Profile #{resolution.matched_profile_id}</span>
          ) : null}
          {resolution.resolved_at ? (
            <span>固化时间：{formatStoreResolutionTime(resolution.resolved_at)}</span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function formatStoreResolutionTime(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

type SizeAttributeTableModel = {
  columns: Array<{ attributeID: number; label: string }>;
  rows: Array<{
    label: string;
    saleAttributeValueID: number;
    values: Record<number, string>;
  }>;
};

export function SizeAttributeTable({
  shein,
}: {
  shein?: SheinPreviewPayload | null;
}) {
  const model = buildSizeAttributeTableModel(shein);
  if (!model) {
    return null;
  }

  const gridTemplateColumns = `minmax(7rem, 0.8fr) repeat(${model.columns.length}, minmax(8rem, 1fr))`;

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
            SHEIN 尺码表
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-600">
            {model.rows.length} 个尺码 · {model.columns.length} 个尺码字段
          </p>
        </div>
        <Badge className="w-fit rounded-full px-3 py-1 text-xs" variant="success">
          已生成
        </Badge>
      </div>
      <div className="mt-3 overflow-hidden rounded-2xl border border-zinc-200">
        <div className="overflow-x-auto">
          <div className="min-w-[32rem]">
            <div
              className="grid bg-zinc-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500"
              style={{ gridTemplateColumns }}
            >
              <span>尺码</span>
              {model.columns.map((column) => (
                <span key={column.attributeID}>{column.label}</span>
              ))}
            </div>
            <div className="divide-y divide-zinc-100">
              {model.rows.map((row) => (
                <div
                  className="grid items-center gap-2 px-3 py-2 text-sm"
                  key={row.saleAttributeValueID}
                  style={{ gridTemplateColumns }}
                >
                  <span className="font-medium text-zinc-950">{row.label}</span>
                  {model.columns.map((column) => (
                    <span className="text-zinc-700" key={column.attributeID}>
                      {row.values[column.attributeID] || "-"}
                    </span>
                  ))}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export function buildSizeAttributeTableModel(
  shein?: SheinPreviewPayload | null,
): SizeAttributeTableModel | null {
  const sizeAttributes = firstNonEmptySizeAttributes(
    shein?.preview_payload?.size_attribute_list,
    shein?.preview_product?.size_attribute_list,
    shein?.draft_payload?.size_attribute_list,
    shein?.request_draft?.size_attribute_list,
  );
  if (sizeAttributes.length === 0) {
    return null;
  }

  const columnLabels = buildSizeAttributeColumnLabels(shein);
  const saleValueLabels = buildSaleAttributeValueLabels(shein);
  const columns: SizeAttributeTableModel["columns"] = [];
  const rows: SizeAttributeTableModel["rows"] = [];
  const columnIDs = new Set<number>();
  const rowByValueID = new Map<number, SizeAttributeTableModel["rows"][number]>();

  for (const item of sizeAttributes) {
    const attributeID = item.attribute_id ?? 0;
    const saleAttributeValueID = item.relate_sale_attribute_value_id ?? 0;
    const value = String(item.attribute_extra_value ?? "").trim();
    if (attributeID <= 0 || saleAttributeValueID <= 0 || !value) {
      continue;
    }
    if (!columnIDs.has(attributeID)) {
      columnIDs.add(attributeID);
      columns.push({
        attributeID,
        label: columnLabels.get(attributeID) ?? `attribute_id ${attributeID}`,
      });
    }
    let row = rowByValueID.get(saleAttributeValueID);
    if (!row) {
      row = {
        label: saleValueLabels.get(saleAttributeValueID) ?? String(saleAttributeValueID),
        saleAttributeValueID,
        values: {},
      };
      rowByValueID.set(saleAttributeValueID, row);
      rows.push(row);
    }
    row.values[attributeID] = value;
  }

  if (columns.length === 0 || rows.length === 0) {
    return null;
  }
  return { columns, rows };
}

function firstNonEmptySizeAttributes(
  ...candidates: Array<SheinSizeAttribute[] | undefined>
) {
  return candidates.find((candidate) => candidate && candidate.length > 0) ?? [];
}

function buildSizeAttributeColumnLabels(shein?: SheinPreviewPayload | null) {
  const labels = new Map<number, string>();
  for (const attribute of
    shein?.editor_context?.attributes?.current?.size_chart_attributes ?? []) {
    const attributeID = attribute.attribute_id ?? 0;
    if (attributeID <= 0) {
      continue;
    }
    const label =
      attribute.attribute_name_en ||
      attribute.attribute_name ||
      attribute.name ||
      `attribute_id ${attributeID}`;
    labels.set(attributeID, label);
  }
  return labels;
}

function buildSaleAttributeValueLabels(shein?: SheinPreviewPayload | null) {
  const labels = new Map<number, string>();
  const addResolved = (attribute?: SheinResolvedSaleAttribute) => {
    const valueID = attribute?.attribute_value_id ?? 0;
    const value = String(attribute?.value ?? "").trim();
    if (valueID > 0 && value && !labels.has(valueID)) {
      labels.set(valueID, value);
    }
  };

  for (const attribute of
    shein?.editor_context?.sale_attributes?.current?.sku_attributes ?? []) {
    addResolved(attribute);
  }
  for (const skc of shein?.draft_payload?.skc_list ?? []) {
    addResolved(skc.sale_attribute);
    for (const sku of skc.sku_list ?? []) {
      for (const attribute of sku.sale_attributes ?? []) {
        addResolved(attribute);
      }
    }
  }
  for (const skc of shein?.request_draft?.skc_list ?? []) {
    addResolved(skc.sale_attribute);
    for (const sku of skc.sku_list ?? []) {
      for (const attribute of sku.sale_attributes ?? []) {
        addResolved(attribute);
      }
    }
  }
  for (const attribute of shein?.final_review?.sale_attributes ?? []) {
    addResolved(attribute);
  }

  return labels;
}

export function SkuPricingTable({
  priceOverrides,
  pricing,
  setPriceOverrides,
}: {
  priceOverrides: Record<string, string>;
  pricing?: SheinPreviewPayload["pricing"];
  setPriceOverrides: Dispatch<SetStateAction<Record<string, string>>>;
}) {
  return (
    <>
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
          SKU 价格确认
        </p>
        <p className="mt-1 text-sm leading-6 text-zinc-600">
          价格来自 SDS 人民币成本换算，提交前可人工覆盖单个 SKU 售价。
        </p>
      </div>
      <div
        id="shein-final-review-pricing"
        className="scroll-mt-6 overflow-hidden rounded-2xl border border-zinc-200"
      >
        <div className="overflow-x-auto">
          <div className="min-w-[38rem]">
            <div className="grid grid-cols-[1.5fr_0.7fr_0.7fr_0.8fr] bg-zinc-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span>SKU</span>
              <span>成本</span>
              <span>自动价</span>
              <span>最终售价</span>
            </div>
            <div className="max-h-72 divide-y divide-zinc-100 overflow-auto">
              {(pricing?.sku_prices ?? []).map((sku) => (
                <div
                  key={sku.supplier_sku}
                  className="grid grid-cols-[1.5fr_0.7fr_0.7fr_0.8fr] items-center gap-2 px-3 py-2 text-sm"
                >
                  <span className="truncate font-medium text-zinc-900">
                    {sku.supplier_sku}
                  </span>
                  <span className="text-zinc-600">CNY {sku.cost_cny ?? "-"}</span>
                  <span className="text-zinc-600">
                    {money(sku.calculated_price, sku.currency)}
                  </span>
                  <Input
                    className="h-9 rounded-xl"
                    value={priceOverrides[sku.supplier_sku ?? ""] ?? ""}
                    onChange={(event) =>
                      setPriceOverrides((current) => ({
                        ...current,
                        [sku.supplier_sku ?? ""]: event.target.value,
                      }))
                    }
                  />
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

