import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  buildSelectedAttributes,
  pendingCandidatesSignature,
  presentAttributeReviewStatus,
} from "@/components/listingkit/shein/shein-attribute-review-card-model";
import {
  AttributeRow,
  PendingCandidateRow,
} from "@/components/listingkit/shein/shein-attribute-review-card-sections";
import type {
  SheinEditorContext,
  SheinPendingAttributeCandidate,
  SheinResolvedAttribute,
} from "@/lib/types/listingkit";

export function SheinAttributeReviewCard({
  editorContext,
  isApplying,
  onConfirmAttributes,
  onConfirmFallbackAttributes,
}: {
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onConfirmAttributes?: (attributes: SheinResolvedAttribute[]) => void;
  onConfirmFallbackAttributes?: () => void;
}) {
  const current = editorContext?.attributes?.current;

  if (!current) {
    return null;
  }

  const resolvedAttributes = current.resolved_attributes ?? [];
  const pendingCandidates = current.pending_attribute_candidates ?? [];
  const recommendedCandidates = current.recommended_attribute_candidates ?? [];
  const hasSignal =
    Boolean(current.status) ||
    Boolean(current.review_notes?.length) ||
    resolvedAttributes.length > 0 ||
    pendingCandidates.length > 0 ||
    recommendedCandidates.length > 0;

  if (!hasSignal) {
    return null;
  }

  return (
    <SheinAttributeReviewContent
      current={current}
      isApplying={isApplying}
      key={pendingCandidatesSignature([...pendingCandidates, ...recommendedCandidates])}
      onConfirmAttributes={onConfirmAttributes}
      onConfirmFallbackAttributes={onConfirmFallbackAttributes}
      pendingCandidates={pendingCandidates}
      recommendedCandidates={recommendedCandidates}
      resolvedAttributes={resolvedAttributes}
    />
  );
}

function SheinAttributeReviewContent({
  current,
  isApplying,
  onConfirmAttributes,
  onConfirmFallbackAttributes,
  pendingCandidates,
  recommendedCandidates,
  resolvedAttributes,
}: {
  current: NonNullable<NonNullable<SheinEditorContext["attributes"]>["current"]>;
  isApplying?: boolean;
  onConfirmAttributes?: (attributes: SheinResolvedAttribute[]) => void;
  onConfirmFallbackAttributes?: () => void;
  pendingCandidates: SheinPendingAttributeCandidate[];
  recommendedCandidates: SheinPendingAttributeCandidate[];
  resolvedAttributes: SheinResolvedAttribute[];
}) {
  const [selectedValues, setSelectedValues] = useState<Record<string, string>>({});
  const requiredCandidates = pendingCandidates.filter((candidate) => candidate.required);
  const nonRequiredPendingCandidates = pendingCandidates.filter(
    (candidate) => !candidate.required,
  );
  const importantCandidates = [
    ...nonRequiredPendingCandidates.filter((candidate) => candidate.important),
    ...recommendedCandidates.filter((candidate) => candidate.important),
  ];
  const optionalCandidates = [
    ...nonRequiredPendingCandidates.filter((candidate) => !candidate.important),
    ...recommendedCandidates.filter((candidate) => !candidate.important),
  ];
  const selectableCandidates = [
    ...requiredCandidates,
    ...importantCandidates,
    ...optionalCandidates,
  ];
  const fallbackPendingAttributes =
    selectableCandidates.length === 0 ? current.pending_attributes ?? [] : [];
  const selectedAttributes = buildSelectedAttributes(
    selectableCandidates,
    selectedValues,
  );
  const canConfirm = selectedAttributes.length > 0 && Boolean(onConfirmAttributes);
  const canConfirmFallback =
    fallbackPendingAttributes.length > 0 && Boolean(onConfirmFallbackAttributes);

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SHEIN 普通属性确认
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            先处理必填未完成项；重要建议和其他建议不会阻断提交，但补齐后资料更完整。
          </p>
        </div>

        <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
          {current.status ? (
            <span>状态 {presentAttributeReviewStatus(current.status)}</span>
          ) : null}
          {typeof current.resolved_count === "number" ? (
            <span>已确认 {current.resolved_count}</span>
          ) : null}
          {typeof current.unresolved_count === "number" ? (
            <span>未完成 {current.unresolved_count}</span>
          ) : null}
        </div>

        {requiredCandidates.length > 0 ? (
          <div
            className="space-y-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-3"
            id="shein-attribute-required-group"
          >
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
                必填未完成
              </p>
              <p className="mt-1 text-sm leading-6 text-amber-900">
                这些属性来自 SHEIN 类目模板，未确认前会阻断提交。
              </p>
            </div>
            <div className="space-y-2">
              {requiredCandidates.map((candidate) => (
                <PendingCandidateRow
                  candidate={candidate}
                  key={`${candidate.attribute_id}-${candidate.name}`}
                  value={selectedValues[String(candidate.attribute_id ?? candidate.name)] ?? ""}
                  onChange={(value) =>
                    setSelectedValues((currentValues) => ({
                      ...currentValues,
                      [String(candidate.attribute_id ?? candidate.name)]: value,
                    }))
                  }
                />
              ))}
            </div>
            <Button
              className="h-9"
              disabled={!canConfirm || isApplying}
              onClick={() => onConfirmAttributes?.(selectedAttributes)}
              variant="secondary"
            >
              {isApplying ? "保存中..." : "保存已选择属性"}
            </Button>
          </div>
        ) : null}

        {importantCandidates.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-sky-200 bg-sky-50/70 p-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-sky-700">
                重要建议
              </p>
              <p className="mt-1 text-sm leading-6 text-sky-900">
                这些不是必填阻断项，但 SHEIN 标记为重要属性，建议客户确认。
              </p>
            </div>
            <div className="space-y-2">
              {importantCandidates.map((candidate) => (
                <PendingCandidateRow
                  candidate={candidate}
                  key={`${candidate.attribute_id}-${candidate.name}`}
                  tone="recommended"
                  value={selectedValues[String(candidate.attribute_id ?? candidate.name)] ?? ""}
                  onChange={(value) =>
                    setSelectedValues((currentValues) => ({
                      ...currentValues,
                      [String(candidate.attribute_id ?? candidate.name)]: value,
                    }))
                  }
                />
              ))}
            </div>
            {requiredCandidates.length === 0 ? (
              <Button
                className="h-9"
                disabled={!canConfirm || isApplying}
                onClick={() => onConfirmAttributes?.(selectedAttributes)}
                variant="secondary"
              >
                {isApplying ? "保存中..." : "保存已选择属性"}
              </Button>
            ) : null}
          </div>
        ) : null}

        {optionalCandidates.length > 0 ? (
          <details className="rounded-2xl border border-zinc-200 bg-zinc-50/80 p-3">
            <summary className="cursor-pointer text-xs font-semibold uppercase tracking-[0.18em] text-zinc-600">
              其他建议属性（不阻断提交）
            </summary>
            <div className="mt-3 space-y-2">
              {optionalCandidates.map((candidate) => (
                <PendingCandidateRow
                  candidate={candidate}
                  key={`${candidate.attribute_id}-${candidate.name}`}
                  tone="recommended"
                  value={selectedValues[String(candidate.attribute_id ?? candidate.name)] ?? ""}
                  onChange={(value) =>
                    setSelectedValues((currentValues) => ({
                      ...currentValues,
                      [String(candidate.attribute_id ?? candidate.name)]: value,
                    }))
                  }
                />
              ))}
            </div>
            {requiredCandidates.length === 0 && importantCandidates.length === 0 ? (
              <Button
                className="mt-3 h-9"
                disabled={!canConfirm || isApplying}
                onClick={() => onConfirmAttributes?.(selectedAttributes)}
                variant="secondary"
              >
                {isApplying ? "保存中..." : "保存已选择属性"}
              </Button>
            ) : null}
          </details>
        ) : null}

        {resolvedAttributes.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
                已确认属性
              </p>
              <p className="mt-1 text-sm leading-6 text-emerald-900">
                这些属性已进入当前 SHEIN 资料。人工确认的同类解析会用于后续缓存命中。
              </p>
            </div>
            <div className="grid gap-2 lg:grid-cols-2">
              {resolvedAttributes.map((attribute) => (
                <AttributeRow
                  key={`${attribute.attribute_id ?? attribute.name}-${attribute.value}`}
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
          </div>
        ) : null}

        {fallbackPendingAttributes.length > 0 ? (
          <div
            className="space-y-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-3"
            id="shein-attribute-required-group"
          >
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-700">
                当前没有可选的 SHEIN 模板候选值
              </p>
              <p className="mt-1 text-sm leading-6 text-amber-900">
                当前只能看到 SDS 来源属性，无法选择真实 SHEIN attribute_id。内部测试可先按当前 SDS 属性确认，正式发布前仍建议重新获取模板后复核。
              </p>
            </div>
            <div className="grid gap-2 lg:grid-cols-2">
              {fallbackPendingAttributes.map((attribute) => (
                <AttributeRow
                  key={`${attribute.name ?? ""}-${attribute.value ?? ""}`}
                  name={attribute.name}
                  value={attribute.value}
                />
              ))}
            </div>
            <Button
              className="h-9"
              disabled={!canConfirmFallback || isApplying}
              onClick={() => onConfirmFallbackAttributes?.()}
              variant="secondary"
            >
              {isApplying ? "保存中..." : "按当前 SDS 属性确认"}
            </Button>
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
