import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
export type ConfigurationScope = { userId: string; organizationId: string };
export class ConfigurationError extends Error {
  constructor(
    public code: string,
    public status = 500,
  ) {
    super(code);
  }
}
export async function configurationRequest<T>(
  scope: ConfigurationScope,
  path: string,
  schema: z.ZodType<T>,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("X-Expected-User-ID", scope.userId);
  headers.set("X-Expected-Organization-ID", scope.organizationId);
  headers.set("Accept", "application/json");
  let response: Response;
  try {
    response = await fetch("/api/workbench/agents/" + path, {
      ...init,
      headers,
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
    });
  } catch {
    throw new ConfigurationError(
      init.method && init.method !== "GET"
        ? "OUTCOME_UNKNOWN"
        : "DEPENDENCY_UNAVAILABLE",
    );
  }
  const payload = await readBoundedStrictJSON(
    response,
    128 * 1024,
    init.signal ?? undefined,
  ).catch(() => {
    throw new ConfigurationError(
      init.method && init.method !== "GET"
        ? "OUTCOME_UNKNOWN"
        : "INVALID_UPSTREAM_RESPONSE",
    );
  });
  if (!response.ok) {
    const error = z
      .object({ code: z.string().regex(/^[A-Z_]{1,80}$/) })
      .safeParse(payload);
    throw new ConfigurationError(
      error.success ? error.data.code : "DEPENDENCY_UNAVAILABLE",
      response.status,
    );
  }
  const parsed = schema.safeParse(payload);
  if (response.status !== 200 || !parsed.success)
    throw new ConfigurationError(
      init.method && init.method !== "GET"
        ? "OUTCOME_UNKNOWN"
        : "INVALID_UPSTREAM_RESPONSE",
    );
  return parsed.data;
}
