import { z } from "zod";

export const WORKBENCH_CONTEXT_QUERY_KEY = ["workbench-context"] as const;

const nonblankIdSchema = z.string().trim().min(1);

const organizationSchema = z
  .object({
    id: nonblankIdSchema,
    name: z.string(),
    roles: z.array(z.string()),
  })
  .strict();

const workbenchContextSchema = z
  .object({
    user: z.object({ id: nonblankIdSchema }).strict(),
    homeOrganizationId: nonblankIdSchema,
    effectiveOrganizationId: nonblankIdSchema.nullable(),
    selectionRequired: z.boolean(),
    organizations: z.array(organizationSchema),
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
  });

const errorEnvelopeSchema = z
  .object({
    code: z.string().trim().min(1),
    message: z.string(),
    requestId: z.string(),
    fieldErrors: z.array(z.unknown()),
  })
  .strict();

export type WorkbenchContext = z.infer<typeof workbenchContextSchema>;
export type WorkbenchOrganization = WorkbenchContext["organizations"][number];

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
    throw new WorkbenchContextError(
      0,
      "WORKBENCH_REQUEST_FAILED",
      "",
      [],
    );
  }
  const payload = await response.json().catch(() => null);

  if (!response.ok) {
    const parsedError = errorEnvelopeSchema.safeParse(payload);
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

  const parsedContext = workbenchContextSchema.safeParse(payload);
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
