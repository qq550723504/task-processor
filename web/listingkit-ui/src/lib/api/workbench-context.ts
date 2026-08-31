import { z } from "zod";

export const WORKBENCH_CONTEXT_QUERY_KEY = ["workbench-context"] as const;

const safeIdSchema = z
  .string()
  .trim()
  .min(1)
  .max(128)
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/);

const safeDisplayNameSchema = z
  .string()
  .trim()
  .min(1)
  .max(256)
  .refine((value) => !/[\u0000-\u001f\u007f]/.test(value));

const roleSchema = safeIdSchema;

const organizationSchema = z
  .object({
    id: safeIdSchema,
    name: safeDisplayNameSchema,
    roles: z.array(roleSchema).max(128),
  })
  .strict();

export const workbenchContextSchema = z
  .object({
    user: z.object({ id: safeIdSchema }).strict(),
    homeOrganizationId: safeIdSchema,
    effectiveOrganizationId: safeIdSchema.nullable(),
    selectionRequired: z.boolean(),
    organizations: z.array(organizationSchema).max(1000),
  })
  .strict()
  .superRefine((context, refinement) => {
    const organizationIds = new Set<string>();
    for (const organization of context.organizations) {
      if (organizationIds.has(organization.id)) {
        refinement.addIssue({
          code: "custom",
          message: "Organization IDs must be unique",
          path: ["organizations"],
        });
      }
      organizationIds.add(organization.id);
    }
    if (
      context.effectiveOrganizationId !== null &&
      !organizationIds.has(context.effectiveOrganizationId)
    ) {
      refinement.addIssue({
        code: "custom",
        message: "Effective Organization must be accessible",
        path: ["effectiveOrganizationId"],
      });
    }
    const organizationCount = context.organizations.length;
    const isResolverReachableState =
      (organizationCount === 0 &&
        context.effectiveOrganizationId === null &&
        !context.selectionRequired) ||
      (organizationCount === 1 &&
        context.effectiveOrganizationId === context.organizations[0]?.id &&
        !context.selectionRequired) ||
      (organizationCount > 1 &&
        ((context.effectiveOrganizationId === null &&
          context.selectionRequired) ||
          (context.effectiveOrganizationId !== null &&
            !context.selectionRequired)));
    if (!isResolverReachableState) {
      refinement.addIssue({
        code: "custom",
        message:
          "Organization count, effective Organization, and selection state must match the resolver",
        path: ["effectiveOrganizationId"],
      });
    }
  });

const fieldErrorSchema = z
  .object({
    field: z.string().trim().min(1).max(128),
    code: z.string().trim().min(1).max(128),
  })
  .strict();

export const workbenchErrorEnvelopeSchema = z
  .object({
    code: z
      .string()
      .trim()
      .min(1)
      .max(128)
      .regex(/^[A-Z][A-Z0-9_]*$/),
    message: z.string().max(2048),
    requestId: z
      .string()
      .max(256)
      .refine((value) => !/[\u0000-\u001f\u007f]/.test(value)),
    fieldErrors: z.array(fieldErrorSchema).max(128),
  })
  .strict();

export type WorkbenchContext = z.infer<typeof workbenchContextSchema>;
export type WorkbenchOrganization = WorkbenchContext["organizations"][number];
export type WorkbenchErrorEnvelope = z.infer<
  typeof workbenchErrorEnvelopeSchema
>;

export function parseWorkbenchContextPayload(payload: unknown) {
  return workbenchContextSchema.safeParse(payload);
}

export function parseWorkbenchErrorEnvelopePayload(payload: unknown) {
  return workbenchErrorEnvelopeSchema.safeParse(payload);
}

export class WorkbenchContextError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly requestId: string,
    public readonly fieldErrors: unknown[],
  ) {
    super(`Workbench request failed (${code})`);
    this.name = "WorkbenchContextError";
  }
}

export function fetchWorkbenchContext(): Promise<WorkbenchContext> {
  return requestWorkbenchContext("/api/workbench/context", {
    method: "GET",
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });
}

export function switchEffectiveOrganization(
  organizationId: string,
): Promise<WorkbenchContext> {
  return requestWorkbenchContext(
    "/api/workbench/context/effective-organization",
    {
      method: "PUT",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ organizationId }),
    },
  );
}

async function requestWorkbenchContext(
  input: string,
  init: RequestInit,
): Promise<WorkbenchContext> {
  let response: Response;
  try {
    response = await fetch(input, init);
  } catch {
    throw new WorkbenchContextError(0, "WORKBENCH_REQUEST_FAILED", "", []);
  }
  const payload = await response.json().catch(() => null);

  if (!response.ok) {
    const parsedError = parseWorkbenchErrorEnvelopePayload(payload);
    if (!parsedError.success) {
      throw invalidResponseError(response.status);
    }
    throw new WorkbenchContextError(
      response.status,
      parsedError.data.code,
      parsedError.data.requestId,
      parsedError.data.fieldErrors,
    );
  }

  const parsedContext = parseWorkbenchContextPayload(payload);
  if (!parsedContext.success) {
    throw invalidResponseError(response.status);
  }
  return parsedContext.data;
}

function invalidResponseError(status: number) {
  return new WorkbenchContextError(
    status,
    "INVALID_WORKBENCH_RESPONSE",
    "",
    [],
  );
}
