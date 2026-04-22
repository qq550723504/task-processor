import { Card } from "@/components/shared/card";
import type { SheinEditorContext } from "@/lib/types/listingkit";

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
            {scope}
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
  const candidates = current.candidates?.slice(0, 3) ?? [];
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
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SHEIN sale attribute review
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            Review current sale attribute selection and candidate fit before submission.
          </p>
        </div>

        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
          {current.status ? <span>Status {current.status}</span> : null}
          {current.primary_attribute_id ? (
            <span>Primary {current.primary_attribute_id}</span>
          ) : null}
          {current.secondary_attribute_id ? (
            <span>Secondary {current.secondary_attribute_id}</span>
          ) : null}
        </div>

        {skcAttributes.length > 0 || skuAttributes.length > 0 ? (
          <div className="space-y-2">
            {skcAttributes.map((attribute, index) => (
              <SaleAttributeRow
                key={`skc-${index}-${attribute.name}-${attribute.value}`}
                scope={attribute.scope ?? "skc"}
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
            {skuAttributes.map((attribute, index) => (
              <SaleAttributeRow
                key={`sku-${index}-${attribute.name}-${attribute.value}`}
                scope={attribute.scope ?? "sku"}
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
        ) : null}

        {candidates.length > 0 ? (
          <div className="space-y-2">
            {candidates.map((candidate, index) => (
              <div
                className="rounded-xl border border-zinc-200/80 bg-zinc-50 px-3 py-2"
                key={`candidate-${index}-${candidate.name}-${candidate.source_dimension}`}
              >
                <p className="text-sm font-medium text-zinc-900">
                  {candidate.name}
                </p>
                <p className="mt-1 text-xs leading-5 text-zinc-600">
                  {candidate.source_dimension}
                  {candidate.selected_scope ? ` · ${candidate.selected_scope}` : ""}
                </p>
                {candidate.reasons?.length ? (
                  <p className="mt-1 text-xs leading-5 text-zinc-600">
                    {candidate.reasons.join("；")}
                  </p>
                ) : null}
              </div>
            ))}
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
            {current.review_notes.map((note) => (
              <p className="text-sm leading-6 text-zinc-700" key={note}>
                {note}
              </p>
            ))}
          </div>
        ) : null}
      </div>
    </Card>
  );
}
