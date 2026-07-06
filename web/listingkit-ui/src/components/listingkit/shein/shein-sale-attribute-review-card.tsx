import { useEffect, useMemo, useRef, useState } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
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
  SheinInspectionSKCPatchPayload,
  SheinResolvedSaleAttribute,
  SheinSaleAttributeTemplateOption,
} from "@/lib/types/listingkit";

type ManualSaleAttributeSelection = {
  valueId?: number;
  textValue?: string;
};

export function SheinSaleAttributeReviewCard({
  applyErrorMessage,
  statusMessage,
  statusTone = "default",
  editorContext,
  isApplying,
  onConfirmCurrentSaleAttributes,
  onRegenerateSaleAttributes,
  onApplyManualSaleAttributes,
}: {
  applyErrorMessage?: string | null;
  statusMessage?: string | null;
  statusTone?: "default" | "success";
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onConfirmCurrentSaleAttributes?: (() => void) | null;
  onRegenerateSaleAttributes?: (() => void) | null;
  onApplyManualSaleAttributes?:
    | ((payload: {
        primaryOption?: SheinSaleAttributeTemplateOption | null;
        secondaryOption?: SheinSaleAttributeTemplateOption | null;
        skcSelections: Record<string, ManualSaleAttributeSelection>;
        skuSelections: Record<string, ManualSaleAttributeSelection>;
      }) => void)
    | null;
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

  return (
    <SheinSaleAttributeReviewContent
      applyErrorMessage={applyErrorMessage}
      statusMessage={statusMessage}
      statusTone={statusTone}
      current={current}
      isApplying={isApplying}
      onApplyManualSaleAttributes={onApplyManualSaleAttributes}
      onConfirmCurrentSaleAttributes={onConfirmCurrentSaleAttributes}
      onRegenerateSaleAttributes={onRegenerateSaleAttributes}
    />
  );
}

function SheinSaleAttributeReviewContent({
  applyErrorMessage,
  statusMessage,
  statusTone = "default",
  current,
  isApplying,
  onConfirmCurrentSaleAttributes,
  onRegenerateSaleAttributes,
  onApplyManualSaleAttributes,
}: {
  applyErrorMessage?: string | null;
  statusMessage?: string | null;
  statusTone?: "default" | "success";
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >;
  isApplying?: boolean;
  onConfirmCurrentSaleAttributes?: (() => void) | null;
  onRegenerateSaleAttributes?: (() => void) | null;
  onApplyManualSaleAttributes?:
    | ((payload: {
        primaryOption?: SheinSaleAttributeTemplateOption | null;
        secondaryOption?: SheinSaleAttributeTemplateOption | null;
        skcSelections: Record<string, ManualSaleAttributeSelection>;
        skuSelections: Record<string, ManualSaleAttributeSelection>;
      }) => void)
    | null;
}) {
  const manualTemplateOptions = useMemo(
    () =>
      sortSaleAttributeTemplateOptions(
        (current.template_options ?? []).filter(
          (option) => (option.attribute_value_list?.length ?? 0) > 0,
        ),
      ),
    [current.template_options],
  );
  const primaryTemplateOptions = useMemo(() => {
    const importantOptions = manualTemplateOptions.filter(
      (option) => option.important,
    );
    return importantOptions.length > 0
      ? importantOptions
      : manualTemplateOptions;
  }, [manualTemplateOptions]);
  const secondaryRequired = isSecondarySaleAttributeRequired(current);
  const hasMatchingSecondaryTemplate =
    hasMatchingSecondaryTemplateOption(current);
  const secondaryTemplateUnavailable =
    !secondaryRequired &&
    !hasMatchingSecondaryTemplate &&
    !current.secondary_attribute_id &&
    Boolean(current.secondary_source_dimension?.trim());
  const skcAttributes = current.skc_attributes?.slice(0, 2) ?? [];
  const skuAttributes = current.sku_attributes?.slice(0, 3) ?? [];
  const candidates = current.candidates ?? [];
  const hasMissingValueIDs =
    skcAttributes.some((attribute) => !attribute.attribute_value_id) ||
    skuAttributes.some((attribute) => !attribute.attribute_value_id);
  const initialPrimaryOptionID = String(
    pickTemplateOptionID({
      options: primaryTemplateOptions,
      candidates,
      currentAttributeID: current.primary_attribute_id,
      ignoreCurrentSelection: hasMissingValueIDs,
      scope: "primary",
      sourceDimension: current.primary_source_dimension,
    }) ?? "",
  );
  const initialSecondaryOptionID = String(
    pickTemplateOptionID({
      options: manualTemplateOptions.filter(
        (option) => option.attribute_id !== Number(initialPrimaryOptionID || 0),
      ),
      candidates,
      currentAttributeID: current.secondary_attribute_id,
      emptyFallback: true,
      preferEmptyWhenUnmatched: !secondaryRequired,
      ignoreCurrentSelection: hasMissingValueIDs,
      scope: "secondary",
      sourceDimension: current.secondary_source_dimension,
    }) ?? "",
  );
  const manualSaleAttributeFormKey = [
    current.primary_attribute_id ?? "",
    current.secondary_attribute_id ?? "",
    current.primary_source_dimension ?? "",
    current.secondary_source_dimension ?? "",
    primaryTemplateOptions.map((option) => option.attribute_id).join(","),
    candidates.map((candidate) => candidate.attribute_id).join(","),
    candidates.length,
    (current.skc_patches ?? []).length,
    hasMissingValueIDs ? "1" : "0",
    initialPrimaryOptionID,
    initialSecondaryOptionID,
  ].join("|");

  const primaryAttributes = skcAttributes.filter(
    (attribute) => attribute.attribute_id === current.primary_attribute_id,
  );
  const secondaryAttributes = skuAttributes.filter(
    (attribute) => attribute.attribute_id === current.secondary_attribute_id,
  );
  const fallbackPrimaryAttributes =
    primaryAttributes.length > 0
      ? primaryAttributes
      : skcAttributes.slice(0, 1);
  const fallbackSecondaryAttributes =
    secondaryAttributes.length > 0
      ? secondaryAttributes
      : skuAttributes.slice(0, 1);
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
  const needsTemplateRefresh =
    !current.primary_attribute_id ||
    manualTemplateOptions.length === 0 ||
    current.review_notes?.some(
      (note) =>
        note.includes("缺少 SHEIN AttributeAPI") ||
        note.includes("模板未就绪") ||
        note.includes("无法加载销售属性模板"),
    ) === true;
  const isPartial =
    current.status === "partial" ||
    current.status === "blocked" ||
    unresolvedCandidates.length > 0 ||
    hasMissingValueIDs ||
    current.recommend_category_review;
  const canConfirm =
    Boolean(onConfirmCurrentSaleAttributes) &&
    !hasMissingValueIDs &&
    isPartial &&
    Boolean(current.primary_attribute_id) &&
    (skcAttributes.length > 0 || skuAttributes.length > 0);
  const canRegenerate =
    Boolean(onRegenerateSaleAttributes) &&
    isPartial &&
    (hasMissingValueIDs || needsTemplateRefresh);
  const canManualEdit =
    Boolean(onApplyManualSaleAttributes) &&
    (current.skc_patches?.length ?? 0) > 0 &&
    manualTemplateOptions.length > 0;
  const statusLabel = canRegenerate
    ? "建议重新生成"
    : secondaryRequired
      ? "需要补其他规格"
      : !secondaryRequired && canConfirm
        ? "可直接确认"
        : current.status
          ? presentSaleReviewStatus(current.status)
          : undefined;
  const statusDescription = canRegenerate
    ? "当前结果还不能直接使用，建议先重新生成属性后再继续确认。"
    : secondaryRequired
      ? "当前商品需要补齐其他规格后，才能继续确认销售属性。"
      : secondaryTemplateUnavailable
        ? "主规格结果已可用，当前类目下其他规格可以跳过。"
        : canConfirm
          ? "当前主规格和其他规格结果已经可以直接确认。"
          : "请先检查当前识别结果。";
  const actionDescription = canRegenerate
    ? "先重新生成可用模板或 value_id；如果结果仍不准确，再展开手工修正规格。"
    : canConfirm && !secondaryRequired
      ? "确认无误后，直接提交当前结果即可。"
      : secondaryRequired
        ? "先进入手工修正规格，补齐其他规格字段和值。"
        : canManualEdit
          ? "如果系统结果不准确，再展开手工修正规格逐项修改。"
          : "当前结果还需要进一步检查。";
  const customValueDeniedNote = findCustomValueDeniedNote(current.review_notes);
  const customValueDeniedAttributeName =
    fallbackSecondaryAttributes[0]?.name ??
    fallbackPrimaryAttributes[0]?.name ??
    undefined;
  const hasProcessingNotes =
    Boolean(current.selection_summary?.length) ||
    Boolean(current.review_notes?.length) ||
    skcAttributes.length > 0 ||
    skuAttributes.length > 0;

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        {statusMessage ? (
          <Alert variant={statusTone === "success" ? "success" : "default"}>
            <AlertDescription>{statusMessage}</AlertDescription>
          </Alert>
        ) : null}
        {applyErrorMessage ? (
          <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm leading-6 text-rose-700">
            保存销售属性失败：{applyErrorMessage}
          </div>
        ) : null}
        <div className="flex flex-col gap-3 2xl:grid 2xl:grid-cols-[minmax(0,1fr),auto] 2xl:items-start">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 销售属性确认
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              {statusDescription}
            </p>
          </div>
        </div>

        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
          {statusLabel ? <span>状态 {statusLabel}</span> : null}
          {current.primary_attribute_id ? (
            <span>主规格 {current.primary_attribute_id}</span>
          ) : null}
          {current.secondary_attribute_id ? (
            <span>其他规格 {current.secondary_attribute_id}</span>
          ) : null}
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <ResultSummaryCard
            description="系统当前识别到的主规格"
            mapped={formatResolvedAttributeMap(fallbackPrimaryAttributes[0])}
            title="主规格"
            value={formatResolvedAttributeValue(fallbackPrimaryAttributes[0])}
          />
          <ResultSummaryCard
            description={
              secondaryTemplateUnavailable
                ? "当前类目存在其他规格字段，但没有可用于当前来源维度的模板，这一步可以跳过。"
                : "系统当前识别到的其他规格"
            }
            mapped={
              secondaryTemplateUnavailable
                ? undefined
                : formatResolvedAttributeMap(fallbackSecondaryAttributes[0])
            }
            title="其他规格"
            value={
              secondaryTemplateUnavailable
                ? "当前来源维度暂无可用模板"
                : formatResolvedAttributeValue(
                    fallbackSecondaryAttributes[0],
                    "未识别或未使用",
              )
            }
          />
        </div>

        {canRegenerate || (canConfirm && !secondaryRequired) ? (
          <div className="rounded-2xl border border-sky-200 bg-sky-50/70 p-4">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-sky-700">
              当前操作
            </p>
            <p className="mt-2 text-sm leading-6 text-sky-950">
              {actionDescription}
            </p>
            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:flex-wrap">
              <Button
                className="h-9 w-full px-3 text-xs sm:w-auto"
                disabled={isApplying}
                onClick={
                  canRegenerate
                    ? () => onRegenerateSaleAttributes?.()
                    : () => onConfirmCurrentSaleAttributes?.()
                }
              >
                {canRegenerate
                  ? isApplying
                    ? "重新生成中..."
                    : "重新生成属性"
                  : isApplying
                    ? "保存中..."
                    : "直接确认当前结果"}
              </Button>
            </div>
          </div>
        ) : null}

        {customValueDeniedNote ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50/70 p-3">
            <p className="text-sm font-medium text-amber-950">
              当前类目不支持该销售属性自定义值
            </p>
            <p className="mt-1 text-sm leading-6 text-amber-900">
              当前类目的
              {customValueDeniedAttributeName
                ? `「${customValueDeniedAttributeName}」`
                : "该销售属性"}
              不支持自定义值。请优先改选模板已有值；如果模板值仍然不覆盖当前商品规格，建议切换类目后再重试。
            </p>
            <p className="mt-1 text-xs leading-5 text-amber-700">
              {customValueDeniedNote}
            </p>
          </div>
        ) : null}

        {hasMissingValueIDs ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50/70 p-3">
            <p className="text-sm leading-6 text-amber-900">
              当前销售属性只有 `attribute_id`，还缺少真实
              `value_id`，不能直接确认。
              你可以先重新生成属性；如果结果仍不准确，下面也可以手工修正规格。
            </p>
          </div>
        ) : null}

        {canRegenerate && !hasMissingValueIDs && needsTemplateRefresh ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50/70 p-3">
            <p className="text-sm leading-6 text-amber-900">
              当前还没有拿到可用的销售属性模板或主规格识别结果，暂时不能直接确认。
              请先点“重新生成属性”在线重试；拿到模板后，这里才会出现可确认或可手工修正的规格控件。
            </p>
          </div>
        ) : null}

        {canManualEdit ? (
          <SheinManualSaleAttributeForm
            key={manualSaleAttributeFormKey}
            canApplyManual={Boolean(onApplyManualSaleAttributes)}
            current={current}
            currentHasMissingValueIDs={hasMissingValueIDs}
            isApplying={Boolean(isApplying)}
            onApplyManualSaleAttributes={onApplyManualSaleAttributes}
            primaryOptionCandidates={primaryTemplateOptions}
            secondaryOptionCandidates={manualTemplateOptions}
            secondaryRequired={secondaryRequired}
            secondaryTemplateUnavailable={secondaryTemplateUnavailable}
            selectedSourceDimensions={{
              primarySourceDimension: current.primary_source_dimension,
              secondarySourceDimension: current.secondary_source_dimension,
            }}
            initialPrimaryOptionID={initialPrimaryOptionID}
            initialSecondaryOptionID={initialSecondaryOptionID}
          />
        ) : null}

        {candidates.length > 0 ? (
          <details
            className={`rounded-2xl border p-3 ${
              isPartial
                ? "border-amber-200 bg-amber-50/70"
                : "border-zinc-200 bg-zinc-50/80"
            }`}
            id="shein-sale-attribute-unresolved-group"
          >
            <summary className="cursor-pointer list-none">
              <SectionHeading
                description="需要排查时，再展开查看系统为什么会这样匹配。普通使用时可以忽略这里。"
                title="查看匹配原因"
                tone={isPartial ? "amber" : "zinc"}
              />
            </summary>
            <div className="mt-3 space-y-4">
              {fallbackPrimaryAttributes.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
                    主规格
                  </p>
                  <SaleAttributeList
                    attributes={fallbackPrimaryAttributes}
                    scopeFallback="skc"
                  />
                  <CandidateReasonList
                    candidates={primaryCandidates}
                    emptyText="暂无主规格候选说明。"
                  />
                </div>
              ) : null}
              {fallbackSecondaryAttributes.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-600">
                    其他规格
                  </p>
                  <SaleAttributeList
                    attributes={fallbackSecondaryAttributes}
                    scopeFallback="sku"
                  />
                  <CandidateReasonList
                    candidates={secondaryCandidates}
                    emptyText="暂无其他规格候选说明。"
                  />
                </div>
              ) : null}
            </div>
          </details>
        ) : null}

        {hasProcessingNotes ? (
          <details className="rounded-2xl border border-zinc-200 bg-zinc-50/80 p-3">
            <summary className="cursor-pointer list-none text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              查看处理说明
            </summary>
            <div className="mt-3 space-y-3">
              {skcAttributes.length > 0 || skuAttributes.length > 0 ? (
                <div className="space-y-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-3">
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
                    资料包中的规格
                  </p>
                  <SaleAttributeList
                    attributes={skcAttributes}
                    scopeFallback="skc"
                  />
                  <SaleAttributeList
                    attributes={skuAttributes}
                    scopeFallback="sku"
                  />
                </div>
              ) : null}
              {current.selection_summary?.map((line) => (
                <p className="text-sm leading-6 text-zinc-700" key={line}>
                  {line}
                </p>
              ))}
              {current.review_notes?.map((note, index) => (
                <p
                  className="text-sm leading-6 text-zinc-700"
                  key={`${index}-${note}`}
                >
                  {note}
                </p>
              ))}
            </div>
          </details>
        ) : null}
      </div>
    </Card>
  );
}

function SheinManualSaleAttributeForm({
  canApplyManual,
  current,
  currentHasMissingValueIDs,
  isApplying,
  onApplyManualSaleAttributes,
  primaryOptionCandidates,
  secondaryOptionCandidates,
  secondaryRequired,
  secondaryTemplateUnavailable,
  selectedSourceDimensions,
  initialPrimaryOptionID,
  initialSecondaryOptionID,
}: {
  canApplyManual: boolean;
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >;
  currentHasMissingValueIDs: boolean;
  isApplying: boolean;
  onApplyManualSaleAttributes?:
    | (((payload: {
        primaryOption?: SheinSaleAttributeTemplateOption | null;
        secondaryOption?: SheinSaleAttributeTemplateOption | null;
        skcSelections: Record<string, ManualSaleAttributeSelection>;
        skuSelections: Record<string, ManualSaleAttributeSelection>;
      }) => void) | null);
  primaryOptionCandidates: SheinSaleAttributeTemplateOption[];
  secondaryOptionCandidates: SheinSaleAttributeTemplateOption[];
  secondaryRequired: boolean;
  secondaryTemplateUnavailable: boolean;
  selectedSourceDimensions: {
    primarySourceDimension?: string;
    secondarySourceDimension?: string;
  };
  initialPrimaryOptionID: string;
  initialSecondaryOptionID: string;
}) {
  const candidates = current.candidates ?? [];
  const [primaryOptionID, setPrimaryOptionID] = useState(
    initialPrimaryOptionID,
  );
  const [secondaryOptionID, setSecondaryOptionID] = useState(
    initialSecondaryOptionID,
  );
  const [hasExplicitEmptySecondarySelection, setHasExplicitEmptySecondarySelection] =
    useState(false);
  const [manualDetailsOpen, setManualDetailsOpen] = useState(
    currentHasMissingValueIDs || secondaryRequired,
  );
  const primaryOption = primaryOptionCandidates.find(
    (option) => String(option.attribute_id ?? "") === primaryOptionID,
  ) ?? null;
  const secondaryTemplateOptions = useMemo(
    () =>
      secondaryOptionCandidates.filter(
        (option) => option.attribute_id !== primaryOption?.attribute_id,
      ),
    [secondaryOptionCandidates, primaryOption?.attribute_id],
  );
  const selectedSecondaryOptionID =
    secondaryOptionID &&
    secondaryTemplateOptions.some(
      (option) => String(option.attribute_id ?? "") === secondaryOptionID,
    )
      ? secondaryOptionID
      : hasExplicitEmptySecondarySelection && !secondaryRequired
        ? ""
      : String(
          pickTemplateOptionID({
            options: secondaryTemplateOptions,
            candidates,
            currentAttributeID: current.secondary_attribute_id,
            emptyFallback: true,
            preferEmptyWhenUnmatched: !secondaryRequired,
            ignoreCurrentSelection: currentHasMissingValueIDs,
            scope: "secondary",
            sourceDimension: current.secondary_source_dimension,
          }) ?? "",
        );
  const secondaryOption =
    secondaryTemplateOptions.find(
      (option) =>
        String(option.attribute_id ?? "") === selectedSecondaryOptionID,
    ) ?? null;
  const [skcSelections, setSKCSelections] = useState<
    Record<string, ManualSaleAttributeSelection>
  >(() =>
    buildInitialManualSKCSelections({
      current,
      primaryOption,
      primarySourceDimension: selectedSourceDimensions.primarySourceDimension,
    }),
  );
  const [skuSelections, setSKUSelections] = useState<
    Record<string, ManualSaleAttributeSelection>
  >(() =>
    buildInitialManualSKUSelections({
      current,
      secondaryOption,
      secondarySourceDimension:
        selectedSourceDimensions.secondarySourceDimension,
    }),
  );

  const allSKCSelected =
    (current.skc_patches ?? []).length > 0 &&
    (current.skc_patches ?? []).every(
      (patch) =>
        patch.supplier_code &&
        hasManualSelection(skcSelections[patch.supplier_code]),
    );
  const allSKUSelected =
    (!secondaryRequired && !secondaryOption) ||
    (current.skc_patches ?? [])
      .flatMap((patch) => patch.sku_patches ?? [])
      .every(
        (patch) =>
          patch.supplier_sku &&
          hasManualSelection(skuSelections[patch.supplier_sku]),
      );
  const canSaveManual =
    canApplyManual &&
    Boolean(primaryOption?.attribute_id) &&
    (!secondaryRequired || Boolean(secondaryOption?.attribute_id)) &&
    allSKCSelected &&
    allSKUSelected;

  const previousPrimaryAttributeIDRef = useRef<number | null | undefined>(
    primaryOption?.attribute_id,
  );
  const previousSecondaryAttributeIDRef = useRef<number | null | undefined>(
    secondaryOption?.attribute_id,
  );

  useEffect(() => {
    if (previousPrimaryAttributeIDRef.current !== primaryOption?.attribute_id) {
      previousPrimaryAttributeIDRef.current = primaryOption?.attribute_id;
      setSKCSelections(
        buildInitialManualSKCSelections({
          current,
          primaryOption,
          primarySourceDimension:
            selectedSourceDimensions.primarySourceDimension,
        }),
      );
    }
  }, [
    current,
    primaryOption,
    primaryOption?.attribute_id,
    selectedSourceDimensions.primarySourceDimension,
  ]);

  useEffect(() => {
    if (
      previousSecondaryAttributeIDRef.current !== secondaryOption?.attribute_id
    ) {
      previousSecondaryAttributeIDRef.current = secondaryOption?.attribute_id;
      setSKUSelections(
        buildInitialManualSKUSelections({
          current,
          secondaryOption,
          secondarySourceDimension:
            selectedSourceDimensions.secondarySourceDimension,
        }),
      );
    }
  }, [
    current,
    secondaryOption,
    secondaryOption?.attribute_id,
    selectedSourceDimensions.secondarySourceDimension,
  ]);

  return (
    <details
      className="rounded-2xl border border-amber-200 bg-amber-50/70 p-3"
      open={manualDetailsOpen}
      onToggle={(event) => setManualDetailsOpen(event.currentTarget.open)}
    >
      <summary className="cursor-pointer list-none">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <SectionHeading
            description="只有系统结果不准确时，才需要展开这里手工修正。保存后会写入 SKC/SKU 规格，并优先用文本值向 SHEIN 换取真实 value_id。"
            title="手工修正规格"
            tone="amber"
          />
          <span className="inline-flex h-8 shrink-0 items-center justify-center rounded-full border border-amber-300 bg-white px-3 text-xs font-medium text-amber-800">
            {manualDetailsOpen ? "收起" : "展开修正"}
          </span>
        </div>
      </summary>
      <div className="mt-3 space-y-3">
        <div className="grid gap-3 2xl:grid-cols-2">
          <TemplateOptionSelect
            id="shein-sale-attribute-primary-template"
            label={`第 1 步：主规格字段${selectedSourceDimensions.primarySourceDimension ? ` · 来源 ${selectedSourceDimensions.primarySourceDimension}` : ""}`}
            onChange={setPrimaryOptionID}
            options={primaryOptionCandidates}
            value={primaryOptionID}
          />
          {secondaryTemplateUnavailable ? (
            <OptionalSecondaryTemplateNotice
              label={`第 2 步：其他规格字段（选填）${selectedSourceDimensions.secondarySourceDimension ? ` · 来源 ${selectedSourceDimensions.secondarySourceDimension}` : ""}`}
            />
          ) : (
            <TemplateOptionSelect
              allowEmpty={!secondaryRequired}
              id="shein-sale-attribute-secondary-template"
              label={`第 2 步：其他规格字段（${secondaryRequired ? "必填" : "选填"}）${selectedSourceDimensions.secondarySourceDimension ? ` · 来源 ${selectedSourceDimensions.secondarySourceDimension}` : ""}`}
              onChange={(value) => {
                setHasExplicitEmptySecondarySelection(
                  !secondaryRequired && value === "",
                );
                setSecondaryOptionID(value);
              }}
              options={secondaryTemplateOptions}
              placeholder={
                secondaryRequired
                  ? "请选择其他规格字段"
                  : "不填写其他规格"
              }
              value={selectedSecondaryOptionID}
            />
          )}
        </div>
        <div className="space-y-3">
          {(current.skc_patches ?? []).map((patch) => (
            <ManualSKCMappingRow
              key={patch.supplier_code ?? patch.skc_name}
              patch={patch}
              primaryOption={primaryOption}
              secondaryOption={secondaryOption}
              primarySourceDimension={selectedSourceDimensions.primarySourceDimension}
              secondarySourceDimension={
                selectedSourceDimensions.secondarySourceDimension
              }
              skcSelection={
                patch.supplier_code
                  ? skcSelections[patch.supplier_code]
                  : undefined
              }
              skuSelections={skuSelections}
              onSKCChange={(selection) => {
                if (!patch.supplier_code) {
                  return;
                }
                setSKCSelections((state) => ({
                  ...state,
                  [patch.supplier_code!]: selection,
                }));
              }}
              onSKUChange={(supplierSKU, selection) =>
                setSKUSelections((state) => ({
                  ...state,
                  [supplierSKU]: selection,
                }))
              }
            />
          ))}
        </div>
        {canSaveManual ? (
          <div className="flex justify-end">
            <Button
              className="h-9 w-full px-3 text-xs sm:w-auto"
              disabled={isApplying}
              onClick={() =>
                onApplyManualSaleAttributes?.({
                  primaryOption,
                  secondaryOption,
                  skcSelections,
                  skuSelections: secondaryOption ? skuSelections : {},
                })
              }
            >
              {isApplying ? "保存中..." : "保存手工修正"}
            </Button>
          </div>
        ) : null}
      </div>
    </details>
  );
}

function TemplateOptionSelect({
  id,
  label,
  value,
  onChange,
  options,
  allowEmpty,
  placeholder = "选择模板属性",
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: SheinSaleAttributeTemplateOption[];
  allowEmpty?: boolean;
  placeholder?: string;
}) {
  return (
    <Label
      className="block rounded-xl border border-zinc-200 bg-white px-3 py-2"
      htmlFor={id}
    >
      <span className="block text-sm font-medium text-zinc-950">{label}</span>
      <Select
        className="mt-2 rounded-xl"
        id={id}
        name={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{allowEmpty ? "不使用" : placeholder}</option>
        {options.map((option) => (
          <option
            key={option.attribute_id}
            value={String(option.attribute_id ?? "")}
          >
            {formatTemplateOptionLabel(option)}
          </option>
        ))}
      </Select>
    </Label>
  );
}

function OptionalSecondaryTemplateNotice({ label }: { label: string }) {
  return (
    <div className="rounded-xl border border-dashed border-zinc-300 bg-zinc-50/70 px-3 py-2">
      <p className="text-sm font-medium text-zinc-950">{label}</p>
      <p className="mt-2 text-sm leading-6 text-zinc-700">
        当前类目存在其他规格字段，但没有可用于当前来源维度的模板，可保持只使用主规格。
      </p>
      <p className="mt-1 text-xs leading-5 text-zinc-500">
        如果只是尺寸这类来源维度没有映射模板，这一步不用强行选择；只有主规格结果不准确时，再继续手工修正。
      </p>
    </div>
  );
}

function ManualSKCMappingRow({
  patch,
  primaryOption,
  secondaryOption,
  primarySourceDimension,
  secondarySourceDimension,
  skcSelection,
  skuSelections,
  onSKCChange,
  onSKUChange,
}: {
  patch: SheinInspectionSKCPatchPayload;
  primaryOption: SheinSaleAttributeTemplateOption | null;
  secondaryOption: SheinSaleAttributeTemplateOption | null;
  primarySourceDimension?: string;
  secondarySourceDimension?: string;
  skcSelection?: ManualSaleAttributeSelection;
  skuSelections: Record<string, ManualSaleAttributeSelection>;
  onSKCChange: (selection: ManualSaleAttributeSelection) => void;
  onSKUChange: (
    supplierSKU: string,
    selection: ManualSaleAttributeSelection,
  ) => void;
}) {
  return (
    <div className="space-y-3 rounded-xl border border-zinc-200 bg-white/80 p-3">
      <div className="grid gap-3 2xl:grid-cols-[minmax(0,1.2fr),minmax(280px,1fr)]">
        <div>
          <p className="text-sm font-medium text-zinc-900">
            {patch.skc_name ?? patch.sale_name ?? patch.supplier_code}
          </p>
          <p className="mt-1 text-xs text-zinc-600">
            {primarySourceDimension || "主来源维度"}：
            {resolveSourceValue(patch.attributes, primarySourceDimension) ||
              "未识别"}
          </p>
        </div>
        <ValueOptionSelect
          id={`shein-sale-attribute-skc-${patch.supplier_code ?? patch.skc_name ?? "unknown"}`}
          label="第 3 步：主规格值"
          onChange={onSKCChange}
          options={primaryOption?.attribute_value_list ?? []}
          sourceValue={resolveSourceValue(
            patch.attributes,
            primarySourceDimension,
          )}
          value={skcSelection}
        />
      </div>
      {secondaryOption && (patch.sku_patches?.length ?? 0) > 0 ? (
        <div className="grid gap-2 2xl:grid-cols-2">
          {(patch.sku_patches ?? []).map((skuPatch) => (
            <div
              className="rounded-xl border border-zinc-200/80 bg-zinc-50/70 p-3"
              key={skuPatch.supplier_sku ?? ""}
            >
              <p className="text-sm font-medium text-zinc-900">
                {skuPatch.supplier_sku}
              </p>
              <p className="mt-1 text-xs text-zinc-600">
                {secondarySourceDimension || "次来源维度"}：
                {resolveSourceValue(
                  skuPatch.attributes,
                  secondarySourceDimension,
                ) || "未识别"}
              </p>
              <div className="mt-2">
                <ValueOptionSelect
                  id={`shein-sale-attribute-sku-${skuPatch.supplier_sku ?? "unknown"}`}
                  label="第 3 步：其他规格值"
                  onChange={(selection) => {
                    if (skuPatch.supplier_sku) {
                      onSKUChange(skuPatch.supplier_sku, selection);
                    }
                  }}
                  options={secondaryOption.attribute_value_list ?? []}
                  sourceValue={resolveSourceValue(
                    skuPatch.attributes,
                    secondarySourceDimension,
                  )}
                  value={
                    skuPatch.supplier_sku
                      ? skuSelections[skuPatch.supplier_sku]
                      : undefined
                  }
                />
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function ValueOptionSelect({
  id,
  label,
  value,
  onChange,
  options,
  sourceValue,
}: {
  id: string;
  label: string;
  value?: ManualSaleAttributeSelection;
  onChange: (value: ManualSaleAttributeSelection) => void;
  options: NonNullable<
    SheinSaleAttributeTemplateOption["attribute_value_list"]
  >;
  sourceValue?: string;
}) {
  const selectValue = value?.textValue?.trim()
    ? ""
    : String(value?.valueId ?? "");
  return (
    <Label
      className="block rounded-xl border border-zinc-200 bg-white px-3 py-2"
      htmlFor={id}
    >
      <span className="block text-sm font-medium text-zinc-950">{label}</span>
      <Select
        className="mt-2 rounded-xl"
        id={id}
        name={id}
        value={selectValue}
        onChange={(event) =>
          onChange({
            valueId: event.target.value
              ? Number(event.target.value)
              : undefined,
            textValue: "",
          })
        }
      >
        <option value="">选择模板值或改为手工输入</option>
        {options.map((option) => (
          <option
            key={option.attribute_value_id}
            value={String(option.attribute_value_id ?? "")}
          >
            {option.value_en ?? option.value ?? option.attribute_value_id}
          </option>
        ))}
      </Select>
      <div className="mt-2 space-y-1">
        <Input
          id={`${id}-text`}
          name={`${id}-text`}
          placeholder={
            sourceValue
              ? `手工输入，建议值：${sourceValue}`
              : "手工输入销售属性值"
          }
          value={value?.textValue ?? ""}
          onChange={(event) =>
            onChange({
              valueId: value?.valueId,
              textValue: event.target.value,
            })
          }
        />
        <p className="text-xs text-zinc-500">
          可直接选择模板值；如果没有合适值，就手工输入，系统会优先向 SHEIN
          换取真实值 ID。
        </p>
      </div>
    </Label>
  );
}

function ResultSummaryCard({
  title,
  value,
  description,
  mapped,
}: {
  title: string;
  value: string;
  description: string;
  mapped?: string;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/90 p-3">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <p className="mt-2 text-sm font-medium text-zinc-950">{value}</p>
      <p className="mt-1 text-xs leading-5 text-zinc-600">{description}</p>
      {mapped ? (
        <p className="mt-2 text-[11px] uppercase tracking-[0.16em] text-zinc-500">
          {mapped}
        </p>
      ) : null}
    </div>
  );
}

function resolveSourceValue(
  attributes?: Record<string, string>,
  sourceDimension?: string,
) {
  if (!attributes || !sourceDimension) {
    return "";
  }
  return attributes[sourceDimension] ?? "";
}

function buildInitialManualSKCSelections({
  current,
  primaryOption,
  primarySourceDimension,
}: {
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >;
  primaryOption: SheinSaleAttributeTemplateOption | null;
  primarySourceDimension?: string;
}) {
  const selections: Record<string, ManualSaleAttributeSelection> = {};
  for (const patch of current.skc_patches ?? []) {
    if (!patch.supplier_code) {
      continue;
    }
    const sourceValue = resolveSourceValue(
      patch.attributes,
      primarySourceDimension,
    );
    const fallbackValueID = findResolvedSaleAttributeValueID({
      attributes: current.skc_attributes,
      attributeID: primaryOption?.attribute_id,
      sourceValue,
    });
    selections[patch.supplier_code] = buildInitialManualSelection({
      option: primaryOption,
      sourceValue,
      fallbackValueID,
    });
  }
  return selections;
}

function buildInitialManualSKUSelections({
  current,
  secondaryOption,
  secondarySourceDimension,
}: {
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >;
  secondaryOption: SheinSaleAttributeTemplateOption | null;
  secondarySourceDimension?: string;
}) {
  const selections: Record<string, ManualSaleAttributeSelection> = {};
  for (const skcPatch of current.skc_patches ?? []) {
    for (const skuPatch of skcPatch.sku_patches ?? []) {
      if (!skuPatch.supplier_sku) {
        continue;
      }
      const sourceValue = resolveSourceValue(
        skuPatch.attributes,
        secondarySourceDimension,
      );
      const fallbackValueID = findResolvedSaleAttributeValueID({
        attributes: current.sku_attributes,
        attributeID: secondaryOption?.attribute_id,
        sourceValue,
      });
      selections[skuPatch.supplier_sku] = buildInitialManualSelection({
        option: secondaryOption,
        sourceValue,
        fallbackValueID,
      });
    }
  }
  return selections;
}

function buildInitialManualSelection({
  option,
  sourceValue,
  fallbackValueID,
}: {
  option: SheinSaleAttributeTemplateOption | null;
  sourceValue?: string;
  fallbackValueID?: number;
}): ManualSaleAttributeSelection {
  const matchedValueID = findTemplateValueID(option, sourceValue, fallbackValueID);
  if (matchedValueID) {
    return { valueId: matchedValueID, textValue: "" };
  }
  return sourceValue?.trim() ? { textValue: sourceValue.trim() } : {};
}

function findResolvedSaleAttributeValueID({
  attributes,
  attributeID,
  sourceValue,
}: {
  attributes?: SheinResolvedSaleAttribute[];
  attributeID?: number;
  sourceValue?: string;
}) {
  const normalizedSource = normalizeSaleAttributeToken(sourceValue);
  const match = (attributes ?? []).find((attribute) => {
    if (attributeID && attribute.attribute_id !== attributeID) {
      return false;
    }
    if (!normalizedSource) {
      return Boolean(attribute.attribute_value_id);
    }
    return (
      normalizeSaleAttributeToken(attribute.value) === normalizedSource &&
      Boolean(attribute.attribute_value_id)
    );
  });
  return match?.attribute_value_id;
}

function findTemplateValueID(
  option: SheinSaleAttributeTemplateOption | null,
  sourceValue?: string,
  fallbackValueID?: number,
) {
  const values = option?.attribute_value_list ?? [];
  if (fallbackValueID && values.some((value) => value.attribute_value_id === fallbackValueID)) {
    return fallbackValueID;
  }
  const normalizedSource = normalizeSaleAttributeToken(sourceValue);
  if (!normalizedSource) {
    return undefined;
  }
  return values.find(
    (value) =>
      normalizeSaleAttributeToken(value.value_en) === normalizedSource ||
      normalizeSaleAttributeToken(value.value) === normalizedSource,
  )?.attribute_value_id;
}

function pickTemplateOptionID({
  options,
  candidates,
  currentAttributeID,
  emptyFallback,
  preferEmptyWhenUnmatched,
  ignoreCurrentSelection,
  scope,
  sourceDimension,
}: {
  options: SheinSaleAttributeTemplateOption[];
  candidates: NonNullable<
    NonNullable<
      NonNullable<SheinEditorContext["sale_attributes"]>["current"]
    >["candidates"]
  >;
  currentAttributeID?: number;
  emptyFallback?: boolean;
  preferEmptyWhenUnmatched?: boolean;
  ignoreCurrentSelection?: boolean;
  scope: "primary" | "secondary";
  sourceDimension?: string;
}) {
  if (options.length === 0) {
    return emptyFallback ? "" : undefined;
  }

  const byCurrent = currentAttributeID
    ? options.find((option) => option.attribute_id === currentAttributeID)
    : undefined;
  const bySourceCandidate = sourceDimension
    ? candidates.find(
        (candidate) =>
          candidate.attribute_id &&
          candidate.selected_scope === scope &&
          normalizeSaleAttributeToken(candidate.source_dimension) ===
            normalizeSaleAttributeToken(sourceDimension) &&
          options.some(
            (option) => option.attribute_id === candidate.attribute_id,
          ),
      )
    : undefined;
  const byScopedCandidate = candidates.find(
    (candidate) =>
      candidate.attribute_id &&
      candidate.selected_scope === scope &&
      options.some((option) => option.attribute_id === candidate.attribute_id),
  );
  const bySourceName = sourceDimension
    ? options.find(
        (option) =>
          normalizeSaleAttributeToken(option.name_en ?? option.name) ===
          normalizeSaleAttributeToken(sourceDimension),
      )
    : undefined;
  const byPrimaryLabel =
    scope === "primary"
      ? options.find((option) => option.important)
      : options.find((option) => !option.important);
  const byScopeFallback =
    scope === "primary"
      ? options.find((option) => option.skc_scope)
      : options.find((option) => !option.skc_scope);

  if (
    preferEmptyWhenUnmatched &&
    !byCurrent &&
    !bySourceCandidate &&
    !byScopedCandidate &&
    !bySourceName
  ) {
    return "";
  }

  const ordered = ignoreCurrentSelection
    ? [
        byPrimaryLabel,
        bySourceCandidate,
        byScopedCandidate,
        bySourceName,
        byScopeFallback,
        byCurrent,
      ]
    : [
        byCurrent,
        byPrimaryLabel,
        bySourceCandidate,
        byScopedCandidate,
        bySourceName,
        byScopeFallback,
      ];
  const match = ordered.find(
    (option): option is SheinSaleAttributeTemplateOption => Boolean(option),
  );
  if (!match && preferEmptyWhenUnmatched) {
    return "";
  }
  return match?.attribute_id ?? (emptyFallback ? "" : options[0]?.attribute_id);
}

function sortSaleAttributeTemplateOptions(
  options: SheinSaleAttributeTemplateOption[],
) {
  return [...options].sort((left, right) => {
    if (Boolean(left.important) !== Boolean(right.important)) {
      return left.important ? -1 : 1;
    }
    if (Boolean(left.skc_scope) !== Boolean(right.skc_scope)) {
      return left.skc_scope ? -1 : 1;
    }
    return String(
      left.name_en ?? left.name ?? left.attribute_id ?? "",
    ).localeCompare(
      String(right.name_en ?? right.name ?? right.attribute_id ?? ""),
      undefined,
      { sensitivity: "base" },
    );
  });
}

function isSecondarySaleAttributeRequired(
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >,
) {
  const skcPatches = current.skc_patches ?? [];
  const hasMultiSKUWithinSingleSKC = skcPatches.some(
    (patch) => (patch.sku_patches?.length ?? 0) > 1,
  );
  if (!hasMultiSKUWithinSingleSKC) {
    return false;
  }
  const secondarySourceDimension = current.secondary_source_dimension?.trim();
  if (!secondarySourceDimension) {
    return false;
  }
  const hasSecondarySourceVariation = skcPatches.some((patch) => {
    const values = new Set(
      (patch.sku_patches ?? [])
        .map((skuPatch) => {
          const entries = Object.entries(skuPatch.attributes ?? {});
          const match = entries.find(([key]) =>
            saleDimensionMatches(key, secondarySourceDimension),
          );
          return match?.[1]?.trim();
        })
        .filter((value): value is string => Boolean(value)),
    );
    return values.size > 1;
  });
  if (!hasSecondarySourceVariation) {
    return false;
  }
  return hasMatchingSecondaryTemplateOption(current);
}

function hasMatchingSecondaryTemplateOption(
  current: NonNullable<
    NonNullable<SheinEditorContext["sale_attributes"]>["current"]
  >,
) {
  const secondarySourceDimension = current.secondary_source_dimension?.trim();
  if (!secondarySourceDimension) {
    return false;
  }
  const hasSecondaryCandidate = (current.candidates ?? []).some((candidate) => {
    if (
      !candidate.attribute_id ||
      candidate.attribute_id === current.primary_attribute_id
    ) {
      return false;
    }
    if (candidate.skc_scope) {
      return false;
    }
    return (
      saleDimensionMatches(
        candidate.source_dimension,
        secondarySourceDimension,
      ) || saleDimensionMatches(candidate.name, secondarySourceDimension)
    );
  });
  if (hasSecondaryCandidate) {
    return true;
  }
  return (current.template_options ?? []).some((option) => {
    if (
      !option.attribute_id ||
      option.attribute_id === current.primary_attribute_id
    ) {
      return false;
    }
    if (option.skc_scope) {
      return false;
    }
    return (
      saleDimensionMatches(option.name, secondarySourceDimension) ||
      saleDimensionMatches(option.name_en, secondarySourceDimension)
    );
  });
}

function saleDimensionMatches(left?: string | null, right?: string | null) {
  const normalizedLeft = normalizeSaleDimension(left);
  const normalizedRight = normalizeSaleDimension(right);
  return normalizedLeft !== "" && normalizedLeft === normalizedRight;
}

function normalizeSaleDimension(value?: string | null) {
  switch ((value ?? "").trim().toLowerCase()) {
    case "color":
    case "colour":
    case "颜色":
    case "颜色分类":
      return "color";
    case "size":
    case "尺码":
    case "尺寸":
    case "规格":
      return "size";
    case "quantity":
    case "count":
    case "件数":
    case "数量":
      return "quantity";
    case "style":
    case "style type":
    case "款式":
    case "类型":
      return "style";
    default:
      return (value ?? "").trim().toLowerCase();
  }
}

function formatTemplateOptionLabel(option: SheinSaleAttributeTemplateOption) {
  const base =
    option.name_en ?? option.name ?? String(option.attribute_id ?? "");
  if (option.important) {
    return `${base} · 主规格`;
  }
  return base;
}

function normalizeSaleAttributeToken(value?: string) {
  return (value ?? "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "");
}

function hasManualSelection(selection?: ManualSaleAttributeSelection) {
  return Boolean(selection?.valueId) || Boolean(selection?.textValue?.trim());
}

function formatResolvedAttributeValue(
  attribute?: SheinResolvedSaleAttribute,
  fallback = "未识别",
) {
  if (!attribute) {
    return fallback;
  }
  const name = attribute.name ?? "未命名属性";
  const value = attribute.value ?? "未填写";
  return `${name}：${value}`;
}

function formatResolvedAttributeMap(attribute?: SheinResolvedSaleAttribute) {
  if (!attribute?.attribute_id) {
    return undefined;
  }
  return `attribute_id ${attribute.attribute_id}${
    attribute.attribute_value_id
      ? ` · value_id ${attribute.attribute_value_id}`
      : ""
  }`;
}

function findCustomValueDeniedNote(notes?: string[] | null) {
  return (notes ?? []).find(
    (note) =>
      note.includes("没有自定义属性值权限") ||
      note.includes("不支持自定义值") ||
      note.includes("已跳过自定义尝试"),
  );
}
