import { Card } from "@/components/shared/card";
import { Button } from "@/components/shared/button";
import {
  matchingCandidates,
  presentSaleReviewStatus,
} from "@/components/listingkit/shein/shein-sale-attribute-review-card-model";
import {
  CandidateReasonList,
  SaleAttributeList,
  SectionHeading,
} from "@/components/listingkit/shein/shein-sale-attribute-review-card-sections";
import type {
  SheinEditorContext,
} from "@/lib/types/listingkit";

export function SheinSaleAttributeReviewCard({
  editorContext,
  isApplying,
  onConfirmCurrentSaleAttributes,
}: {
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onConfirmCurrentSaleAttributes?: (() => void) | null;
}) {
  const current = editorContext?.sale_attributes?.current;
  if (!current) {
    return null;
  }

  const skcAttributes = current.skc_attributes?.slice(0, 2) ?? [];
  const skuAttributes = current.sku_attributes?.slice(0, 3) ?? [];
  const candidates = current.candidates ?? [];
  const hasSignal =
    Boolean(current.status) ||
    Boolean(current.review_notes?.length) ||
    skcAttributes.length > 0 ||
    skuAttributes.length > 0 ||
    candidates.length > 0;

  if (!hasSignal) {
    return null;
  }

  const primaryAttributes = skcAttributes.filter(
    (attribute) => attribute.attribute_id === current.primary_attribute_id,
  );
  const secondaryAttributes = skuAttributes.filter(
    (attribute) => attribute.attribute_id === current.secondary_attribute_id,
  );
  const fallbackPrimaryAttributes =
    primaryAttributes.length > 0 ? primaryAttributes : skcAttributes.slice(0, 1);
  const fallbackSecondaryAttributes =
    secondaryAttributes.length > 0 ? secondaryAttributes : skuAttributes.slice(0, 1);
  const primaryCandidates = matchingCandidates(
    candidates,
    current.primary_attribute_id,
    "primary",
  );
  const secondaryCandidates = matchingCandidates(
    candidates,
    current.secondary_attribute_id,
    "secondary",
  );
  const unresolvedCandidates = candidates.filter(
    (candidate) =>
      candidate.selected_scope !== "primary" &&
      candidate.selected_scope !== "secondary",
  );
  const isPartial =
    current.status === "partial" ||
    current.status === "blocked" ||
    unresolvedCandidates.length > 0 ||
    current.recommend_category_review;
  const canConfirm =
    Boolean(onConfirmCurrentSaleAttributes) &&
    isPartial &&
    Boolean(current.primary_attribute_id) &&
    (skcAttributes.length > 0 || skuAttributes.length > 0);

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 销售属性确认
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              检查主规格、其他规格和 SDS 变体值是否完整映射到 SHEIN 销售属性。
            </p>
          </div>
          {canConfirm ? (
            <Button
              className="h-9 shrink-0 px-3 text-xs"
              disabled={isApplying}
              tone="secondary"
              onClick={() => onConfirmCurrentSaleAttributes?.()}
            >
              确认当前规格
            </Button>
          ) : null}
        </div>

        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
          {current.status ? (
            <span>状态 {presentSaleReviewStatus(current.status)}</span>
          ) : null}
          {current.primary_attribute_id ? (
            <span>主规格 {current.primary_attribute_id}</span>
          ) : null}
          {current.secondary_attribute_id ? (
            <span>其他规格 {current.secondary_attribute_id}</span>
          ) : null}
        </div>

        {fallbackPrimaryAttributes.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-sky-200 bg-sky-50/70 p-3">
            <SectionHeading
              description="SHEIN 的主规格通常对应 SKC 维度，例如颜色或款式。"
              title="主规格确认"
              tone="sky"
            />
            <SaleAttributeList attributes={fallbackPrimaryAttributes} scopeFallback="skc" />
            <CandidateReasonList candidates={primaryCandidates} emptyText="暂无主规格候选说明。" />
          </div>
        ) : null}

        {fallbackSecondaryAttributes.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-zinc-200 bg-zinc-50/80 p-3">
            <SectionHeading
              description="其他规格通常对应 SKU 维度，例如尺码、件数或其他可选规格。"
              title="其他规格确认"
            />
            <SaleAttributeList attributes={fallbackSecondaryAttributes} scopeFallback="sku" />
            <CandidateReasonList candidates={secondaryCandidates} emptyText="暂无其他规格候选说明。" />
          </div>
        ) : null}

        {candidates.length > 0 ? (
          <div
            className={`space-y-3 rounded-2xl border p-3 ${
              isPartial
                ? "border-amber-200 bg-amber-50/70"
                : "border-zinc-200 bg-zinc-50/80"
            }`}
            id="shein-sale-attribute-unresolved-group"
          >
            <SectionHeading
              description="这里展示 resolver 看到的候选维度和拟合原因，用来判断是否覆盖了 SDS 颜色、尺码或款式。"
              title="变体覆盖检查"
              tone={isPartial ? "amber" : "zinc"}
            />
            <CandidateReasonList candidates={candidates} />
          </div>
        ) : null}

        {skcAttributes.length > 0 || skuAttributes.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-3">
            <SectionHeading
              description="这些销售属性已经进入当前 SHEIN 资料包。"
              title="已映射销售属性"
              tone="emerald"
            />
            <SaleAttributeList attributes={skcAttributes} scopeFallback="skc" />
            <SaleAttributeList attributes={skuAttributes} scopeFallback="sku" />
          </div>
        ) : null}

        {current.selection_summary?.length ? (
          <div className="space-y-2">
            {current.selection_summary.map((line) => (
              <p className="text-sm leading-6 text-zinc-700" key={line}>
                {line}
              </p>
            ))}
          </div>
        ) : null}

        {current.review_notes?.length ? (
          <div className="space-y-2">
            {current.review_notes.map((note, index) => (
              <p className="text-sm leading-6 text-zinc-700" key={`${index}-${note}`}>
                {note}
              </p>
            ))}
          </div>
        ) : null}
      </div>
    </Card>
  );
}
