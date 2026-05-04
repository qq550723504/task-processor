import { useState } from "react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type {
  SheinEditorContext,
  SheinPendingAttributeCandidate,
  SheinResolvedAttribute,
} from "@/lib/types/listingkit";

function presentAttributeReviewStatus(status?: string) {
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
  isApplying,
  onConfirmAttributes,
}: {
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onConfirmAttributes?: (attributes: SheinResolvedAttribute[]) => void;
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
  pendingCandidates,
  recommendedCandidates,
  resolvedAttributes,
}: {
  current: NonNullable<NonNullable<SheinEditorContext["attributes"]>["current"]>;
  isApplying?: boolean;
  onConfirmAttributes?: (attributes: SheinResolvedAttribute[]) => void;
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
  const selectedAttributes = buildSelectedAttributes(
    selectableCandidates,
    selectedValues,
  );
  const canConfirm = selectedAttributes.length > 0 && Boolean(onConfirmAttributes);

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
              tone="secondary"
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
                tone="secondary"
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
                tone="secondary"
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

function pendingCandidatesSignature(candidates: SheinPendingAttributeCandidate[]) {
  return candidates
    .map((candidate) => `${candidate.attribute_id ?? ""}:${candidate.name ?? ""}`)
    .join("|");
}

function PendingCandidateRow({
  candidate,
  tone = "pending",
  value,
  onChange,
}: {
  candidate: SheinPendingAttributeCandidate;
  tone?: "pending" | "recommended";
  value: string;
  onChange: (value: string) => void;
}) {
  const options = candidate.attribute_value_list ?? [];
  const borderClass =
    tone === "recommended"
      ? "border-sky-200"
      : "border-amber-200";
  return (
    <label className={`block rounded-xl border ${borderClass} bg-white px-3 py-2`}>
      <span className="block text-sm font-medium text-zinc-950">
        {candidate.name ?? candidate.attribute_name_en ?? candidate.attribute_name}
      </span>
      <span className="mt-1 block text-[11px] uppercase tracking-[0.16em] text-zinc-500">
        attribute_id {candidate.attribute_id}
        {candidate.required ? " · 必填" : ""}
        {candidate.important ? " · 重要" : ""}
      </span>
      <span className="mt-1 block text-xs leading-5 text-zinc-600">
        {candidate.required
          ? "SHEIN 模板必填，未确认会阻断提交。"
          : candidate.important
            ? "SHEIN 重要属性，建议补齐但不作为阻断。"
            : "建议属性，不影响提交。"}
      </span>
      {options.length > 0 ? (
        <select
          className="mt-2 h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm text-zinc-800"
          value={value}
          onChange={(event) => onChange(event.target.value)}
        >
          <option value="">选择 SHEIN 属性值</option>
          {options.map((option) => (
            <option
              key={option.attribute_value_id}
              value={String(option.attribute_value_id)}
            >
              {option.value_en || option.value || option.attribute_value_id}
              {option.value && option.value_en ? ` / ${option.value}` : ""}
            </option>
          ))}
        </select>
      ) : (
        <p className="mt-2 text-sm text-zinc-600">
          这个模板属性没有可选值。当前 MVP 暂不支持手工文本录入。
        </p>
      )}
    </label>
  );
}

function buildSelectedAttributes(
  candidates: SheinPendingAttributeCandidate[],
  selectedValues: Record<string, string>,
): SheinResolvedAttribute[] {
  return candidates.flatMap((candidate) => {
    const key = String(candidate.attribute_id ?? candidate.name);
    const selectedValueID = Number(selectedValues[key]);
    if (!candidate.attribute_id || !selectedValueID) {
      return [];
    }
    const selectedValue = candidate.attribute_value_list?.find(
      (option) => option.attribute_value_id === selectedValueID,
    );
    return [
      {
        name: candidate.name ?? candidate.attribute_name_en ?? candidate.attribute_name,
        value:
          selectedValue?.value_en ??
          selectedValue?.value ??
          String(selectedValueID),
        attribute_id: candidate.attribute_id,
        attribute_value_id: selectedValueID,
        attribute_type: candidate.attribute_type,
        attribute_mode: candidate.attribute_mode,
        data_dimension: candidate.data_dimension,
        cascade_attribute_id: candidate.cascade_attribute_id,
        matched_by: "manual_attribute_review",
        required: candidate.required,
        important: candidate.important,
        skc_scope: candidate.skc_scope,
      },
    ];
  });
}
