import { Card } from "@/components/shared/card";
import type { SheinEditorContext } from "@/lib/types/listingkit";

function AttributeRow({
  name,
  value,
  mapped,
}: {
  name?: string;
  value?: string;
  mapped?: string;
}) {
  if (!name) {
    return null;
  }

  return (
    <div className="rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2">
      <p className="text-sm font-medium text-zinc-900">{name}</p>
      {value ? <p className="mt-1 text-sm text-zinc-700">{value}</p> : null}
      {mapped ? (
        <p className="mt-1 text-[11px] uppercase tracking-[0.16em] text-zinc-500">
          {mapped}
        </p>
      ) : null}
    </div>
  );
}

export function SheinAttributeReviewCard({
  editorContext,
}: {
  editorContext?: SheinEditorContext | null;
}) {
  const current = editorContext?.attributes?.current;
  if (!current) {
    return null;
  }

  const resolvedAttributes = current.resolved_attributes?.slice(0, 4) ?? [];
  const hasSignal =
    Boolean(current.status) ||
    Boolean(current.review_notes?.length) ||
    resolvedAttributes.length > 0;

  if (!hasSignal) {
    return null;
  }

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SHEIN attribute review
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            Review current attribute mapping status before final submission.
          </p>
        </div>

        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
          {current.status ? <span>Status {current.status}</span> : null}
          {typeof current.resolved_count === "number" ? (
            <span>Resolved {current.resolved_count}</span>
          ) : null}
          {typeof current.unresolved_count === "number" ? (
            <span>Unresolved {current.unresolved_count}</span>
          ) : null}
        </div>

        {resolvedAttributes.length > 0 ? (
          <div className="space-y-2">
            {resolvedAttributes.map((attribute) => (
              <AttributeRow
                key={`${attribute.name}-${attribute.value}`}
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
