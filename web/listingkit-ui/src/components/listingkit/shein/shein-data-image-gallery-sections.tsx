export function ImageRoleStatus({
  count,
  fallbackReady = false,
  label,
  required = false,
}: {
  count: number;
  fallbackReady?: boolean;
  label: string;
  required?: boolean;
}) {
  const ready = count > 0 || fallbackReady;
  return (
    <div
      className={[
        "rounded-xl border px-3 py-2",
        ready
          ? "border-emerald-200 bg-emerald-50 text-emerald-800"
          : required
            ? "border-amber-200 bg-amber-50 text-amber-800"
            : "border-zinc-200 bg-white text-zinc-600",
      ].join(" ")}
    >
      <div className="font-semibold">{label}</div>
      <div className="mt-1 text-[11px]">
        {count > 0
          ? `${count} 张已设置`
          : fallbackReady
            ? "默认使用首图"
            : required
              ? "需要设置"
              : "未设置"}
      </div>
    </div>
  );
}
