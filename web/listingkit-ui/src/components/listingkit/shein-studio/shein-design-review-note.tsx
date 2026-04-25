"use client";

type QuickNote = {
  label: string;
  value: string;
};

const QUICK_NOTES: QuickNote[] = [
  { label: "Too busy", value: "Too busy for print." },
  { label: "Thin lines", value: "Too many thin lines." },
  { label: "Small text", value: "Text is too small." },
  { label: "Weak contrast", value: "Contrast is too weak." },
];

export function SheinDesignReviewNote({
  disabled = false,
  note,
  onChange,
}: {
  disabled?: boolean;
  note?: string;
  onChange?: (value: string) => void;
}) {
  if (!onChange && disabled) {
    return note ? (
      <div className="rounded-[1rem] border border-dashed border-zinc-200 bg-zinc-50 px-3 py-3 text-xs leading-6 text-zinc-500">
        Review note: {note}
      </div>
    ) : null;
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        {QUICK_NOTES.map((item) => (
          <button
            className="rounded-full border border-zinc-200 bg-white px-2 py-1 text-[11px] font-semibold text-zinc-600 transition hover:bg-zinc-100"
            key={item.label}
            onClick={() => onChange?.(item.value)}
            type="button"
          >
            {item.label}
          </button>
        ))}
      </div>
      <textarea
        className="min-h-20 w-full rounded-[1rem] border border-zinc-200 bg-white px-3 py-3 text-xs text-zinc-700 outline-none transition focus:border-zinc-950"
        disabled={disabled}
        onChange={(event) => onChange?.(event.target.value)}
        placeholder="Optional review note for this style."
        value={note ?? ""}
      />
    </div>
  );
}
