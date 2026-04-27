export function SheinStudioProgressStrip({
  createdTaskCount,
  generatedStyleCount,
  selectedStyleCount,
}: {
  createdTaskCount: number;
  generatedStyleCount: number;
  selectedStyleCount: number;
}) {
  return (
    <div className="mb-3 grid gap-3 rounded-[1.5rem] border border-zinc-200 bg-white/80 px-4 py-4 text-sm shadow-sm md:grid-cols-3">
      <ProgressMetric
        label="Generated"
        value={generatedStyleCount > 0 ? `${generatedStyleCount} styles` : "Not yet"}
      />
      <ProgressMetric
        label="Approved"
        value={selectedStyleCount > 0 ? `${selectedStyleCount} selected` : "None"}
      />
      <ProgressMetric
        label="SHEIN tasks"
        value={createdTaskCount > 0 ? `${createdTaskCount} created` : "Pending"}
      />
    </div>
  );
}

function ProgressMetric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-zinc-400">
        {label}
      </p>
      <p className="mt-1 text-lg font-semibold text-zinc-950">{value}</p>
    </div>
  );
}
