"use client";

import type { ReactNode } from "react";

import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";

type SDSFieldRegistration = Record<string, unknown>;

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint: string;
  children: ReactNode;
}) {
  return (
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">{label}</span>
      {children}
      <p className="text-sm leading-6 text-zinc-500">{hint}</p>
    </Label>
  );
}

function inputClassName() {
  return "rounded-2xl px-4 py-3";
}

export function TaskSDSOptions({
  enabled,
  onEnabledChange,
  variantIdRegistration,
  parentProductIdRegistration,
  prototypeGroupIdRegistration,
  layerIdRegistration,
  designTypeRegistration,
  fitLevelRegistration,
  resizeModeRegistration,
}: {
  enabled: boolean;
  onEnabledChange: (enabled: boolean) => void;
  variantIdRegistration: SDSFieldRegistration;
  parentProductIdRegistration: SDSFieldRegistration;
  prototypeGroupIdRegistration: SDSFieldRegistration;
  layerIdRegistration: SDSFieldRegistration;
  designTypeRegistration: SDSFieldRegistration;
  fitLevelRegistration: SDSFieldRegistration;
  resizeModeRegistration: SDSFieldRegistration;
}) {
  return (
    <section className="space-y-4 rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-sm font-medium text-zinc-900">SDS 同步设置</h2>
          <p className="text-sm leading-6 text-zinc-500">
            如果你需要把设计素材回写到 SDS，可以在这里补充对应的商品和图层信息。
          </p>
        </div>
        <Label className="inline-flex items-center gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900">
          <Checkbox
            checked={enabled}
            onChange={(event) => onEnabledChange(event.target.checked)}
          />
          <span>启用 SDS 同步</span>
        </Label>
      </div>

      {enabled ? (
        <div className="grid gap-4 md:grid-cols-2">
          <Field
            label="Variant ID"
            hint="Required. SDS child SKU ID. Sync will not run unless this value is a positive integer."
          >
            <Input
              className={inputClassName()}
              inputMode="numeric"
              placeholder="89764"
              {...variantIdRegistration}
            />
          </Field>

          <Field
            label="Parent product ID"
            hint="Optional. Parent SDS product ID. Leave blank when you want the backend to infer it from the variant."
          >
            <Input
              className={inputClassName()}
              inputMode="numeric"
              placeholder="89763"
              {...parentProductIdRegistration}
            />
          </Field>

          <Field
            label="Prototype group ID"
            hint="Optional. When blank, the backend will use the current design page default template group."
          >
            <Input
              className={inputClassName()}
              inputMode="numeric"
              placeholder="14555"
              {...prototypeGroupIdRegistration}
            />
          </Field>

          <Field
            label="Layer ID"
            hint="Optional. Target print layer. Leave blank to let the backend choose the main design layer."
          >
            <Input
              className={inputClassName()}
              placeholder="698744758333792256"
              {...layerIdRegistration}
            />
          </Field>

          <Field
            label="Design type"
            hint="Use material for the current SDS design page workflow unless the backend contract changes."
          >
            <Select className={inputClassName()} {...designTypeRegistration}>
              <option value="material">material</option>
              <option value="image">image</option>
            </Select>
          </Field>

          <Field
            label="Fit level"
            hint="Optional scaling level. The live SDS flow has been validated with 1."
          >
            <Input
              className={inputClassName()}
              inputMode="decimal"
              placeholder="1"
              {...fitLevelRegistration}
            />
          </Field>

          <Field
            label="Resize mode"
            hint="Optional scaling mode. The live SDS flow has been validated with 0."
          >
            <Input
              className={inputClassName()}
              inputMode="numeric"
              placeholder="0"
              {...resizeModeRegistration}
            />
          </Field>

          <div className="rounded-2xl border border-sky-200 bg-sky-50 px-4 py-4 text-sm leading-6 text-sky-900 md:col-span-2">
            Verified SDS test values: variant <span className="font-mono">89764</span>,
            parent product <span className="font-mono">89763</span>, prototype group{" "}
            <span className="font-mono">14555</span>, layer{" "}
            <span className="font-mono">698744758333792256</span>.
          </div>
        </div>
      ) : null}
    </section>
  );
}
