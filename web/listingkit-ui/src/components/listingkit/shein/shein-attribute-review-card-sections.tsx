import type { SheinPendingAttributeCandidate } from "@/lib/types/listingkit";
import type { SelectedAttributeInput } from "@/components/listingkit/shein/shein-attribute-review-card-model";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";

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
  value: SelectedAttributeInput;
  onChange: (value: SelectedAttributeInput) => void;
}) {
  const options = candidate.attribute_value_list ?? [];
  const allowsMultiple = (candidate.attribute_input_num ?? 1) > 1;
  const maxSelections = Math.max(candidate.attribute_input_num ?? 1, 1);
  const selectedValueIds = value.selectedValueIds ?? [];
  const borderClass =
    tone === "recommended"
      ? "border-sky-200"
      : "border-amber-200";
  return (
    <div className={`rounded-xl border ${borderClass} bg-white px-3 py-2`}>
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
        <div className="mt-2 space-y-2">
          {allowsMultiple ? (
            <div className="space-y-2 rounded-xl border border-zinc-200/80 bg-zinc-50/80 p-3">
              <p className="text-sm text-zinc-700">
                最多可选 {maxSelections} 个模板值。
              </p>
              <div className="space-y-2">
                {options.map((option) => {
                  const optionValue = String(option.attribute_value_id);
                  const checked = selectedValueIds.includes(optionValue);
                  const reachedLimit =
                    !checked && selectedValueIds.length >= maxSelections;
                  return (
                    <label
                      className="flex items-start gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2"
                      key={option.attribute_value_id}
                    >
                      <Checkbox
                        checked={checked}
                        disabled={reachedLimit}
                        onChange={(event) => {
                          const nextSelectedValueIds = event.target.checked
                            ? [...selectedValueIds, optionValue]
                            : selectedValueIds.filter((item) => item !== optionValue);
                          onChange({
                            ...value,
                            selectedValueIds: nextSelectedValueIds,
                          });
                        }}
                      />
                      <span className="text-sm text-zinc-800">
                        {option.value_en || option.value || option.attribute_value_id}
                        {option.value && option.value_en ? ` / ${option.value}` : ""}
                      </span>
                    </label>
                  );
                })}
              </div>
            </div>
          ) : (
            <Select
              className="rounded-xl"
              value={value.selectedValueId ?? ""}
              onChange={(event) =>
                onChange({
                  ...value,
                  selectedValueId: event.target.value,
                })
              }
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
            </Select>
          )}
          <Input
            placeholder="如模板要求手填补充值，可在这里输入数字或文本"
            value={value.extraValue ?? ""}
            onChange={(event) =>
              onChange({
                ...value,
                extraValue: event.target.value,
              })
            }
          />
          <p className="text-sm text-zinc-600">
            某些模板属性在选择候选值后，还需要补一个手工值；不确定时可以先留空，系统会继续按商品语义推导。
          </p>
        </div>
      ) : (
        <div className="mt-2 space-y-2">
          <Input
            placeholder="输入属性值，例如型号或材质"
            value={value.textValue ?? ""}
            onChange={(event) =>
              onChange({
                ...value,
                textValue: event.target.value,
              })
            }
          />
          <p className="text-sm text-zinc-600">
            这个模板属性没有可选值，请手工输入要写入 SHEIN 的文本属性值。
          </p>
        </div>
      )}
    </div>
  );
}
