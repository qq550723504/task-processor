import { scopeLabel } from "@/components/listingkit/shein/shein-sale-attribute-review-card-model";
import type {
  SheinResolvedSaleAttribute,
  SheinSaleAttributeCandidateInfo,
} from "@/lib/types/listingkit";

export function SectionHeading({
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

export function SaleAttributeList({
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

export function CandidateReasonList({
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
