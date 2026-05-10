import type { ReactNode } from "react";

export function SectionHeading({
  description,
  eyebrow,
  title,
}: {
  description: string;
  eyebrow: string;
  title: string;
}) {
  return (
    <div>
      <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
        {eyebrow}
      </div>
      <h3 className="mt-1 text-lg font-semibold tracking-[-0.02em] text-zinc-950">
        {title}
      </h3>
      <p className="mt-1 text-xs leading-5 text-zinc-600">{description}</p>
    </div>
  );
}

export function clampProductImageCount(value: string) {
  const parsed = parsePositiveInteger(value);
  if (!Number.isFinite(parsed)) {
    return 1;
  }
  return Math.min(9, Math.max(1, parsed));
}

export function parsePositiveInteger(value: string) {
  return Number.parseInt(value.trim(), 10);
}

export function NumberInput({
  label,
  max,
  min,
  setValue,
  value,
}: {
  label: string;
  max: number;
  min: number;
  setValue: (value: string) => void;
  value: string;
}) {
  return (
    <label className="space-y-2">
      <span className="text-sm font-medium text-zinc-700">{label}</span>
      <input
        className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
        inputMode="numeric"
        max={max}
        min={min}
        onChange={(event) => setValue(event.target.value)}
        value={value}
      />
    </label>
  );
}

export function GenerationMessages({
  creatingError,
  creatingMessage,
  generationError,
  saveMessage,
  selectedStyleCount,
  selectionReady,
}: {
  creatingError: string;
  creatingMessage: string;
  generationError: string;
  saveMessage: string;
  selectedStyleCount: number;
  selectionReady: boolean;
}) {
  return (
    <>
      {!selectionReady ? (
        <Message tone="info">
          当前还不能生成或创建任务，请先回到第 1 步完成 SDS 商品选择。
        </Message>
      ) : null}
      {generationError ? (
        <Message tone="error">{generationError}</Message>
      ) : null}
      {creatingError ? <Message tone="error">{creatingError}</Message> : null}
      {creatingMessage ? <Message tone="info">{creatingMessage}</Message> : null}
      {selectedStyleCount > 0 ? (
        <Message tone="success">
          已选择 {selectedStyleCount} 个款式用于 SHEIN 审核。
        </Message>
      ) : null}
      {saveMessage ? <Message tone="neutral">{saveMessage}</Message> : null}
    </>
  );
}

export function Message({
  children,
  tone,
}: {
  children: ReactNode;
  tone: "error" | "info" | "neutral" | "success";
}) {
  const classes = {
    error: "border-rose-200 bg-rose-50 text-rose-700",
    info: "border-sky-200 bg-sky-50 text-sky-800",
    neutral: "border-zinc-200 bg-zinc-50 text-zinc-600",
    success: "border-emerald-200 bg-emerald-50 text-emerald-800",
  };

  return (
    <div className={`rounded-2xl border px-4 py-3 text-sm ${classes[tone]}`}>
      {children}
    </div>
  );
}
