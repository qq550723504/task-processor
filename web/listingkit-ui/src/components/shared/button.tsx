import { ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/utils/cn";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  tone?: "primary" | "secondary" | "ghost" | "danger";
};

const toneClasses: Record<NonNullable<ButtonProps["tone"]>, string> = {
  primary:
    "bg-zinc-950 text-white hover:bg-zinc-800 disabled:bg-zinc-300 disabled:text-zinc-500",
  secondary:
    "bg-white text-zinc-900 ring-1 ring-zinc-200 hover:bg-zinc-100 disabled:text-zinc-400",
  ghost:
    "bg-transparent text-zinc-700 hover:bg-zinc-100 disabled:text-zinc-400",
  danger:
    "bg-rose-600 text-white hover:bg-rose-500 disabled:bg-rose-200 disabled:text-rose-400",
};

export function Button({
  className,
  tone = "primary",
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex h-10 items-center justify-center rounded-xl px-4 text-sm font-medium transition disabled:cursor-not-allowed",
        toneClasses[tone],
        className,
      )}
      {...props}
    />
  );
}
