"use client";

import type { ReactNode } from "react";

import { cn } from "@/lib/utils/cn";

export function ListingKitPageShell({
  backgroundClassName,
  children,
  className,
  contentClassName,
  overlayClassName,
}: {
  backgroundClassName?: string;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  overlayClassName?: string;
}) {
  return (
    <div
      className={cn(
        "relative flex flex-1 flex-col py-6",
        backgroundClassName,
        className,
      )}
    >
      {overlayClassName ? (
        <div className={cn("pointer-events-none absolute inset-0", overlayClassName)} />
      ) : null}
      <div className={cn("relative flex w-full flex-1 flex-col gap-6", contentClassName)}>
        {children}
      </div>
    </div>
  );
}
