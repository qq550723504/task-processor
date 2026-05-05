import { Card } from "@/components/shared/card";
import type {
  SheinEditorContext,
  SheinResolvedSaleAttribute,
  SheinSaleAttributeCandidateInfo,
} from "@/lib/types/listingkit";

function presentSaleReviewStatus(status?: string) {
  switch (status) {
    case "resolved":
      return "已完成";
    case "partial":
      return "待补齐";
    case "blocked":
      return "有阻断";
    default:
      return status;
  }
}

function SaleAttributeRow({
  scope,
  name,
  value,
  mapped,
}: {
  scope?: string;
  name?: string;
  value?: string;
  mapped?: string;
}) {
  if (!name && !value) {
    return null;
  }

  return (
    <div className="rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        {scope ? (
          <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
            {scopeLabel(scope)}
          </span>
        ) : null}
        <p className="text-sm font-medium text-zinc-900">{name}</p>
      </div>
      {value ? <p className="mt-1 text-sm text-zinc-700">{value}</p> : null}
      {mapped ? (
        <p className="mt-1 text-[11px] uppercase tracking-[0.16em] text-zinc-500">
          {mapped}
        </p>
      ) : null}
    </div>
  );
}

export function SheinSaleAttributeReviewCard({
  editorContext,
}: {
  editorContext?: SheinEditorContext | null;
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

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SHEIN 销售属性确认
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            检查主规格、其他规格和 SDS 变体值是否完整映射到 SHEIN 销售属性。
          </p>
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

function SectionHeading({
  title,
  description,
  tone = "zinc",
}: {
  title: string;
  description: string;
  tone?: "amber" | "emerald" | "sky" | "zinc";
}) {
  const toneClass =
    tone === "amber"
      ? "text-amber-700"
      : tone === "emerald"
        ? "text-emerald-700"
        : tone === "sky"
          ? "text-sky-700"
          : "text-zinc-600";
  return (
    <div>
      <p className={`text-xs font-semibold uppercase tracking-[0.18em] ${toneClass}`}>
        {title}
      </p>
      <p className="mt-1 text-sm leading-6 text-zinc-700">{description}</p>
    </div>
  );
}

function SaleAttributeList({
  attributes,
  scopeFallback,
}: {
  attributes: SheinResolvedSaleAttribute[];
  scopeFallback: "skc" | "sku";
}) {
  if (!attributes.length) {
    return null;
  }
  return (
    <div className="space-y-2">
      {attributes.map((attribute, index) => (
        <SaleAttributeRow
          key={`${scopeFallback}-${index}-${attribute.name}-${attribute.value}`}
          scope={attribute.scope ?? scopeFallback}
          name={attribute.name}
          value={attribute.value}
          mapped={
            attribute.attribute_id
              ? `attribute_id ${attribute.attribute_id}${
                  attribute.attribute_value_id
                    ? ` · value_id ${attribute.attribute_value_id}`
                    : ""
                }`
              : undefined
          }
        />
      ))}
    </div>
  );
}

function CandidateReasonList({
  candidates,
  emptyText,
}: {
  candidates: SheinSaleAttributeCandidateInfo[];
  emptyText?: string;
}) {
  if (!candidates.length) {
    return emptyText ? (
      <p className="rounded-xl border border-zinc-200/80 bg-white/70 px-3 py-2 text-sm text-zinc-600">
        {emptyText}
      </p>
    ) : null;
  }
  return (
    <div className="space-y-2">
      {candidates.map((candidate, index) => (
        <div
          className="rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2"
          key={`candidate-${index}-${candidate.name}-${candidate.source_dimension}`}
        >
          <p className="text-sm font-medium text-zinc-900">
            {candidate.name ?? "未命名候选"}
          </p>
          <p className="mt-1 text-xs leading-5 text-zinc-600">
            {candidate.source_dimension ?? "未知来源维度"}
            {candidate.selected_scope ? ` · ${scopeLabel(candidate.selected_scope)}` : ""}
            {candidate.attribute_id ? ` · attribute_id ${candidate.attribute_id}` : ""}
          </p>
          {candidate.reasons?.length ? (
            <p className="mt-1 text-xs leading-5 text-zinc-600">
              {candidate.reasons.join("；")}
            </p>
          ) : null}
        </div>
      ))}
    </div>
  );
}

function matchingCandidates(
  candidates: SheinSaleAttributeCandidateInfo[],
  attributeID?: number,
  scope?: string,
) {
  return candidates.filter((candidate) => {
    const matchesAttribute = attributeID
      ? candidate.attribute_id === attributeID
      : false;
    return matchesAttribute || candidate.selected_scope === scope;
  });
}

function scopeLabel(scope: string) {
  switch (scope) {
    case "primary":
      return "主规格";
    case "secondary":
      return "其他规格";
    case "skc":
      return "主规格/SKC";
    case "sku":
      return "其他规格/SKU";
    default:
      return scope;
  }
}
