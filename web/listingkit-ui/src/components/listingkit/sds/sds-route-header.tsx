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
    <div className="w-full space-y-2 rounded-lg border border-border bg-card px-5 py-5 shadow-sm">
      {links.length > 0 ? (
        <div className="flex flex-wrap gap-3 text-sm font-medium text-muted-foreground">
          {links.map((link) => (
            <Link
              className="transition hover:text-foreground"
              href={link.href}
              key={link.href}
            >
              {link.label}
            </Link>
          ))}
        </div>
      ) : null}
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
        {eyebrow}
      </p>
      <h1 className="text-2xl font-semibold tracking-tight text-foreground">
        {title}
      </h1>
      <p className="max-w-3xl text-sm leading-7 text-muted-foreground">{description}</p>
    </div>
  );
}
