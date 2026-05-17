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
      className="scroll-mt-24 rounded-[1.5rem] border border-white/70 bg-white/86 p-4 shadow-sm"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
            {eyebrow}
          </div>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">{title}</h2>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-zinc-600">{description}</p>
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>

      <div className="mt-4">{children}</div>
    </section>
  );
}
