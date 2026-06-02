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
    <div className="mb-3 grid gap-3 rounded-[1.5rem] border border-zinc-200 bg-white/80 px-4 py-4 text-sm shadow-sm sm:grid-cols-2 xl:grid-cols-3">
      <ProgressMetric
        label="已生成"
        value={generatedStyleCount > 0 ? `${generatedStyleCount} 个款式` : "未生成"}
      />
      <ProgressMetric
        label="已批准"
        value={selectedStyleCount > 0 ? `${selectedStyleCount} 个已选` : "无"}
      />
      <ProgressMetric
        label="SHEIN 任务"
        value={createdTaskCount > 0 ? `${createdTaskCount} 个已创建` : "待创建"}
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
