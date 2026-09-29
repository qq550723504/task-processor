import { z } from "zod";
import { commercialResourceBalancesSchema } from "./commercial-billing";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

export const COMMERCIAL_MAX_BYTES = 64 * 1024;
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const timestamp = z
  .string()
  .max(40)
  .regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/)
  .refine((v) => Number.isFinite(Date.parse(v)));
const unavailable = z
  .object({ state: z.literal("unavailable"), value: z.null() })
  .strict();
const count = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const services = z
  .object({
    records: count,
    active: count,
    expired: count,
    expiring_soon: count,
  })
  .strict()
  .refine(
    (v) => v.active + v.expired <= v.records && v.expiring_soon <= v.active,
  );
const overview = z
  .object({
    schema_version: z.literal("unified-base-prepaid-v1"),
    organization_id: id,
    observed_at: timestamp,
    base_plan: z
      .object({
        code: z.literal("base_payg"),
        name: z.literal("基础方案"),
        subscription_required: z.literal(false),
        store_period_days: z.literal(30),
        ai_limit_period: z.literal("utc_calendar_month"),
        resource_expiry: z.literal("none"),
      })
      .strict(),
    resources: z.discriminatedUnion("state", [
      unavailable,
      z
        .object({
          state: z.literal("available"),
          value: commercialResourceBalancesSchema,
        })
        .strict(),
    ]),
    store_services: z.discriminatedUnion("state", [
      unavailable,
      z.object({ state: z.literal("available"), value: services }).strict(),
    ]),
  })
  .strict()
  .refine(
    (v) =>
      v.resources.state !== "available" ||
      v.resources.value.organization_id === v.organization_id,
  );
export type CommercialOverview = z.infer<typeof overview>;
export function parseCommercialOverview(
  payload: unknown,
): CommercialOverview | null {
  const result = overview.safeParse(payload);
  return result.success ? result.data : null;
}

const errorStatuses: Readonly<Record<string, number>> = {
  INVALID_REQUEST: 400,
  AUTHENTICATION_REQUIRED: 401,
  PERMISSION_DENIED: 403,
  ORGANIZATION_ACCESS_DENIED: 403,
  ORGANIZATION_ACCESS_REVOKED: 403,
  ORGANIZATION_SUSPENDED: 403,
  ORGANIZATION_SELECTION_REQUIRED: 409,
  ORGANIZATION_CONTEXT_CHANGED: 409,
  DEPENDENCY_UNAVAILABLE: 503,
  DEADLINE_EXCEEDED: 504,
  INVALID_UPSTREAM_RESPONSE: 502,
};
export function parseCommercialReadFailure(payload: unknown, status: number) {
  const parsed = parseWorkbenchErrorEnvelopePayload(payload);
  return parsed.success && errorStatuses[parsed.data.code] === status
    ? {
        ...parsed.data,
        message: "Commercial read could not be completed",
        fieldErrors: [],
      }
    : null;
}
export class CommercialReadError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly requestId = "",
  ) {
    super("Commercial read could not be completed");
  }
}

export async function getCommercialOverview(
  expectedOrganizationId: string,
  signal?: AbortSignal,
): Promise<CommercialOverview> {
  if (!id.safeParse(expectedOrganizationId).success)
    throw new CommercialReadError(400, "INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  const timeout = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch("/api/workbench/commercial/overview", {
      method: "GET",
      headers: {
        Accept: "application/json",
        "X-Expected-Organization-ID": expectedOrganizationId,
      },
      cache: "no-store",
      redirect: "manual",
      signal: controller.signal,
    });
    const payload = await readBoundedStrictJSON(
      response,
      response.status === 200 ? COMMERCIAL_MAX_BYTES : 8192,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status === 200) {
      const result = parseCommercialOverview(payload);
      if (result?.organization_id === expectedOrganizationId) return result;
    }
    const failure = parseCommercialReadFailure(payload, response.status);
    throw failure
      ? new CommercialReadError(
          response.status,
          failure.code,
          failure.requestId,
        )
      : new CommercialReadError(502, "INVALID_UPSTREAM_RESPONSE");
  } catch (error) {
    if (controller.signal.aborted)
      throw new CommercialReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof CommercialReadError) throw error;
    throw new CommercialReadError(502, "INVALID_UPSTREAM_RESPONSE");
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}
