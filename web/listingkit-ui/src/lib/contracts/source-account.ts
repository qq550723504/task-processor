import { z } from "zod";

export const SOURCE_ACCOUNT_REQUEST_BODY_MAX_BYTES = 8 * 1024;
export const SOURCE_ACCOUNT_QUERY_MAX_BYTES = 1024;
export const SOURCE_ACCOUNT_CURSOR_MAX_BYTES = 512;
export const SOURCE_ACCOUNT_RESPONSE_MAX_BYTES = 128 * 1024;

const MAX_SIGNED_BIGINT = BigInt("9223372036854775807");
const BASE64URL_ALPHABET =
  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

export const sourceAccountContextIdSchema = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);

export const sourceAccountOperationIdSchema = z
  .string()
  .refine(isCanonicalSourceAccountOperationId);

export const sourceAccountIdSchema = z
  .string()
  .refine(isCanonicalSourceAccountId);

export const sourceAccountDisplayNameSchema = z
  .string()
  .refine(isValidSourceAccountDisplayName);

export const sourceAccountCreateRequestSchema = z
  .object({
    displayName: sourceAccountDisplayNameSchema,
    platform: z.literal("1688"),
  })
  .strict();

const sourceAccountVersionSchema = z
  .string()
  .regex(/^[1-9][0-9]*$/)
  .refine(isPositiveSignedInt64);

const sourceAccountTimestampSchema = z
  .string()
  .refine(isUTCRFC3339Timestamp);

export const sourceAccountSchema = z
  .object({
    id: sourceAccountIdSchema,
    platform: z.literal("1688"),
    displayName: sourceAccountDisplayNameSchema,
    managementStatus: z.enum(["enabled", "disabled"]),
    connectionStatus: z.literal("pending_connection"),
    version: sourceAccountVersionSchema,
    createdAt: sourceAccountTimestampSchema,
    updatedAt: sourceAccountTimestampSchema,
  })
  .strict();

export const sourceAccountMutationResponseSchema = z
  .object({
    schemaVersion: z.literal(1),
    account: sourceAccountSchema,
    replayed: z.boolean(),
  })
  .strict();

export const sourceAccountDetailResponseSchema = z
  .object({ schemaVersion: z.literal(1), account: sourceAccountSchema })
  .strict();

export const sourceAccountPageResponseSchema = z
  .object({
    schemaVersion: z.literal(1),
    items: z.array(sourceAccountSchema).max(100),
    nextCursor: z
      .string()
      .refine(
        (value) =>
          sourceAccountUTF8Length(value) <= SOURCE_ACCOUNT_CURSOR_MAX_BYTES &&
          isCanonicalSourceAccountCursor(value),
      )
      .nullable(),
  })
  .strict();

export function isCanonicalSourceAccountOperationId(value: string) {
  return (
    value !== "00000000-0000-0000-0000-000000000000" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
      value,
    )
  );
}

export function isCanonicalSourceAccountId(value: string) {
  return (
    value !== "00000000-0000-0000-0000-000000000000" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
      value,
    )
  );
}

export function isCanonicalSourceAccountCursor(value: string) {
  if (!/^[A-Za-z0-9_-]+$/.test(value) || value.length % 4 === 1) return false;
  const trailingValue = BASE64URL_ALPHABET.indexOf(value.at(-1)!);
  if (value.length % 4 === 2) return (trailingValue & 15) === 0;
  if (value.length % 4 === 3) return (trailingValue & 3) === 0;
  return true;
}

export function isStrongSourceAccountETag(value: string) {
  const matched = /^"([1-9][0-9]*)"$/.exec(value);
  return matched ? isPositiveSignedInt64(matched[1]!) : false;
}

export function sourceAccountUTF8Length(value: string) {
  return new TextEncoder().encode(value).byteLength;
}

function isValidSourceAccountDisplayName(value: string) {
  return (
    value === value.trim() &&
    value.length > 0 &&
    sourceAccountUTF8Length(value) <= 120 &&
    !/[\p{Cc}\p{Cs}]/u.test(value)
  );
}

function isPositiveSignedInt64(value: string) {
  try {
    return /^[1-9][0-9]*$/.test(value) && BigInt(value) <= MAX_SIGNED_BIGINT;
  } catch {
    return false;
  }
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
  if (year < 1 || month < 1 || month > 12 || hour > 23 || minute > 59 || second > 59) {
    return false;
  }
  return day >= 1 && day <= new Date(Date.UTC(year, month, 0)).getUTCDate();
}
