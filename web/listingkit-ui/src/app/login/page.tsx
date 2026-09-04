import { redirect } from "next/navigation";

import {
  loginMethodForEntry,
  normalizeReturnTo,
  resolveLoginEntry,
} from "@/lib/server/login-entry";

type LoginPageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const resolvedSearchParams = (await searchParams) ?? {};
  const rawReturnTo = resolvedSearchParams.returnTo;
  const returnTo = normalizeReturnTo(
    Array.isArray(rawReturnTo) ? rawReturnTo[0] ?? null : rawReturnTo ?? null,
  );
  const rawMethod = resolvedSearchParams.method;
  const entry = resolveLoginEntry(
    rawMethod === undefined
      ? []
      : Array.isArray(rawMethod)
        ? rawMethod
        : [rawMethod],
  );
  const method = loginMethodForEntry(entry);
  const loginUrl = new URLSearchParams({ returnTo });
  if (method) {
    loginUrl.set("method", method);
  }

  redirect(`/api/zitadel-auth/login?${loginUrl.toString()}`);
}
