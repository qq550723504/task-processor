"use client";

import type { ReactNode } from "react";

type ListingKitSettingsSectionProps = {
  id?: string;
  eyebrow: string;
  title: string;
  description: string;
  actions?: ReactNode;
  children: ReactNode;
};

export function ListingKitSettingsSection({
  id,
  eyebrow,
  title,
  description,
  actions,
  children,
}: ListingKitSettingsSectionProps) {
  return (
    <section
      id={id}
      className="scroll-mt-24 rounded-[1.5rem] border border-border/70 bg-card/95 p-4 shadow-sm"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-start sm:justify-between">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
            {eyebrow}
          </div>
          <h2 className="mt-1 text-lg font-semibold text-foreground">{title}</h2>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">{description}</p>
        </div>
        {actions ? (
          <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap sm:items-center">
            {actions}
          </div>
        ) : null}
      </div>

      <div className="mt-4">{children}</div>
    </section>
  );
}
