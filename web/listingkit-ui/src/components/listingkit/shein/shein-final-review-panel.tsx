"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { SheinCustomerIssueSummary } from "@/components/listingkit/shein/shein-customer-issue-summary";
import {
  buildFinalReviewModel,
  initialPriceOverrides,
  manualPriceOverridesFromStrings,
  type ReviewSummaryItem,
} from "@/components/listingkit/shein/shein-final-review-helpers";
import {
  FailureGuidance,
  FinalReviewOverviewCards,
  ImageStructureDetails,
  ImageSubmitSummary,
  ReviewSummaryGrid,
  SkuPricingTable,
  StoreResolutionCard,
} from "@/components/listingkit/shein/shein-final-review-sections";
import {
  FinalReviewHeader,
  FinalReviewReadinessBanner,
  FinalReviewSubmitActions,
} from "@/components/listingkit/shein/shein-final-review-action-sections";
import {
  buildSheinCustomerIssues,
  type CustomerIssue,
} from "@/lib/shein-studio/shein-customer-issues";
import { getSheinSubmissionState } from "@/lib/listingkit/semantic-fields";
import {
  sheinPublishInFlight,
  sheinPublishSucceeded,
} from "@/lib/shein-studio/shein-submission-display";
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
  onSubmit?: (
    action: "publish" | "save_draft",
    payload?: {
      confirmed?: boolean;
      submit_mode?: "publish" | "save_draft";
      manual_price_overrides?: Record<string, number>;
    },
  ) => void;
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
  onSubmit,
}: Props) {
  const pricing = shein?.pricing;
  const initialOverrides = initialPriceOverrides(pricing);
  const [priceOverrides, setPriceOverrides] = useState<Record<string, string>>(
    () => initialOverrides,
  );
  const lastSyncedOverridesRef = useRef<Record<string, string>>(initialOverrides);
  const manualOverrides = useMemo(
    () => manualPriceOverridesFromStrings(priceOverrides),
    [priceOverrides],
  );
  const pricingSyncKey = useMemo(
    () =>
      JSON.stringify(
        (pricing?.sku_prices ?? []).map((sku) => ({
          supplier_sku: sku.supplier_sku ?? "",
          final_price: sku.final_price ?? null,
          updated_at: pricing?.updated_at ?? null,
        })),
      ),
    [pricing?.sku_prices, pricing?.updated_at],
  );

  useEffect(() => {
    const nextOverrides = initialPriceOverrides(pricing);
    setPriceOverrides((current) => {
      const lastSynced = lastSyncedOverridesRef.current;
      if (JSON.stringify(current) !== JSON.stringify(lastSynced)) {
        return current;
      }
      lastSyncedOverridesRef.current = nextOverrides;
      return nextOverrides;
    });
  }, [pricing, pricingSyncKey]);

  const sheinSubmission = getSheinSubmissionState(shein);
  const customerIssues = useMemo(() => buildSheinCustomerIssues(shein), [shein]);
  const model = useMemo(
    () =>
      buildFinalReviewModel({
        customerBlockingCount: customerIssues.filter(
          (issue) => issue.severity !== "warning",
        ).length,
        shein,
      }),
    [customerIssues, shein],
  );

  if (!model) {
    return null;
  }

  const {
    blockingCount,
    confirmed,
    finalImages,
    finalReview,
    imageBlocked,
    imageCounts,
    ready,
    submitHint,
    summaryItems,
  } = model;
  const publishInFlight = Boolean(
    isSubmitting && submitAction === "publish",
  ) || sheinPublishInFlight(sheinSubmission);
  const publishSucceeded = sheinPublishSucceeded(sheinSubmission);
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
      <FinalReviewHeader confirmed={confirmed} />

      <FinalReviewReadinessBanner
        blockingCount={blockingCount}
        confirmed={confirmed}
        ready={ready}
      />

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

      <StoreResolutionCard resolution={shein?.store_resolution} />

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
              : "先修复图片、类目、属性或 SKU 阻断项，确认当前结果后再重新提交。"
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
          title="保存当前修改失败"
          detail={saveErrorMessage}
          impact="当前价格、图片或 SKU 修改还没有写回任务结果，本次提交不会继续执行。"
          nextStep="先检查网络或字段完整性，确认价格、图片和 SKU 后重新提交。"
        />
      ) : null}
      <FinalReviewSubmitActions
        confirmed={confirmed}
        isSaving={isSaving}
        isPublished={publishSucceeded}
        isSubmitting={publishInFlight || isSubmitting}
        manualOverrides={manualOverrides}
        onSubmit={onSubmit}
        ready={ready}
        submitAction={submitAction}
        submitHint={submitHint}
      />
    </section>
  );
}
