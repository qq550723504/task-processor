"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/shared/button";
import { SheinCustomerIssueSummary } from "@/components/listingkit/shein/shein-customer-issue-summary";
import {
  buildSheinCustomerIssues,
  type CustomerIssue,
} from "@/lib/shein-studio/shein-customer-issues";
import type {
  SheinFinalReviewImage,
  SheinPreviewPayload,
  SheinReadinessItem,
} from "@/lib/types/listingkit";

type Props = {
  shein?: SheinPreviewPayload | null;
  isSaving?: boolean;
  isSubmitting?: boolean;
  submitAction?: "publish" | "save_draft" | null;
  submitErrorMessage?: string | null;
  saveMessage?: string | null;
  saveErrorMessage?: string | null;
  canSelectBlockingItem?: (item: SheinReadinessItem) => boolean;
  onSelectBlockingItem?: (item: SheinReadinessItem) => void;
  onSaveFinalDraft?: (payload: {
    confirmed?: boolean;
    submit_mode?: "publish" | "save_draft";
    manual_price_overrides?: Record<string, number>;
  }) => void;
  onSubmit?: (action: "publish" | "save_draft") => void;
};

function money(value?: number, currency?: string) {
  if (!value || value <= 0) {
    return "-";
  }
  return `${currency ?? "USD"} ${value.toFixed(2)}`;
}

type ReviewSummaryItem = {
  key: "category" | "attributes" | "sale_attributes" | "images";
  title: string;
  message: string;
  status: "done" | "blocked" | "warning";
  actionLabel?: string;
};

function hasBlockingKey(items: SheinReadinessItem[], keys: string[]) {
  return items.some((item) => keys.includes(item.key ?? ""));
}

function imageRoleCounts(images?: SheinFinalReviewImage[]) {
  const counts = {
    final: images?.filter((image) => image.final !== false).length ?? 0,
    main: 0,
    swatch: 0,
    sizeMap: 0,
    skc: 0,
    gallery: 0,
  };
  for (const image of images ?? []) {
    if (image.final === false) {
      continue;
    }
    if (image.main || image.role === "main") counts.main += 1;
    else if (image.swatch || image.role === "swatch") counts.swatch += 1;
    else if (image.size_map || image.role === "size_map") counts.sizeMap += 1;
    else if (image.role === "skc") counts.skc += 1;
    else counts.gallery += 1;
  }
  return counts;
}

function imageRoleLabel(image: SheinFinalReviewImage) {
  if (image.main || image.role === "main") return "主图";
  if (image.role === "swatch" || image.swatch) return "色块来源";
  if (image.role === "size_map" || image.size_map) return "尺寸图";
  if (image.role === "skc") return "SKC 图";
  if (image.role === "white_bg") return "白底图";
  return "图库";
}

function imageRoleTone(image: SheinFinalReviewImage) {
  if (image.main || image.role === "main") return "bg-zinc-950 text-white";
  if (image.role === "swatch" || image.swatch) return "bg-amber-100 text-amber-800";
  if (image.role === "size_map" || image.size_map) return "bg-sky-100 text-sky-800";
  if (image.role === "skc") return "bg-emerald-100 text-emerald-800";
  return "bg-zinc-100 text-zinc-700";
}

function summaryTone(status: ReviewSummaryItem["status"]) {
  switch (status) {
    case "blocked":
      return "border-amber-200 bg-amber-50 text-amber-900";
    case "warning":
      return "border-sky-200 bg-sky-50 text-sky-900";
    default:
      return "border-emerald-200 bg-emerald-50 text-emerald-900";
  }
}

function summaryStatusLabel(status: ReviewSummaryItem["status"]) {
  switch (status) {
    case "blocked":
      return "需处理";
    case "warning":
      return "建议检查";
    default:
      return "已完成";
  }
}

function FailureGuidance({
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

export function SheinFinalReviewPanel({
  shein,
  isSaving,
  isSubmitting,
  submitAction,
  submitErrorMessage,
  saveMessage,
  saveErrorMessage,
  canSelectBlockingItem,
  onSelectBlockingItem,
  onSaveFinalDraft,
  onSubmit,
}: Props) {
  const [isPublishConfirming, setIsPublishConfirming] = useState(false);
  const finalReview = shein?.final_review;
  const pricing = shein?.pricing;
  const [priceOverrides, setPriceOverrides] = useState<Record<string, string>>(
    () =>
      Object.fromEntries(
        pricing?.sku_prices?.map((sku) => [
          sku.supplier_sku ?? "",
          String(sku.final_price ?? ""),
        ]) ?? [],
      ),
  );

  const manualOverrides = useMemo(() => {
    const out: Record<string, number> = {};
    for (const [sku, value] of Object.entries(priceOverrides)) {
      const price = Number(value);
      if (sku && Number.isFinite(price) && price > 0) {
        out[sku] = price;
      }
    }
    return out;
  }, [priceOverrides]);
  const customerIssues = useMemo(() => buildSheinCustomerIssues(shein), [shein]);

  if (!finalReview && !pricing) {
    return null;
  }

  const blockers = finalReview?.blocking_items ?? [];
  const confirmed = finalReview?.confirmed === true;
  const ready = shein?.submit_readiness?.ready === true;
  const readinessBlockers = shein?.submit_readiness?.blocking_items ?? [];
  const visibleBlockers = blockers.length ? blockers : readinessBlockers;
  const allBlockingItems = [...readinessBlockers, ...blockers];
  const customerBlockingCount = customerIssues.filter(
    (issue) => issue.severity !== "warning",
  ).length;
  const blockingCount = customerBlockingCount || visibleBlockers.length;
  const imageCounts = imageRoleCounts(finalReview?.images);
  const finalImages = (finalReview?.images ?? []).filter(
    (image) => image.final !== false && image.url,
  );
  const attributeResolution = shein?.editor_context?.attributes?.current;
  const finalAttributeCount = finalReview?.attributes?.length ?? 0;
  const resolvedAttributeCount =
    finalAttributeCount || attributeResolution?.resolved_count || 0;
  const attributeBlocked = hasBlockingKey(allBlockingItems, [
    "attributes",
    "attribute_review",
  ]);
  const attributeResolvedByState =
    !attributeBlocked &&
    attributeResolution?.status === "resolved" &&
    resolvedAttributeCount > 0;
  const attributeDone = !attributeBlocked && (finalAttributeCount > 0 || attributeResolvedByState);
  const attributeMessage =
    finalAttributeCount > 0
      ? `已确认 ${finalAttributeCount} 个普通属性`
      : attributeResolvedByState && attributeResolution?.source === "manual_fallback_review"
        ? `已按当前 SDS 属性确认 ${resolvedAttributeCount} 个普通属性`
        : attributeResolvedByState
          ? `已确认 ${resolvedAttributeCount} 个普通属性`
          : "普通属性未展示已确认结果，建议检查必填属性。";
  const imageBlocked =
    hasBlockingKey(allBlockingItems, ["images", "final_images", "preview_product"]) ||
    imageCounts.final === 0 ||
    imageCounts.main === 0;
  const summaryItems: ReviewSummaryItem[] = [
    {
      key: "category",
      title: "类目确认",
      status: hasBlockingKey(allBlockingItems, ["category", "category_review"])
        ? "blocked"
        : finalReview?.category_id
          ? "done"
          : "warning",
      message: finalReview?.category_id
        ? `已选择类目 ${finalReview.category_id}`
        : "还没有明确的 SHEIN 类目 ID，请确认类目后再提交。",
      actionLabel: "去确认类目",
    },
    {
      key: "attributes",
      title: "普通属性",
      status: attributeBlocked ? "blocked" : attributeDone ? "done" : "warning",
      message: attributeMessage,
      actionLabel: "去确认属性",
    },
    {
      key: "sale_attributes",
      title: "销售属性",
      status: hasBlockingKey(allBlockingItems, ["sale_attributes", "variants"])
        ? "blocked"
        : (finalReview?.sale_attributes?.length ?? 0) > 0
          ? "done"
          : "warning",
      message:
        (finalReview?.sale_attributes?.length ?? 0) > 0
          ? `已映射 ${finalReview?.sale_attributes?.length ?? 0} 个销售属性`
          : "销售属性未展示已映射结果，建议检查主规格和其他规格。",
      actionLabel: "去确认销售属性",
    },
    {
      key: "images",
      title: "图片资料",
      status: imageBlocked ? "blocked" : "done",
      message: imageBlocked
        ? "主图、色块图或最终提交图片不完整，请先检查图片角色。"
        : `最终 ${imageCounts.final} 张，主图 ${imageCounts.main} 张，色块图 ${imageCounts.swatch} 张`,
      actionLabel: "去检查图片",
    },
  ];
  const submitHint = !ready
    ? `还差 ${blockingCount} 个阻断项，修复后才能提交。`
    : !confirmed
      ? "资料已通过检查，请先确认最终草稿。"
      : "可以保存到 SHEIN 草稿箱，也可以正式发布。";
  const canSelectIssue = (issue: CustomerIssue) =>
    Boolean(
      issue.actionKey &&
        canSelectBlockingItem?.({
          key: issue.actionKey,
          label: issue.title,
          message: issue.message,
        }),
    );
  const handleSelectIssue = (issue: CustomerIssue) => {
    if (!issue.actionKey) {
      return;
    }
    onSelectBlockingItem?.({
      key: issue.actionKey,
      label: issue.title,
      message: issue.message,
    });
  };
  const handleSelectSummaryItem = (item: ReviewSummaryItem) => {
    onSelectBlockingItem?.({
      key: item.key,
      label: item.title,
      message: item.message,
    });
  };

  return (
    <section className="space-y-4 rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
            SHEIN 最终确认
          </p>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">
            确认即将提交的资料
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-zinc-600">
            发布前核对价格、SKU、属性和最终图片。保存最终草稿后才能从这里提交。
          </p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold ${
            confirmed
              ? "bg-emerald-100 text-emerald-700"
              : "bg-amber-100 text-amber-700"
          }`}
        >
          {confirmed ? "已确认" : "待确认"}
        </span>
      </div>

      <div
        className={`rounded-2xl border p-4 ${
          ready
            ? "border-emerald-200 bg-emerald-50"
            : "border-amber-200 bg-amber-50"
        }`}
      >
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div
              className={`text-xs font-semibold uppercase tracking-[0.18em] ${
                ready ? "text-emerald-700" : "text-amber-700"
              }`}
            >
              提交前检查
            </div>
            <div className="mt-1 text-sm font-semibold text-zinc-950">
              {ready
                ? confirmed
                  ? "可以提交"
                  : "资料已就绪，还需要最终确认"
                : "暂时不能提交"}
            </div>
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              {ready
                ? "后端 readiness 已通过。提交前请确认价格、图片和 SKU。"
                : "需要先修复阻断项，提交按钮会保持不可用。"}
            </p>
          </div>
          <span
            className={`rounded-full px-3 py-1 text-xs font-semibold ${
              ready ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"
            }`}
          >
            {ready ? "已就绪" : `${blockingCount} 个阻断项`}
          </span>
        </div>
      </div>

      <SheinCustomerIssueSummary
        issues={customerIssues}
        canSelectIssue={canSelectIssue}
        onSelectIssue={handleSelectIssue}
      />

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
          {summaryItems.map((item) => (
            <div
              className={`rounded-2xl border p-3 ${summaryTone(item.status)}`}
              key={item.key}
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-semibold">{item.title}</p>
                    <span className="rounded-full bg-white/75 px-2 py-1 text-[10px] font-semibold">
                      {summaryStatusLabel(item.status)}
                    </span>
                  </div>
                  <p className="mt-1 text-sm leading-6">{item.message}</p>
                </div>
                {item.status !== "done" && onSelectBlockingItem ? (
                  <Button
                    className="h-8 shrink-0 px-3 text-xs"
                    tone="secondary"
                    onClick={() => handleSelectSummaryItem(item)}
                  >
                    {item.actionLabel ?? "去修复"}
                  </Button>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
              图片提交摘要
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              最终图片 {imageCounts.final} 张 · 主图 {imageCounts.main} 张 · 色块图{" "}
              {imageCounts.swatch} 张 · SKC 图 {imageCounts.skc} 张 · 尺寸图{" "}
              {imageCounts.sizeMap} 张 · 图库 {imageCounts.gallery} 张
            </p>
          </div>
          {imageBlocked && onSelectBlockingItem ? (
            <Button
              className="h-8 px-3 text-xs"
              tone="secondary"
              onClick={() =>
                handleSelectSummaryItem({
                  key: "images",
                  title: "图片资料",
                  message: "请检查最终提交图片角色。",
                  status: "blocked",
                })
              }
            >
              去检查图片
            </Button>
          ) : null}
        </div>
      </div>

      <div className="rounded-2xl border border-zinc-200 bg-white p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
              图片结构明细
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-600">
              这里按提交前角色展示最终图片。单变体可直接使用首图作为色块和 SKC 图来源；色块来源会在提交时生成纯色色块图后上传 SHEIN。
            </p>
          </div>
          {imageBlocked ? (
            <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-800">
              图片资料需处理
            </span>
          ) : (
            <span className="rounded-full bg-emerald-100 px-3 py-1 text-xs font-semibold text-emerald-800">
              图片结构完整
            </span>
          )}
        </div>
        <div className="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {finalImages.map((image, index) => (
            <div
              className="min-w-0 rounded-2xl border border-zinc-100 bg-zinc-50 p-3"
              key={`${image.url}-${index}`}
            >
              <div className="flex items-center justify-between gap-2">
                <span
                  className={`rounded-full px-2 py-1 text-[10px] font-semibold ${imageRoleTone(image)}`}
                >
                  {imageRoleLabel(image)}
                </span>
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
          {imageCounts.main === 0 ? (
            <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
              缺主图
            </div>
          ) : null}
          {imageBlocked && imageCounts.swatch === 0 ? (
            <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
              缺色块来源图
            </div>
          ) : null}
          {imageCounts.final === 0 ? (
            <div className="rounded-xl bg-amber-50 px-3 py-2 text-amber-800">
              缺最终提交图片
            </div>
          ) : null}
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-3">
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
              <input
                className="h-9 rounded-xl border border-zinc-200 px-3 text-sm outline-none focus:border-zinc-400"
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

      {submitErrorMessage ? (
        <FailureGuidance
          title={submitAction === "save_draft" ? "保存草稿失败" : "提交失败"}
          detail={submitErrorMessage}
          impact={
            submitAction === "save_draft"
              ? "本次不会把资料保存到 SHEIN 草稿箱，当前页面修改仍保留在工作台。"
              : "本次不会把资料提交到 SHEIN，请先处理阻断项或上传问题后再重试。"
          }
          nextStep={
            submitAction === "save_draft"
              ? "先检查图片上传、最终资料和阻断项，再重新保存到 SHEIN 草稿箱。"
              : "先修复图片、类目、属性或 SKU 阻断项，确认最终草稿后再重新提交。"
          }
        />
      ) : null}
      {saveMessage ? (
        <div className="rounded-2xl border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
          {saveMessage}
        </div>
      ) : null}
      {saveErrorMessage ? (
        <FailureGuidance
          title="最终草稿保存失败"
          detail={saveErrorMessage}
          impact="当前确认结果还没有写回最终草稿，本次提交仍会沿用上一次成功保存的版本。"
          nextStep="先检查网络或字段完整性，确认价格、图片和 SKU 后重新保存最终草稿。"
        />
      ) : null}
      {isPublishConfirming ? (
        <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
          <div className="space-y-2">
            <h3 className="text-base font-semibold text-zinc-950">确认发布到 SHEIN</h3>
            <p className="text-sm leading-6 text-zinc-600">
              这会把当前已确认资料正式提交到 SHEIN，请先核对类目、图片和 SKU。
            </p>
            <div className="grid gap-2 text-sm text-zinc-700 sm:grid-cols-3">
              <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
                类目：{finalReview?.category_id ?? "未确认"}
              </div>
              <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
                图片：{finalImages.length} 张
              </div>
              <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
                SKU：{finalReview?.skus?.length ?? 0} 个
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                tone="secondary"
                onClick={() => setIsPublishConfirming(false)}
                type="button"
              >
                取消
              </Button>
              <Button
                disabled={isSubmitting}
                onClick={() => {
                  setIsPublishConfirming(false);
                  onSubmit?.("publish");
                }}
                type="button"
              >
                确认发布
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      <div className="flex flex-wrap gap-2">
        <div className="basis-full rounded-2xl border border-zinc-200 bg-zinc-50 p-3 text-sm leading-6 text-zinc-700">
          <p className="font-semibold text-zinc-950">{submitHint}</p>
          <p className="mt-1">
            保存草稿：上传图片并保存到 SHEIN 草稿箱，不直接上架。正式发布：上传图片并提交 SHEIN 发布接口。
          </p>
        </div>
        <Button
          tone="secondary"
          disabled={isSaving}
          onClick={() =>
            onSaveFinalDraft?.({
              confirmed: true,
              submit_mode: "save_draft",
              manual_price_overrides: manualOverrides,
            })
          }
        >
          确认最终草稿
        </Button>
        <Button
          tone="secondary"
          disabled={!confirmed || !ready || isSubmitting}
          onClick={() => onSubmit?.("save_draft")}
        >
          {submitAction === "save_draft" ? "保存中..." : "保存到 SHEIN 草稿箱"}
        </Button>
        <Button
          disabled={!confirmed || !ready || isSubmitting}
          onClick={() => setIsPublishConfirming(true)}
        >
          {submitAction === "publish" ? "发布中..." : "发布到 SHEIN"}
        </Button>
      </div>
    </section>
  );
}
