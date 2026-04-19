import { cn } from "@/lib/utils/cn";

export function Badge({
  children,
  tone = "neutral",
}: {
  children: string;
  tone?: "neutral" | "success" | "warning" | "danger";
}) {
  const toneClass = {
    neutral: "bg-zinc-100 text-zinc-700",
    success: "bg-emerald-100 text-emerald-700",
    warning: "bg-amber-100 text-amber-700",
    danger: "bg-rose-100 text-rose-700",
  }[tone];

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-medium",
        toneClass,
      )}
    >
      {children}
    </span>
  );
}
