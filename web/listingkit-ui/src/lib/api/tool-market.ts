import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
export type ToolScope = { userId: string; organizationId: string };
export class ToolMarketError extends Error {
  constructor(
    public code: string,
    public status = 500,
  ) {
    super(code);
  }
}
export async function toolRequest<T>(
  scope: ToolScope,
  path: string,
  schema: z.ZodType<T>,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("X-Expected-User-ID", scope.userId);
  headers.set("X-Expected-Organization-ID", scope.organizationId);
  headers.set("Accept", "application/json");
  const write = !!init.method && init.method !== "GET";
  try {
    const r = await fetch("/api/tool-market/" + path, {
      ...init,
      headers,
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
    });
    const payload = await readBoundedStrictJSON(
      r,
      256 << 10,
      init.signal ?? undefined,
    );
    if (!r.ok) {
      const error = z
        .strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) })
        .safeParse(payload);
      throw new ToolMarketError(
        error.success
          ? error.data.code
          : write
            ? "OUTCOME_UNKNOWN"
            : "DEPENDENCY_UNAVAILABLE",
        r.status,
      );
    }
    const parsed = schema.safeParse(payload);
    if (r.status !== 200 || !parsed.success) throw new Error();
    return parsed.data;
  } catch (e) {
    if (e instanceof ToolMarketError) throw e;
    throw new ToolMarketError(
      write ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE",
    );
  }
}
export async function downloadTool(scope: ToolScope) {
  const response = await fetch("/api/tool-market/plugin", {
    headers: {
      "X-Expected-User-ID": scope.userId,
      "X-Expected-Organization-ID": scope.organizationId,
    },
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
    signal: AbortSignal.timeout(15000),
  });
  if (
    response.status !== 200 ||
    response.headers.get("Content-Type") !== "application/zip"
  )
    throw new ToolMarketError("DEPENDENCY_UNAVAILABLE");
  const { readToolPackage } = await import("./tool-package");
  const bytes = await readToolPackage(response, AbortSignal.timeout(15000));
  const url = URL.createObjectURL(
    new Blob([bytes], { type: "application/zip" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = "shuomi-1688-capture.zip";
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
