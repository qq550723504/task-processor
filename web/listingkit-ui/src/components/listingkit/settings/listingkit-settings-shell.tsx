"use client";

import type { ReactNode } from "react";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";

export type ListingKitSettingsSectionDefinition = {
  id: string;
  label: string;
  summary: string;
  badges?: string[];
  render: () => ReactNode;
};

type ListingKitSettingsShellProps = {
  eyebrow: string;
  title: string;
  description: string;
  backgroundClassName: string;
  sections: ListingKitSettingsSectionDefinition[];
};

export function ListingKitSettingsShell({
  eyebrow,
  title,
  description,
  backgroundClassName,
  sections,
}: ListingKitSettingsShellProps) {
  return (
    <ListingKitPageShell backgroundClassName={backgroundClassName}>
      <section className="rounded-[2rem] border border-white/70 bg-white/78 p-5 shadow-[0_24px_90px_rgba(39,39,42,0.10)] sm:p-6">
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
          {eyebrow}
        </p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">{title}</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">{description}</p>
        {sections.length > 0 ? (
          <div className="mt-5 grid gap-2 sm:flex sm:flex-wrap">
            {sections.map((section) => (
              <a
                key={section.id}
                className="rounded-full border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 transition hover:border-zinc-300 hover:text-zinc-950"
                href={`#${section.id}`}
              >
                {section.label}
              </a>
            ))}
          </div>
        ) : null}
      </section>

      {sections.map((section) => (
        <div key={section.id} data-testid={`settings-section-${section.id}`}>
          <div className="mb-2 flex flex-col items-start gap-2 px-1 sm:flex-row sm:flex-wrap sm:items-center">
            <span className="text-xs font-medium text-zinc-600">{section.summary}</span>
            {section.badges?.map((badge) => (
              <span
                key={badge}
                className="rounded-full border border-zinc-200 bg-white px-2 py-0.5 text-[11px] font-medium text-zinc-500"
              >
                {badge}
              </span>
            ))}
          </div>
          {section.render()}
        </div>
      ))}
    </ListingKitPageShell>
  );
}
