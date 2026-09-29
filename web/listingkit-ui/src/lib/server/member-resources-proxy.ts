import { NextResponse } from "next/server";
import { z } from "zod";
import {
  accountFailure,
  readAccountRequestBody,
  serviceOrigin,
} from "./account-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import {
  memberResourceID,
  memberTransferInput,
  memberGrantInput,
  memberQuoteInput,
  parseMemberResourceResult,
  memberResourceErrorCode,
} from "@/lib/api/member-resources";

export async function proxyMemberResources(
  request: Request,
  token: string,
  userId: string,
): Promise<Response> {
  const write = request.method !== "GET";
  if (!token || !memberResourceID.safeParse(userId).success)
    return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId)
    return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  if (write && !hasTrustedSameOriginWrite(request))
    return accountFailure(403, "PERMISSION_DENIED");
  const selections = (request.headers.get("cookie") ?? "")
    .split(";")
    .map((v) => v.trim())
    .filter((v) => v.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (selections.length !== 1)
    return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  let org: string;
  try {
    org = decodeURIComponent(
      selections[0].slice(WORKBENCH_COOKIE_NAME.length + 1),
    );
  } catch {
    return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  }
  if (
    !memberResourceID.safeParse(org).success ||
    request.headers.get("X-Expected-Organization-ID") !== org
  )
    return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const url = new URL(request.url);
  const base = "/api/account/member-resources";
  if (url.pathname !== base && !url.pathname.startsWith(`${base}/`))
    return accountFailure(400, "INVALID_REQUEST");
  const path = url.pathname.slice(base.length);
  const parts = path.split("/").filter(Boolean);
  const directory =
    request.method === "GET" && ["", "/data-prices"].includes(path);
  const quote = request.method === "POST" && path === "/data-quotes";
  const member =
    parts[0] === "members" && memberResourceID.safeParse(parts[1]).success;
  const transfer =
    member &&
    parts.length === 3 &&
    parts[2] === "transfers" &&
    request.method === "POST";
  const stores =
    member &&
    parts.length === 3 &&
    parts[2] === "stores" &&
    request.method === "GET";
  const grant =
    member &&
    parts.length === 5 &&
    parts[2] === "stores" &&
    z.uuid().safeParse(parts[3]).success &&
    parts[4] === "grant" &&
    ["GET", "PUT"].includes(request.method);
  if (
    !(directory || quote || transfer || stores || grant) ||
    request.url.endsWith("?") ||
    (!stores && url.search) ||
    (!write &&
      (request.body ||
        request.headers.has("transfer-encoding") ||
        (request.headers.has("content-length") &&
          request.headers.get("content-length") !== "0")))
  )
    return accountFailure(400, "INVALID_REQUEST");
  if (stores)
    for (const [key, value] of url.searchParams)
      if (
        !["page", "pageSize"].includes(key) ||
        url.searchParams.getAll(key).length !== 1 ||
        !/^[1-9][0-9]{0,6}$/.test(value) ||
        Number(value) > (key === "pageSize" ? 100 : 1000000)
      )
        return accountFailure(400, "INVALID_REQUEST");
  const origin = serviceOrigin();
  if (!origin) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  let forwarded = false;
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
      "X-Requested-Organization-ID": org,
    });
    let body: string | undefined;
    let key: string | undefined;
    if (write) {
      if (
        request.headers
          .get("content-type")
          ?.split(";")[0]
          .trim()
          .toLowerCase() !== "application/json"
      )
        return accountFailure(400, "INVALID_REQUEST");
      const raw = await readAccountRequestBody(
        request,
        8192,
        controller.signal,
      );
      const input = await readBoundedStrictJSON(
        new Response(raw, { headers: { "Content-Type": "application/json" } }),
        8192,
        controller.signal,
      );
      body = JSON.stringify(
        (quote
          ? memberQuoteInput
          : transfer
            ? memberTransferInput
            : memberGrantInput
        ).parse(input),
      );
      headers.set("Content-Type", "application/json");
      if (!quote) {
        key = request.headers.get("Idempotency-Key") ?? undefined;
        if (
          !memberResourceID.safeParse(key).success ||
          (grant && !z.uuid().safeParse(key).success)
        )
          return accountFailure(400, "INVALID_REQUEST");
        headers.set("Idempotency-Key", key!);
      }
    }
    controller.signal.throwIfAborted();
    forwarded = true;
    const response = await fetch(
      `${origin}/api/v1/account/organization/resources/member-resources${path}${url.search}`,
      {
        method: request.method,
        headers,
        ...(body ? { body } : {}),
        redirect: "manual",
        cache: "no-store",
        signal: controller.signal,
      },
    );
    const payload = await readBoundedStrictJSON(
      response,
      response.status === 200 ? 256 * 1024 : 8192,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      if (write && response.status >= 500)
        return accountFailure(response.status, "RESULT_UNVERIFIED", "unknown");
      return accountFailure(
        response.status,
        memberResourceErrorCode(response.status, payload),
      );
    }
    const result = parseMemberResourceResult(
      payload,
      path,
      request.method,
      org,
      key,
    );
    return NextResponse.json(result, {
      headers: {
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return accountFailure(
      controller.signal.aborted ? 504 : forwarded ? 502 : 400,
      write && forwarded
        ? "RESULT_UNVERIFIED"
        : forwarded
          ? "INVALID_UPSTREAM_RESPONSE"
          : "INVALID_REQUEST",
      write && forwarded ? "unknown" : undefined,
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}
