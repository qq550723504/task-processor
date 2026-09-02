import { afterEach, describe, expect, it, vi } from "vitest";

import {
  fetchWorkbenchContext,
  parseWorkbenchContextPayload,
  switchEffectiveOrganization,
  WorkbenchContextError,
} from "@/lib/api/workbench-context";

const VALID_CONTEXT = {
  user: { id: "user-1" },
  homeOrganizationId: "org-a",
  effectiveOrganizationId: "org-b",
  selectionRequired: false,
  organizations: [
    { id: "org-a", name: "硕米科技", roles: ["listingkit_admin"] },
    { id: "org-b", name: "星海贸易", roles: ["listingkit_viewer"] },
  ],
};

describe("workbench context API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("fetches only the same-origin workbench context endpoint and validates its contract", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(VALID_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);

    await expect(fetchWorkbenchContext()).resolves.toEqual(VALID_CONTEXT);
    expect(fetchMock).toHaveBeenCalledWith("/api/workbench/context", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
  });

  it("forwards a query abort signal to context reads", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(VALID_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();

    await expect(fetchWorkbenchContext(controller.signal)).resolves.toEqual(
      VALID_CONTEXT,
    );
    expect(fetchMock).toHaveBeenCalledWith("/api/workbench/context", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
      signal: controller.signal,
    });
  });

  it.each([
    [
      "zero accessible Organizations",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: null,
        selectionRequired: false,
        organizations: [],
      },
    ],
    [
      "one accessible Organization",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: "org-a",
        selectionRequired: false,
        organizations: [VALID_CONTEXT.organizations[0]],
      },
    ],
    [
      "multiple Organizations without an effective selection",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: null,
        selectionRequired: true,
      },
    ],
    ["multiple Organizations with a listed effective selection", VALID_CONTEXT],
  ])("accepts the resolver-reachable state for %s", (_name, payload) => {
    // Mutation caught: accepting selectionRequired based only on a null
    // effective ID allows states the effective Organization resolver cannot emit.
    expect(parseWorkbenchContextPayload(payload).success).toBe(true);
  });

  it.each([
    [
      "zero Organizations with selection required",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: null,
        selectionRequired: true,
        organizations: [],
      },
    ],
    [
      "one Organization without an effective selection",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: null,
        selectionRequired: false,
        organizations: [VALID_CONTEXT.organizations[0]],
      },
    ],
    [
      "one Organization with selection required",
      {
        ...VALID_CONTEXT,
        effectiveOrganizationId: "org-a",
        selectionRequired: true,
        organizations: [VALID_CONTEXT.organizations[0]],
      },
    ],
    [
      "multiple Organizations without a selection requirement",
      { ...VALID_CONTEXT, effectiveOrganizationId: null },
    ],
    [
      "multiple Organizations with a selection requirement despite an effective Organization",
      { ...VALID_CONTEXT, selectionRequired: true },
    ],
  ])("rejects an unreachable resolver state for %s", (_name, payload) => {
    expect(parseWorkbenchContextPayload(payload).success).toBe(false);
  });

  it.each([
    ["a blank user ID", { ...VALID_CONTEXT, user: { id: "  " } }],
    [
      "duplicate Organization IDs",
      {
        ...VALID_CONTEXT,
        organizations: [
          VALID_CONTEXT.organizations[0],
          { ...VALID_CONTEXT.organizations[1], id: "org-a" },
        ],
      },
    ],
    [
      "roles that are not a string array",
      {
        ...VALID_CONTEXT,
        organizations: [
          { ...VALID_CONTEXT.organizations[0], roles: ["admin", 7] },
          VALID_CONTEXT.organizations[1],
        ],
      },
    ],
    [
      "an effective Organization outside the accessible list",
      { ...VALID_CONTEXT, effectiveOrganizationId: "org-revoked" },
    ],
    [
      "a blank Organization ID",
      {
        ...VALID_CONTEXT,
        organizations: [
          { ...VALID_CONTEXT.organizations[0], id: " " },
          VALID_CONTEXT.organizations[1],
        ],
      },
    ],
    [
      "an Organization display name without a non-whitespace character",
      {
        ...VALID_CONTEXT,
        organizations: [
          { ...VALID_CONTEXT.organizations[0], name: "   " },
          VALID_CONTEXT.organizations[1],
        ],
      },
    ],
  ])("fails closed for %s", async (_name, payload) => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(payload)),
    );

    const error = await fetchWorkbenchContext().catch(
      (reason: unknown) => reason,
    );

    expect(error).toBeInstanceOf(WorkbenchContextError);
    expect(error).toMatchObject({
      status: 200,
      code: "INVALID_WORKBENCH_RESPONSE",
      requestId: "",
      fieldErrors: [],
    });
  });

  it("throws a typed stable-code error without depending on backend message text", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json(
          {
            code: "ORGANIZATION_ACCESS_REVOKED",
            message: "this text may change",
            requestId: "req-42",
            fieldErrors: [{ field: "organizationId", code: "revoked" }],
          },
          { status: 403 },
        ),
      ),
    );

    const error = await fetchWorkbenchContext().catch(
      (reason: unknown) => reason,
    );

    expect(error).toBeInstanceOf(WorkbenchContextError);
    expect(error).toMatchObject({
      status: 403,
      code: "ORGANIZATION_ACCESS_REVOKED",
      requestId: "req-42",
      fieldErrors: [{ field: "organizationId", code: "revoked" }],
    });
  });

  it("uses a deterministic client code for malformed error JSON without exposing raw text", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          new Response("private upstream diagnostics", { status: 502 }),
        ),
    );

    const error = await fetchWorkbenchContext().catch(
      (reason: unknown) => reason,
    );

    expect(error).toBeInstanceOf(WorkbenchContextError);
    expect(error).toMatchObject({
      status: 502,
      code: "INVALID_WORKBENCH_RESPONSE",
      requestId: "",
      fieldErrors: [],
    });
    expect(String(error)).not.toContain("private upstream diagnostics");
  });

  it("normalizes transport failures into the typed client error contract", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockRejectedValue(new Error("socket secret")),
    );

    const error = await fetchWorkbenchContext().catch(
      (reason: unknown) => reason,
    );

    expect(error).toBeInstanceOf(WorkbenchContextError);
    expect(error).toMatchObject({
      status: 0,
      code: "WORKBENCH_REQUEST_FAILED",
      requestId: "",
      fieldErrors: [],
    });
    expect(String(error)).not.toContain("socket secret");
  });

  it("switches by Organization ID with JSON and returns the validated next context", async () => {
    const nextContext = {
      ...VALID_CONTEXT,
      effectiveOrganizationId: "org-a",
    };
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(nextContext));
    vi.stubGlobal("fetch", fetchMock);

    await expect(switchEffectiveOrganization("org-a")).resolves.toEqual(
      nextContext,
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/workbench/context/effective-organization",
      {
        method: "PUT",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: '{"organizationId":"org-a"}',
      },
    );
  });
});
