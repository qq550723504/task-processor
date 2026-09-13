import { z } from "zod";

import {
  InvalidStrictJSONResponseError,
  readBoundedStrictJSON,
} from "./strict-json-response";

const utcTimestamp = z.string().regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/).refine((value) => Number.isFinite(Date.parse(value)));
const safeReference = z.string().min(1).max(200).regex(/^[A-Za-z0-9._:-]+$/);
const code = z.string().max(200);

const admissionSchema = z.object({
  intentID: safeReference,
  resumeSecret: z.string().min(43).max(256).regex(/^[A-Za-z0-9_-]+$/),
  createExpiresAt: utcTimestamp,
  completionExpiresAt: utcTimestamp,
}).strict().superRefine((value, context) => {
  if (Date.parse(value.completionExpiresAt) <= Date.parse(value.createExpiresAt)) {
    context.addIssue({ code: "custom", message: "Invalid referral expiry order" });
  }
});
const resumeSchema = z.object({ status: z.literal("created") }).strict();
const projectionSchema = z.object({
  code,
  codeAvailability: z.enum(["available", "not_created"]),
  count: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  generatedAt: utcTimestamp,
  earnings: z.object({ availability: z.literal("unavailable"), amount: z.null() }).strict(),
}).strict().superRefine((value, context) => {
  if ((value.codeAvailability === "available") !== (value.code.length > 0)) {
    context.addIssue({ code: "custom", message: "Invalid referral code availability" });
  }
});
const codeSchema = z.object({ code: code.min(1) }).strict();
const receiptSchema = z.object({
  status: z.literal("complete"),
  intentID: safeReference,
  boundAt: utcTimestamp,
}).strict();
const errorSchema = z.object({ code: z.string().min(1).max(80) }).passthrough();

export type ReferralAdmission = z.infer<typeof admissionSchema>;
export type ReferralProjection = z.infer<typeof projectionSchema>;
export type ReferralReceipt = z.infer<typeof receiptSchema>;
export type ReferralRegistrationInput = {
  code: string;
  email: string;
  givenName: string;
  familyName: string;
};

export class ReferralRequestError extends Error {
  constructor(public readonly status: number, public readonly code: string) {
    super("Referral request could not be completed");
  }
}

export function startReferralRegistration(input: ReferralRegistrationInput, idempotencyKey: string, signal?: AbortSignal) {
  return requestReferral("/api/referral-registration", "POST", admissionSchema, {
    body: JSON.stringify(input),
    headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
    signal,
  });
}

export function resumeReferralRegistration(intentID: string, resumeSecret: string, signal?: AbortSignal) {
  return requestReferral("/api/referral-registration/resume", "POST", resumeSchema, {
    body: JSON.stringify({ intentID, resumeSecret }),
    headers: { "Content-Type": "application/json" },
    signal,
  });
}

export function getAccountReferrals(expectedUserId: string, signal?: AbortSignal) {
  return requestReferral("/api/account/referrals", "GET", projectionSchema, {
    headers: { "X-Expected-User-ID": expectedUserId },
    signal,
  });
}

export function createAccountReferralCode(expectedUserId: string, signal?: AbortSignal) {
  return requestReferral("/api/account/referrals", "POST", codeSchema, {
    headers: { "X-Expected-User-ID": expectedUserId },
    signal,
  });
}

export function completeReferralRegistration(expectedUserId: string, signal?: AbortSignal) {
  return requestReferral("/api/account/referrals/complete", "POST", receiptSchema, {
    headers: { "X-Expected-User-ID": expectedUserId },
    signal,
  });
}

async function requestReferral<T>(
  url: string,
  method: "GET" | "POST",
  schema: z.ZodType<T>,
  options: { body?: string; headers: Record<string, string>; signal?: AbortSignal },
): Promise<T> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) abort();
  const timer = setTimeout(abort, 15_000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch(url, {
      method,
      headers: options.headers,
      ...(options.body === undefined ? {} : { body: options.body }),
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      signal: controller.signal,
    });
    const payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal);
    controller.signal.throwIfAborted();
    if (!response.ok) {
      const error = errorSchema.safeParse(payload);
      throw new ReferralRequestError(response.status, error.success ? error.data.code : "INVALID_UPSTREAM_RESPONSE");
    }
    const parsed = schema.safeParse(payload);
    if (!parsed.success) throw new ReferralRequestError(502, "INVALID_UPSTREAM_RESPONSE");
    return parsed.data;
  } catch (error) {
    if (controller.signal.aborted) throw new ReferralRequestError(504, "DEADLINE_EXCEEDED");
    if (error instanceof ReferralRequestError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new ReferralRequestError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new ReferralRequestError(502, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    options.signal?.removeEventListener("abort", abort);
  }
}
