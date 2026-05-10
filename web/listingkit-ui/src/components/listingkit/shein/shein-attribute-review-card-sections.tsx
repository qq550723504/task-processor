import type { SheinPendingAttributeCandidate } from "@/lib/types/listingkit";

export function AttributeRow({
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

export function PendingCandidateRow({
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
