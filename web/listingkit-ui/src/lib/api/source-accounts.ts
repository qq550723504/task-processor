import { z } from "zod";

import {
  parseWorkbenchErrorEnvelopePayload,
  type WorkbenchErrorEnvelope,
} from "@/lib/api/workbench-context";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import {
  isCanonicalSourceAccountCursor,
  isStrongSourceAccountETag,
  SOURCE_ACCOUNT_CURSOR_MAX_BYTES,
  SOURCE_ACCOUNT_QUERY_MAX_BYTES,
  SOURCE_ACCOUNT_RESPONSE_MAX_BYTES,
  sourceAccountContextIdSchema,
  sourceAccountCreateRequestSchema,
  sourceAccountDetailResponseSchema,
  sourceAccountIdSchema,
  sourceAccountMutationResponseSchema,
  sourceAccountOperationIdSchema,
  sourceAccountPageResponseSchema,
  sourceAccountSchema,
  sourceAccountUTF8Length,
} from "@/lib/contracts/source-account";

const sourceAccountErrorStatuses: Readonly<Record<string, readonly number[]>> = {
  INVALID_REQUEST: [400],
  AUTHENTICATION_REQUIRED: [401],
  ORGANIZATION_SELECTION_REQUIRED: [409],
  ORGANIZATION_ACCESS_DENIED: [403],
  ORGANIZATION_ACCESS_REVOKED: [403],
  ORGANIZATION_SUSPENDED: [403],
  PERMISSION_DENIED: [403],
  SOURCE_ACCOUNT_NOT_FOUND: [404],
  IDEMPOTENCY_CONFLICT: [409],
  VERSION_CONFLICT: [409],
  INVALID_TRANSITION: [409],
  RESOURCE_LIMIT_REACHED: [409],
  ORGANIZATION_CONTEXT_CHANGED: [409],
  IDENTITY_CONTEXT_CHANGED: [409],
  INPUT_TOO_LARGE: [413],
  DEPENDENCY_UNAVAILABLE: [502, 503],
  INVALID_UPSTREAM_RESPONSE: [502],
  OUTCOME_UNKNOWN: [503],
  DEADLINE_EXCEEDED: [504],
};

export type SourceAccount = z.infer<typeof sourceAccountSchema>;
export type SourceAccountMutationResult = z.infer<
  typeof sourceAccountMutationResponseSchema
> & {
  etag: string;
};
export type SourceAccountDetailResult = z.infer<
  typeof sourceAccountDetailResponseSchema
> & {
  etag: string;
};
export type SourceAccountPage = z.infer<typeof sourceAccountPageResponseSchema>;
export type SourceAccountMutationOutcome = "not_sent" | "rejected" | "unknown";

type CommonRequest = {
  expectedOrganizationId: string;
  signal?: AbortSignal;
};

type MutationIntent = CommonRequest & {
  idempotencyKey: string;
  expectedActorSubject: string;
};

export type CreateSourceAccountRequest = MutationIntent & {
  displayName: string;
  platform: "1688";
};

export type ListSourceAccountsRequest = CommonRequest & {
  limit?: number;
  cursor?: string;
};

export type GetSourceAccountRequest = CommonRequest & {
  sourceAccountId: string;
};

export type ChangeSourceAccountStatusRequest = MutationIntent & {
  sourceAccountId: string;
  ifMatch: string;
};

export class SourceAccountAPIError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly requestId: string,
    public readonly payload: WorkbenchErrorEnvelope | null,
    public readonly outcome: SourceAccountMutationOutcome,
  ) {
    super(message);
    this.name = "SourceAccountAPIError";
  }
}

export async function createSourceAccount(
  request: CreateSourceAccountRequest,
): Promise<SourceAccountMutationResult> {
  rejectUnknownKeys(request, [
    "displayName",
    "platform",
    "idempotencyKey",
    "expectedOrganizationId",
    "expectedActorSubject",
    "signal",
  ]);
  const common = parseMutationIntent(request);
  const body = parseOrReject(sourceAccountCreateRequestSchema, {
    displayName: request.displayName,
    platform: request.platform,
  });
  return requestSourceAccountMutation(
    "/api/workbench/source-accounts",
    {
      method: "POST",
      body: JSON.stringify(body),
      headers: mutationHeaders(common, { "Content-Type": "application/json" }),
      signal: request.signal,
    },
    "create",
  );
}

export async function listSourceAccounts(
  request: ListSourceAccountsRequest,
): Promise<SourceAccountPage> {
  rejectUnknownKeys(request, ["limit", "cursor", "expectedOrganizationId", "signal"]);
  const organizationId = parseOrReject(
    sourceAccountContextIdSchema,
    request.expectedOrganizationId,
  );
  rejectAlreadyAborted(request.signal);
  const limit = request.limit ?? 20;
  if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw invalidInput();
  if (
    request.cursor !== undefined &&
    (!isCanonicalSourceAccountCursor(request.cursor) ||
      sourceAccountUTF8Length(request.cursor) > SOURCE_ACCOUNT_CURSOR_MAX_BYTES)
  ) {
    throw invalidInput();
  }
  const query = new URLSearchParams({ limit: String(limit) });
  if (request.cursor !== undefined) query.set("cursor", request.cursor);
  if (sourceAccountUTF8Length(query.toString()) > SOURCE_ACCOUNT_QUERY_MAX_BYTES) {
    throw invalidInput();
  }
  return (
    await requestSourceAccountRead(
      `/api/workbench/source-accounts?${query.toString()}`,
      { method: "GET", headers: readHeaders(organizationId), signal: request.signal },
      sourceAccountPageResponseSchema,
    )
  ).data;
}

export async function getSourceAccount(
  request: GetSourceAccountRequest,
): Promise<SourceAccountDetailResult> {
  rejectUnknownKeys(request, ["sourceAccountId", "expectedOrganizationId", "signal"]);
  const organizationId = parseOrReject(
    sourceAccountContextIdSchema,
    request.expectedOrganizationId,
  );
  const sourceAccountId = parseOrReject(sourceAccountIdSchema, request.sourceAccountId);
  rejectAlreadyAborted(request.signal);
  const result = await requestSourceAccountRead(
    `/api/workbench/source-accounts/${sourceAccountId}`,
    { method: "GET", headers: readHeaders(organizationId), signal: request.signal },
    sourceAccountDetailResponseSchema,
    sourceAccountId,
  );
  const etag = requireMatchingETag(result.data.account.version, result.etag);
  return { ...result.data, etag };
}

export function enableSourceAccount(
  request: ChangeSourceAccountStatusRequest,
): Promise<SourceAccountMutationResult> {
  return changeSourceAccountStatus("enable", request);
}

export function disableSourceAccount(
  request: ChangeSourceAccountStatusRequest,
): Promise<SourceAccountMutationResult> {
  return changeSourceAccountStatus("disable", request);
}

async function changeSourceAccountStatus(
  action: "enable" | "disable",
  request: ChangeSourceAccountStatusRequest,
) {
  rejectUnknownKeys(request, [
    "sourceAccountId",
    "ifMatch",
    "idempotencyKey",
    "expectedOrganizationId",
    "expectedActorSubject",
    "signal",
  ]);
  const common = parseMutationIntent(request);
  const sourceAccountId = parseOrReject(sourceAccountIdSchema, request.sourceAccountId);
  const ifMatch = parseOrReject(
    z.string().refine((value) => isStrongSourceAccountETag(value)),
    request.ifMatch,
  );
  return requestSourceAccountMutation(
    `/api/workbench/source-accounts/${sourceAccountId}/${action}`,
    {
      method: "POST",
      headers: mutationHeaders(common, { "If-Match": ifMatch }),
      signal: request.signal,
    },
    "lifecycle",
    sourceAccountId,
  );
}

function parseMutationIntent(request: MutationIntent) {
  rejectAlreadyAborted(request.signal);
  return {
    expectedOrganizationId: parseOrReject(
      sourceAccountContextIdSchema,
      request.expectedOrganizationId,
    ),
    expectedActorSubject: parseOrReject(
      sourceAccountContextIdSchema,
      request.expectedActorSubject,
    ),
    idempotencyKey: parseOrReject(
      sourceAccountOperationIdSchema,
      request.idempotencyKey,
    ),
  };
}

function readHeaders(expectedOrganizationId: string) {
  return new Headers({
    Accept: "application/json",
    "X-Expected-Organization-ID": expectedOrganizationId,
  });
}

function mutationHeaders(
  intent: ReturnType<typeof parseMutationIntent>,
  additional: Record<string, string>,
) {
  const headers = readHeaders(intent.expectedOrganizationId);
  headers.set("Idempotency-Key", intent.idempotencyKey);
  headers.set("X-Expected-User-ID", intent.expectedActorSubject);
  for (const [name, value] of Object.entries(additional)) headers.set(name, value);
  return headers;
}

async function requestSourceAccountMutation(
  input: string,
  init: RequestInit,
  kind: "create" | "lifecycle",
  expectedAccountId?: string,
): Promise<SourceAccountMutationResult> {
  let response: Response;
  try {
    response = await fetch(input, {
      ...init,
      credentials: "same-origin",
      cache: "no-store",
    });
  } catch {
    throw new SourceAccountAPIError(
      0,
      "OUTCOME_UNKNOWN",
      "Source Account mutation outcome is unknown",
      "",
      null,
      "unknown",
    );
  }
  const payload = await readPayload(response, true, init.signal ?? undefined);
  if (!response.ok) throw responseError(response.status, payload, true);
  const parsed = sourceAccountMutationResponseSchema.safeParse(payload);
  const validStatus =
    kind === "create"
      ? (response.status === 201 && parsed.success && !parsed.data.replayed) ||
        (response.status === 200 && parsed.success && parsed.data.replayed)
      : response.status === 200 && parsed.success;
  if (!parsed.success || !validStatus) throw invalidResponse(response.status, true);
  if (expectedAccountId && parsed.data.account.id !== expectedAccountId) {
    throw invalidResponse(response.status, true);
  }
  const etag = requireMatchingETag(
    parsed.data.account.version,
    response.headers.get("ETag"),
    true,
    response.status,
  );
  return { ...parsed.data, etag };
}

async function requestSourceAccountRead<T>(
  input: string,
  init: RequestInit,
  schema: z.ZodType<T>,
  expectedAccountId?: string,
): Promise<{ data: T; etag: string | null }> {
  let response: Response;
  try {
    response = await fetch(input, {
      ...init,
      credentials: "same-origin",
      cache: "no-store",
    });
  } catch {
    throw new SourceAccountAPIError(
      0,
      "DEPENDENCY_UNAVAILABLE",
      "Source Account request failed",
      "",
      null,
      "rejected",
    );
  }
  let payload: unknown;
  try {
    payload = await readBoundedStrictJSON(
      response,
      SOURCE_ACCOUNT_RESPONSE_MAX_BYTES,
      init.signal ?? undefined,
    );
  } catch {
    throw invalidResponse(response.status, false);
  }
  if (!response.ok) throw responseError(response.status, payload, false);
  if (response.status !== 200) throw invalidResponse(response.status, false);
  const parsed = schema.safeParse(payload);
  if (!parsed.success) throw invalidResponse(response.status, false);
  if (
    expectedAccountId &&
    typeof parsed.data === "object" &&
    parsed.data !== null &&
    "account" in parsed.data &&
    (parsed.data as { account: SourceAccount }).account.id !== expectedAccountId
  ) {
    throw invalidResponse(response.status, false);
  }
  return { data: parsed.data, etag: response.headers.get("ETag") };
}

async function readPayload(
  response: Response,
  mutation: boolean,
  signal?: AbortSignal,
) {
  try {
    return await readBoundedStrictJSON(
      response,
      SOURCE_ACCOUNT_RESPONSE_MAX_BYTES,
      signal,
    );
  } catch {
    throw invalidResponse(response.status, mutation);
  }
}

function responseError(status: number, payload: unknown, mutation: boolean) {
  const parsed = parseWorkbenchErrorEnvelopePayload(payload);
  if (
    !parsed.success ||
    !sourceAccountErrorStatuses[parsed.data.code]?.includes(status)
  ) {
    return invalidResponse(status, mutation);
  }
  const outcome =
    mutation &&
    (status === 502 ||
      parsed.data.code === "OUTCOME_UNKNOWN" ||
      parsed.data.code === "DEADLINE_EXCEEDED")
      ? "unknown"
      : "rejected";
  return new SourceAccountAPIError(
    status,
    parsed.data.code,
    parsed.data.message,
    parsed.data.requestId,
    parsed.data,
    outcome,
  );
}

function invalidResponse(status: number, mutation: boolean) {
  return new SourceAccountAPIError(
    status,
    "INVALID_UPSTREAM_RESPONSE",
    "Source Account response is invalid",
    "",
    null,
    mutation ? "unknown" : "rejected",
  );
}

function invalidInput() {
  return new SourceAccountAPIError(
    0,
    "INVALID_REQUEST",
    "Source Account request is invalid",
    "",
    null,
    "not_sent",
  );
}

function parseOrReject<T>(schema: z.ZodType<T>, input: unknown): T {
  const parsed = schema.safeParse(input);
  if (!parsed.success) throw invalidInput();
  return parsed.data;
}

function rejectUnknownKeys(value: object, allowedKeys: readonly string[]) {
  const allowed = new Set(allowedKeys);
  if (Object.keys(value).some((key) => !allowed.has(key))) throw invalidInput();
}

function rejectAlreadyAborted(signal?: AbortSignal) {
  if (signal?.aborted) throw invalidInput();
}

function requireMatchingETag(
  version: string,
  etag: string | null,
  mutation = false,
  status = 200,
) {
  if (!etag || etag !== `"${version}"` || !isStrongSourceAccountETag(etag)) {
    throw invalidResponse(status, mutation);
  }
  return etag;
}
