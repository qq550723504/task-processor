"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/shared/button";
import { SheinCustomerIssueSummary } from "@/components/listingkit/shein/shein-customer-issue-summary";
import {
  hasBlockingKey,
  imageRoleCounts,
  type ReviewSummaryItem,
} from "@/components/listingkit/shein/shein-final-review-helpers";
import {
  FailureGuidance,
  FinalReviewOverviewCards,
  ImageStructureDetails,
  ImageSubmitSummary,
  ReviewSummaryGrid,
  SkuPricingTable,
} from "@/components/listingkit/shein/shein-final-review-sections";
import {
  buildSheinCustomerIssues,
  type CustomerIssue,
} from "@/lib/shein-studio/shein-customer-issues";
import type {
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

      <ReviewSummaryGrid
        items={summaryItems}
        onSelectItem={onSelectBlockingItem ? handleSelectSummaryItem : undefined}
      />

      <ImageSubmitSummary
        finalCount={imageCounts.final}
        galleryCount={imageCounts.gallery}
        imageBlocked={imageBlocked}
        mainCount={imageCounts.main}
        sizeMapCount={imageCounts.sizeMap}
        skcCount={imageCounts.skc}
        swatchCount={imageCounts.swatch}
        onSelectImages={
          imageBlocked && onSelectBlockingItem
            ? () =>
                handleSelectSummaryItem({
                  key: "images",
                  title: "图片资料",
                  message: "请检查最终提交图片角色。",
                  status: "blocked",
                })
            : undefined
        }
      />

      <ImageStructureDetails
        finalCount={imageCounts.final}
        finalImages={finalImages}
        imageBlocked={imageBlocked}
        mainCount={imageCounts.main}
        swatchCount={imageCounts.swatch}
      />

      <FinalReviewOverviewCards finalReview={finalReview} />

      <SkuPricingTable
        priceOverrides={priceOverrides}
        pricing={pricing}
        setPriceOverrides={setPriceOverrides}
      />

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
