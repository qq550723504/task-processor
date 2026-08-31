import { afterEach, describe, expect, it, vi } from "vitest";

import {
  fetchWorkbenchContext,
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
