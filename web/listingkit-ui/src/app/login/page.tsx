import { redirect } from "next/navigation";

import { normalizeReturnTo } from "@/lib/server/zitadel-auth";

type LoginPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const resolvedSearchParams = (await searchParams) ?? {};
  const rawReturnTo = resolvedSearchParams.returnTo;
  const returnTo = normalizeReturnTo(
    Array.isArray(rawReturnTo) ? rawReturnTo[0] ?? null : rawReturnTo ?? null,
  );

  redirect(`/api/zitadel-auth/login?returnTo=${encodeURIComponent(returnTo)}`);
}
