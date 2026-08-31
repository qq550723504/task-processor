import { z } from "zod";

import {
  parseWorkbenchErrorEnvelopePayload,
  type WorkbenchErrorEnvelope,
} from "@/lib/api/workbench-context";
import {
  workbenchStoreCreateSchema,
  workbenchStoreListFiltersSchema,
  workbenchStoreUpdateSchema,
  type WorkbenchStoreCreateInput,
  type WorkbenchStoreListFilters,
  type WorkbenchStoreUpdateInput,
} from "@/lib/validation/workbench-store";

const canonicalUUIDSchema = z
  .string()
  .refine(
    (value) =>
      value !== "00000000-0000-0000-0000-000000000000" &&
      /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
        value,
      ),
  );
export const EXPECTED_ORGANIZATION_ID_HEADER = "X-Expected-Organization-ID";
const expectedOrganizationIdSchema = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const positiveSafeIntegerSchema = z
  .number()
  .int()
  .min(1)
  .max(Number.MAX_SAFE_INTEGER);
const nonnegativeSafeIntegerSchema = z
  .number()
  .int()
  .min(0)
  .max(Number.MAX_SAFE_INTEGER);

const normalizedPublicString = (minimum: number, maximum: number) =>
  z.string().refine(
    (value) =>
      value === value.trim() &&
      !/\p{Cc}/u.test(value) &&
      Array.from(value).length >= minimum &&
      Array.from(value).length <= maximum,
  );

const utcRFC3339Schema = z.string().refine(isUTCRFC3339Timestamp);

const workbenchStoreSchema = z
  .object({
    id: canonicalUUIDSchema,
    name: normalizedPublicString(1, 120),
    platform: z.literal("shein"),
    region: normalizedPublicString(1, 64),
    externalStoreId: normalizedPublicString(0, 128),
    lifecycleStatus: z.enum([
      "provisioning",
      "active",
      "disabled",
      "deleting",
    ]),
    connectionStatus: z.enum([
      "disconnected",
      "connected",
      "expired",
      "unavailable",
    ]),
    version: positiveSafeIntegerSchema,
    createdAt: utcRFC3339Schema,
    updatedAt: utcRFC3339Schema,
  })
  .strict();

const workbenchStoreListSchema = z
  .object({
    items: z.array(workbenchStoreSchema).max(100),
    quota: z
      .object({
        used: nonnegativeSafeIntegerSchema,
        reserved: nonnegativeSafeIntegerSchema,
        limit: positiveSafeIntegerSchema.nullable(),
        allowed: z.boolean(),
        reason: z.enum(["", "subscription_required", "store_limit_reached"]),
      })
      .strict()
      .superRefine((quota, refinement) => {
        const consumed = quota.used + quota.reserved;
        const valid =
          Number.isSafeInteger(consumed) && quota.limit === null
            ? !quota.allowed && quota.reason === "subscription_required"
            : quota.limit !== null &&
              Number.isSafeInteger(consumed) &&
              quota.allowed === consumed < quota.limit &&
              quota.reason ===
                (consumed < quota.limit ? "" : "store_limit_reached");
        if (!valid) {
          refinement.addIssue({
            code: "custom",
            message: "Quota state is inconsistent",
          });
        }
      }),
    pagination: z
      .object({
        page: positiveSafeIntegerSchema,
        pageSize: positiveSafeIntegerSchema.max(100),
        total: nonnegativeSafeIntegerSchema,
      })
      .strict(),
  })
  .strict();

const workbenchStoreListResponseSchema = workbenchStoreListSchema.superRefine(
  (response, refinement) => {
    if (
      response.items.length > response.pagination.pageSize ||
      response.items.length > response.pagination.total
    ) {
      refinement.addIssue({
        code: "custom",
        message: "Pagination state is inconsistent",
      });
    }
  },
);

const workbenchStoreDeleteSchema = z
  .object({
    id: canonicalUUIDSchema,
    deleted: z.literal(true),
    version: positiveSafeIntegerSchema,
  })
  .strict();

export type WorkbenchStore = z.infer<typeof workbenchStoreSchema>;
export type WorkbenchStoreList = z.infer<typeof workbenchStoreListResponseSchema>;
export type WorkbenchStoreDeleteResult = z.infer<
  typeof workbenchStoreDeleteSchema
>;
export type WorkbenchStoreConnectionStatus =
  WorkbenchStore["connectionStatus"];
export type WorkbenchStoreQuotaReason = WorkbenchStoreList["quota"]["reason"];
export type WorkbenchStoreFieldError =
  WorkbenchErrorEnvelope["fieldErrors"][number];
export const workbenchStoreErrorCodes = [
  "INVALID_REQUEST",
  "AUTHENTICATION_REQUIRED",
  "ORGANIZATION_SELECTION_REQUIRED",
  "ORGANIZATION_ACCESS_DENIED",
  "ORGANIZATION_ACCESS_REVOKED",
  "ORGANIZATION_SUSPENDED",
  "PERMISSION_DENIED",
  "DEPENDENCY_UNAVAILABLE",
  "STORE_NOT_FOUND",
  "STORE_ALREADY_EXISTS",
  "STORE_VERSION_CONFLICT",
  "STORE_INVALID_STATE",
  "SUBSCRIPTION_REQUIRED",
  "STORE_LIMIT_REACHED",
  "ORGANIZATION_CONTEXT_CHANGED",
] as const;
export type WorkbenchStoreKnownErrorCode =
  (typeof workbenchStoreErrorCodes)[number];
export type WorkbenchStoreErrorCode = WorkbenchStoreKnownErrorCode | (string & {});
export type {
  WorkbenchStoreCreateInput,
  WorkbenchStoreListFilters,
  WorkbenchStoreUpdateInput,
} from "@/lib/validation/workbench-store";

export class WorkbenchAPIError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: WorkbenchStoreErrorCode,
    message: string,
    public readonly requestId: string,
    public readonly fieldErrors: WorkbenchErrorEnvelope["fieldErrors"],
  ) {
    super(message);
    this.name = "WorkbenchAPIError";
  }
}

export async function listWorkbenchStores(
  filters: WorkbenchStoreListFilters,
  expectedOrganizationId: string,
): Promise<WorkbenchStoreList> {
  const parsedFilters = parseInput(workbenchStoreListFiltersSchema, filters);
  const search = new URLSearchParams();
  search.set("page", String(parsedFilters.page));
  search.set("pageSize", String(parsedFilters.pageSize));
  if (parsedFilters.platform !== undefined) {
    search.set("platform", parsedFilters.platform);
  }
  if (parsedFilters.status !== undefined) {
    search.set("status", parsedFilters.status);
  }
  return requestWorkbenchStores(
    `/api/workbench/stores?${search.toString()}`,
    { method: "GET" },
    workbenchStoreListResponseSchema,
    200,
    expectedOrganizationId,
  );
}

export async function getWorkbenchStore(
  storeId: string,
  expectedOrganizationId: string,
): Promise<WorkbenchStore> {
  return requestWorkbenchStores(
    storePath(storeId),
    { method: "GET" },
    workbenchStoreSchema,
    200,
    expectedOrganizationId,
  );
}

export async function createWorkbenchStore(
  input: WorkbenchStoreCreateInput,
  idempotencyKey: string,
  expectedOrganizationId: string,
): Promise<WorkbenchStore> {
  const parsedInput = parseInput(workbenchStoreCreateSchema, input);
  const parsedKey = parseInput(canonicalUUIDSchema, idempotencyKey);
  return requestWorkbenchStores(
    "/api/workbench/stores",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": parsedKey,
      },
      body: JSON.stringify(parsedInput),
    },
    workbenchStoreSchema,
    201,
    expectedOrganizationId,
  );
}

export async function updateWorkbenchStore(
  storeId: string,
  input: WorkbenchStoreUpdateInput,
  version: number,
  expectedOrganizationId: string,
): Promise<WorkbenchStore> {
  const parsedInput = parseInput(workbenchStoreUpdateSchema, input);
  return requestWorkbenchStores(
    storePath(storeId),
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        "If-Match": ifMatch(version),
      },
      body: JSON.stringify(parsedInput),
    },
    workbenchStoreSchema,
    200,
    expectedOrganizationId,
  );
}

export async function enableWorkbenchStore(
  storeId: string,
  version: number,
  expectedOrganizationId: string,
): Promise<WorkbenchStore> {
  return storeAction(storeId, "enable", version, expectedOrganizationId);
}

export async function disableWorkbenchStore(
  storeId: string,
  version: number,
  expectedOrganizationId: string,
): Promise<WorkbenchStore> {
  return storeAction(storeId, "disable", version, expectedOrganizationId);
}

export async function deleteWorkbenchStore(
  storeId: string,
  version: number,
  idempotencyKey: string,
  expectedOrganizationId: string,
): Promise<WorkbenchStoreDeleteResult> {
  const parsedKey = parseInput(canonicalUUIDSchema, idempotencyKey);
  return requestWorkbenchStores(
    storePath(storeId),
    {
      method: "DELETE",
      headers: {
        "If-Match": ifMatch(version),
        "Idempotency-Key": parsedKey,
      },
    },
    workbenchStoreDeleteSchema,
    200,
    expectedOrganizationId,
  );
}

function storeAction(
  storeId: string,
  action: "enable" | "disable",
  version: number,
  expectedOrganizationId: string,
) {
  return requestWorkbenchStores(
    `${storePath(storeId)}/${action}`,
    {
      method: "POST",
      headers: { "If-Match": ifMatch(version) },
    },
    workbenchStoreSchema,
    200,
    expectedOrganizationId,
  );
}

function storePath(storeId: string) {
  const parsedId = parseInput(canonicalUUIDSchema, storeId);
  return `/api/workbench/stores/${parsedId}`;
}

function ifMatch(version: number) {
  return `"${parseInput(positiveSafeIntegerSchema, version)}"`;
}

function parseInput<T>(schema: z.ZodType<T>, value: unknown): T {
  const parsed = schema.safeParse(value);
  if (!parsed.success) throw invalidRequestError();
  return parsed.data;
}

async function requestWorkbenchStores<T>(
  input: string,
  init: RequestInit,
  schema: z.ZodType<T>,
  expectedStatus: number,
  expectedOrganizationId: string,
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set(
    EXPECTED_ORGANIZATION_ID_HEADER,
    parseInput(expectedOrganizationIdSchema, expectedOrganizationId),
  );
  headers.set("Accept", "application/json");
  let response: Response;
  try {
    response = await fetch(input, {
      ...init,
      credentials: "same-origin",
      headers,
    });
  } catch {
    throw new WorkbenchAPIError(
      0,
      "WORKBENCH_REQUEST_FAILED",
      "Workbench request failed",
      "",
      [],
    );
  }

  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    const parsedError = parseWorkbenchErrorEnvelopePayload(payload);
    if (!parsedError.success) throw invalidResponseError(response.status);
    throw new WorkbenchAPIError(
      response.status,
      parsedError.data.code,
      parsedError.data.message,
      parsedError.data.requestId,
      parsedError.data.fieldErrors,
    );
  }

  if (response.status !== expectedStatus) {
    throw invalidResponseError(response.status);
  }

  const parsed = schema.safeParse(payload);
  if (!parsed.success) throw invalidResponseError(response.status);
  return parsed.data;
}

function invalidRequestError() {
  return new WorkbenchAPIError(
    0,
    "INVALID_REQUEST",
    "Workbench Store request is invalid",
    "",
    [],
  );
}

function invalidResponseError(status: number) {
  return new WorkbenchAPIError(
    status,
    "INVALID_WORKBENCH_RESPONSE",
    "Workbench response is invalid",
    "",
    [],
  );
}

function isUTCRFC3339Timestamp(value: string) {
  const match =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?Z$/.exec(
      value,
    );
  if (!match) return false;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6]);
  if (
    year < 1 ||
    month < 1 ||
    month > 12 ||
    hour > 23 ||
    minute > 59 ||
    second > 59
  ) {
    return false;
  }
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();
  return day >= 1 && day <= daysInMonth;
}
