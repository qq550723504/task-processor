"use client";

import Link from "next/link";

type SdsRouteHeaderLink = {
  href: string;
  label: string;
};

export function SdsRouteHeader({
  title,
  description,
  eyebrow,
  links,
}: {
  title: string;
  description: string;
  eyebrow: string;
  links: SdsRouteHeaderLink[];
}) {
  return (
    <div className="space-y-2 rounded-lg border border-zinc-200 bg-white px-5 py-5 shadow-sm">
      {links.length > 0 ? (
        <div className="flex flex-wrap gap-3 text-sm font-medium text-zinc-500">
          {links.map((link) => (
            <Link
              className="transition hover:text-zinc-900"
              href={link.href}
              key={link.href}
            >
              {link.label}
            </Link>
          ))}
        </div>
      ) : null}
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {eyebrow}
      </p>
      <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
        {title}
      </h1>
      <p className="max-w-3xl text-sm leading-7 text-zinc-600">{description}</p>
    </div>
  );
}
