import { useEffect, useMemo, useState } from "react";

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
  SheinSaleAttributeTemplateOption,
} from "@/lib/types/listingkit";

type ManualSaleAttributeSelection = {
  valueId?: number;
  textValue?: string;
};

export function SheinSaleAttributeReviewCard({
  editorContext,
  isApplying,
  onConfirmCurrentSaleAttributes,
  onRegenerateSaleAttributes,
  onApplyManualSaleAttributes,
}: {
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onConfirmCurrentSaleAttributes?: (() => void) | null;
  onRegenerateSaleAttributes?: (() => void) | null;
  onApplyManualSaleAttributes?: ((payload: {
    primaryOption?: SheinSaleAttributeTemplateOption | null;
    secondaryOption?: SheinSaleAttributeTemplateOption | null;
    skcSelections: Record<string, ManualSaleAttributeSelection>;
    skuSelections: Record<string, ManualSaleAttributeSelection>;
  }) => void) | null;
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
      current={current}
      isApplying={isApplying}
      key={`${current.status ?? ""}-${current.primary_attribute_id ?? 0}-${current.secondary_attribute_id ?? 0}-${current.template_options?.length ?? 0}-${current.skc_patches?.length ?? 0}`}
      onApplyManualSaleAttributes={onApplyManualSaleAttributes}
      onConfirmCurrentSaleAttributes={onConfirmCurrentSaleAttributes}
      onRegenerateSaleAttributes={onRegenerateSaleAttributes}
    />
  );
}

function SheinSaleAttributeReviewContent({
  current,
  isApplying,
  onConfirmCurrentSaleAttributes,
  onRegenerateSaleAttributes,
  onApplyManualSaleAttributes,
}: {
  current: NonNullable<NonNullable<SheinEditorContext["sale_attributes"]>["current"]>;
  isApplying?: boolean;
  onConfirmCurrentSaleAttributes?: (() => void) | null;
  onRegenerateSaleAttributes?: (() => void) | null;
  onApplyManualSaleAttributes?: ((payload: {
    primaryOption?: SheinSaleAttributeTemplateOption | null;
    secondaryOption?: SheinSaleAttributeTemplateOption | null;
    skcSelections: Record<string, ManualSaleAttributeSelection>;
    skuSelections: Record<string, ManualSaleAttributeSelection>;
  }) => void) | null;
}) {
  const manualTemplateOptions = useMemo(
    () =>
      (current.template_options ?? []).filter(
        (option) => (option.attribute_value_list?.length ?? 0) > 0,
      ),
    [current.template_options],
  );
  const skcAttributes = current.skc_attributes?.slice(0, 2) ?? [];
  const skuAttributes = current.sku_attributes?.slice(0, 3) ?? [];
  const candidates = current.candidates ?? [];
  const hasMissingValueIDs =
    skcAttributes.some((attribute) => !attribute.attribute_value_id) ||
    skuAttributes.some((attribute) => !attribute.attribute_value_id);
  const initialPrimaryOptionID = String(
    pickTemplateOptionID({
      options: manualTemplateOptions,
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
      ignoreCurrentSelection: hasMissingValueIDs,
      scope: "secondary",
      sourceDimension: current.secondary_source_dimension,
    }) ?? "",
  );
  const [primaryOptionID, setPrimaryOptionID] = useState(initialPrimaryOptionID);
  const [secondaryOptionID, setSecondaryOptionID] = useState(initialSecondaryOptionID);
  const [skcSelections, setSKCSelections] = useState<Record<string, ManualSaleAttributeSelection>>({});
  const [skuSelections, setSKUSelections] = useState<Record<string, ManualSaleAttributeSelection>>({});

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
    Boolean(current.primary_attribute_id) &&
    hasMissingValueIDs;
  const primaryOption =
    manualTemplateOptions.find(
      (option) => String(option.attribute_id ?? "") === primaryOptionID,
    ) ?? null;
  const secondaryTemplateOptions = useMemo(
    () =>
      manualTemplateOptions.filter(
        (option) => option.attribute_id !== primaryOption?.attribute_id,
      ),
    [manualTemplateOptions, primaryOption?.attribute_id],
  );
  const secondaryOption =
    secondaryTemplateOptions.find(
      (option) => String(option.attribute_id ?? "") === secondaryOptionID,
    ) ?? null;
  const canManualEdit =
    Boolean(onApplyManualSaleAttributes) &&
    (current.skc_patches?.length ?? 0) > 0 &&
    manualTemplateOptions.length > 0;
  const allSKCSelected =
    (current.skc_patches ?? []).length > 0 &&
    (current.skc_patches ?? []).every(
      (patch) => patch.supplier_code && hasManualSelection(skcSelections[patch.supplier_code]),
    );
  const allSKUSelected =
    !secondaryOption ||
    (current.skc_patches ?? []).flatMap((patch) => patch.sku_patches ?? []).every(
      (patch) => patch.supplier_sku && hasManualSelection(skuSelections[patch.supplier_sku]),
    );
  const canSaveManual =
    canManualEdit && Boolean(primaryOption?.attribute_id) && allSKCSelected && allSKUSelected;

  useEffect(() => {
    if (
      secondaryOptionID &&
      secondaryTemplateOptions.some(
        (option) => String(option.attribute_id ?? "") === secondaryOptionID,
      )
    ) {
      return;
    }
    setSecondaryOptionID(
      String(
        pickTemplateOptionID({
          options: secondaryTemplateOptions,
          candidates,
          currentAttributeID: current.secondary_attribute_id,
          emptyFallback: true,
          ignoreCurrentSelection: hasMissingValueIDs,
          scope: "secondary",
          sourceDimension: current.secondary_source_dimension,
        }) ?? "",
      ),
    );
  }, [
    candidates,
    current.secondary_attribute_id,
    current.secondary_source_dimension,
    hasMissingValueIDs,
    secondaryOptionID,
    secondaryTemplateOptions,
  ]);

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3 xl:grid xl:grid-cols-[minmax(0,1fr),auto] xl:items-start">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 销售属性确认
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-700">
              检查主规格、其他规格和 SDS 变体值是否完整映射到 SHEIN 销售属性。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canRegenerate ? (
              <Button
                className="h-9 shrink-0 px-3 text-xs"
                disabled={isApplying}
                variant="secondary"
                onClick={() => onRegenerateSaleAttributes?.()}
              >
                按当前类目重新生成属性
              </Button>
            ) : null}
            {canSaveManual ? (
              <Button
                className="h-9 shrink-0 px-3 text-xs"
                disabled={isApplying}
                onClick={() =>
                  onApplyManualSaleAttributes?.({
                    primaryOption,
                    secondaryOption,
                    skcSelections,
                    skuSelections,
                  })
                }
              >
                保存手工销售属性
              </Button>
            ) : null}
            {canConfirm ? (
              <Button
                className="h-9 shrink-0 px-3 text-xs"
                disabled={isApplying}
                variant="secondary"
                onClick={() => onConfirmCurrentSaleAttributes?.()}
              >
                确认当前规格
              </Button>
            ) : null}
          </div>
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

        {hasMissingValueIDs ? (
          <div className="rounded-2xl border border-amber-200 bg-amber-50/70 p-3">
            <p className="text-sm leading-6 text-amber-900">
              当前销售属性只有 `attribute_id`，还缺少真实 `value_id`，不能直接确认。
              你可以先按当前类目重新生成普通属性和销售属性；如果结果仍不准确，下面也可以手工修改销售属性。
            </p>
          </div>
        ) : null}

        {canManualEdit ? (
          <div className="space-y-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-3">
            <SectionHeading
              description="如果 AI 生成的销售属性不满意，可以按当前类目模板手工修改。保存后会写入 SKC/SKU 规格，并优先用文本值向 SHEIN 换取真实 value_id。"
              title="手工填写销售属性"
              tone="amber"
            />
            <div className="grid gap-3 xl:grid-cols-2">
              <TemplateOptionSelect
                id="shein-sale-attribute-primary-template"
                label={`主规格模板${current.primary_source_dimension ? ` · 来源 ${current.primary_source_dimension}` : ""}`}
                onChange={setPrimaryOptionID}
                options={manualTemplateOptions}
                value={primaryOptionID}
              />
              <TemplateOptionSelect
                allowEmpty
                id="shein-sale-attribute-secondary-template"
                label={`其他规格模板${current.secondary_source_dimension ? ` · 来源 ${current.secondary_source_dimension}` : ""}`}
                onChange={setSecondaryOptionID}
                options={secondaryTemplateOptions}
                placeholder="不填写其他规格"
                value={secondaryOptionID}
              />
            </div>
            <div className="space-y-3">
              {(current.skc_patches ?? []).map((patch) => (
                <ManualSKCMappingRow
                  key={patch.supplier_code ?? patch.skc_name}
                  patch={patch}
                  primaryOption={primaryOption}
                  secondaryOption={secondaryOption}
                  primarySourceDimension={current.primary_source_dimension}
                  secondarySourceDimension={current.secondary_source_dimension}
                  skcSelection={patch.supplier_code ? skcSelections[patch.supplier_code] : undefined}
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
          </div>
        ) : null}

        <div className="grid gap-4 xl:grid-cols-2">
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

          {candidates.length > 0 ? (
            <details
              className={`rounded-2xl border p-3 ${
                isPartial
                  ? "border-amber-200 bg-amber-50/70"
                  : "border-zinc-200 bg-zinc-50/80"
              }`}
              id="shein-sale-attribute-unresolved-group"
              open={isPartial}
            >
              <summary className="cursor-pointer list-none">
                <SectionHeading
                  description="这里展示 resolver 看到的候选维度和拟合原因，用来判断是否覆盖了 SDS 颜色、尺码或款式。"
                  title="变体覆盖检查"
                  tone={isPartial ? "amber" : "zinc"}
                />
              </summary>
              <div className="mt-3 space-y-3">
                <CandidateReasonList candidates={candidates} />
              </div>
            </details>
          ) : null}
        </div>

        {current.selection_summary?.length || current.review_notes?.length ? (
          <details className="rounded-2xl border border-zinc-200 bg-zinc-50/80 p-3">
            <summary className="cursor-pointer list-none text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              处理说明
            </summary>
            <div className="mt-3 space-y-2">
              {current.selection_summary?.map((line) => (
                <p className="text-sm leading-6 text-zinc-700" key={line}>
                  {line}
                </p>
              ))}
              {current.review_notes?.map((note, index) => (
                <p className="text-sm leading-6 text-zinc-700" key={`${index}-${note}`}>
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
    <Label className="block rounded-xl border border-zinc-200 bg-white px-3 py-2" htmlFor={id}>
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
          <option key={option.attribute_id} value={String(option.attribute_id ?? "")}>
            {option.name_en ?? option.name ?? option.attribute_id}
          </option>
        ))}
      </Select>
    </Label>
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
  onSKUChange: (supplierSKU: string, selection: ManualSaleAttributeSelection) => void;
}) {
  return (
    <div className="space-y-3 rounded-xl border border-zinc-200 bg-white/80 p-3">
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1.2fr),minmax(280px,1fr)]">
        <div>
          <p className="text-sm font-medium text-zinc-900">
            {patch.skc_name ?? patch.sale_name ?? patch.supplier_code}
          </p>
          <p className="mt-1 text-xs text-zinc-600">
            {primarySourceDimension || "主来源维度"}：
            {resolveSourceValue(patch.attributes, primarySourceDimension) || "未识别"}
          </p>
        </div>
        <ValueOptionSelect
          id={`shein-sale-attribute-skc-${patch.supplier_code ?? patch.skc_name ?? "unknown"}`}
          label="主规格值"
          onChange={onSKCChange}
          options={primaryOption?.attribute_value_list ?? []}
          sourceValue={resolveSourceValue(patch.attributes, primarySourceDimension)}
          value={skcSelection}
        />
      </div>
      {secondaryOption && (patch.sku_patches?.length ?? 0) > 0 ? (
        <div className="grid gap-2 xl:grid-cols-2">
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
                {resolveSourceValue(skuPatch.attributes, secondarySourceDimension) || "未识别"}
              </p>
              <div className="mt-2">
                <ValueOptionSelect
                  id={`shein-sale-attribute-sku-${skuPatch.supplier_sku ?? "unknown"}`}
                  label="其他规格值"
                  onChange={(selection) => {
                    if (skuPatch.supplier_sku) {
                      onSKUChange(skuPatch.supplier_sku, selection);
                    }
                  }}
                  options={secondaryOption.attribute_value_list ?? []}
                  sourceValue={resolveSourceValue(skuPatch.attributes, secondarySourceDimension)}
                  value={skuPatch.supplier_sku ? skuSelections[skuPatch.supplier_sku] : undefined}
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
  options: NonNullable<SheinSaleAttributeTemplateOption["attribute_value_list"]>;
  sourceValue?: string;
}) {
  const selectValue = value?.textValue?.trim() ? "" : String(value?.valueId ?? "");
  return (
    <Label className="block rounded-xl border border-zinc-200 bg-white px-3 py-2" htmlFor={id}>
      <span className="block text-sm font-medium text-zinc-950">{label}</span>
      <Select
        className="mt-2 rounded-xl"
        id={id}
        name={id}
        value={selectValue}
        onChange={(event) =>
          onChange({
            valueId: event.target.value ? Number(event.target.value) : undefined,
            textValue: value?.textValue,
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
          placeholder={sourceValue ? `手工输入，建议值：${sourceValue}` : "手工输入销售属性值"}
          value={value?.textValue ?? ""}
          onChange={(event) =>
            onChange({
              valueId: value?.valueId,
              textValue: event.target.value,
            })
          }
        />
        <p className="text-xs text-zinc-500">
          手工输入时会优先按文本调用 SHEIN 校验并创建真实值。
        </p>
      </div>
    </Label>
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

function pickTemplateOptionID({
  options,
  candidates,
  currentAttributeID,
  emptyFallback,
  ignoreCurrentSelection,
  scope,
  sourceDimension,
}: {
  options: SheinSaleAttributeTemplateOption[];
  candidates: NonNullable<
    NonNullable<NonNullable<SheinEditorContext["sale_attributes"]>["current"]>["candidates"]
  >;
  currentAttributeID?: number;
  emptyFallback?: boolean;
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
          options.some((option) => option.attribute_id === candidate.attribute_id),
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
  const byScopeFallback =
    scope === "primary"
      ? options.find((option) => option.skc_scope)
      : options.find((option) => !option.skc_scope);

  const ordered = ignoreCurrentSelection
    ? [bySourceCandidate, byScopedCandidate, bySourceName, byScopeFallback, byCurrent]
    : [byCurrent, bySourceCandidate, byScopedCandidate, bySourceName, byScopeFallback];
  const match = ordered.find((option): option is SheinSaleAttributeTemplateOption => Boolean(option));
  return match?.attribute_id ?? (emptyFallback ? "" : options[0]?.attribute_id);
}

function normalizeSaleAttributeToken(value?: string) {
  return (value ?? "").trim().toLowerCase().replace(/[^a-z0-9]+/g, "");
}

function hasManualSelection(selection?: ManualSaleAttributeSelection) {
  return Boolean(selection?.valueId) || Boolean(selection?.textValue?.trim());
}
