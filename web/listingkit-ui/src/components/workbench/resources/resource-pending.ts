"use client";
import { useMemo, useSyncExternalStore } from "react";
import { z } from "zod";
import {
  memberGrantInput,
  memberResourceID,
  memberTransferInput,
  type MemberResourceScope,
} from "@/lib/api/member-resources";

export const transferPendingSchema = z
  .object({
    memberId: memberResourceID,
    input: memberTransferInput,
    key: z.uuid(),
  })
  .strict();
export const grantPendingSchema = z
  .object({ storeId: z.uuid(), input: memberGrantInput, key: z.uuid() })
  .strict();
export const servicePendingSchema = z
  .object({
    action: z.enum(["activate", "renew", "reactivate"]),
    periods: z.number().int().min(1).max(12),
    version: z.number().int().positive(),
    key: z.uuid(),
  })
  .strict()
  .refine((v) => v.action !== "activate" || v.periods === 1);

const event = "resource-pending";
function subscribe(listener: () => void) {
  window.addEventListener(event, listener);
  window.addEventListener("storage", listener);
  return () => {
    window.removeEventListener(event, listener);
    window.removeEventListener("storage", listener);
  };
}
const serverSnapshot = () => "initializing";
function decode<T>(value: string | null, schema: z.ZodType<T>, maxLength: number): T | null {
  if (value === null) return null;
  if (value.length > maxLength) throw new Error("Pending command exceeds bound");
  return schema.parse(JSON.parse(value));
}

// Reuses the member-command UI's sessionStorage / useSyncExternalStore pattern.
// Only the original bounded command is stored, under its user, organization and
// business object. It is neither a balance authority nor a new recovery owner.
export function useResourcePending<T extends { key: string }>(
  scope: MemberResourceScope,
  object: readonly string[],
  schema: z.ZodType<T>,
  options: { storage?: "local" | "session"; maxLength?: number } = {},
) {
  const maxLength = options.maxLength ?? 8192;
  const storage = () => options.storage === "local" ? localStorage : sessionStorage;
  const storageKey = `resource.pending:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId, ...object])}`;
  const raw = useSyncExternalStore(
    subscribe,
    () => {
      try {
        return storage().getItem(storageKey);
      } catch {
        return "unreadable";
      }
    },
    serverSnapshot,
  );
  const local = useMemo(() => {
    if (raw === "initializing")
      return { command: null, ready: false, error: false };
    try {
      return { command: decode(raw, schema, maxLength), ready: true, error: false };
    } catch {
      return { command: null, ready: false, error: true };
    }
  }, [raw, schema, maxLength]);
  function read() {
    return decode(storage().getItem(storageKey), schema, maxLength);
  }
  function persist(command: T) {
    const validated = schema.parse(command);
    const original = read();
    const encoded = JSON.stringify(validated);
    if (encoded.length > maxLength) throw new Error("Pending command exceeds bound");
    if (original && JSON.stringify(original) !== encoded)
      throw new Error("Original operation is still pending");
    storage().setItem(storageKey, encoded);
    if (storage().getItem(storageKey) !== encoded)
      throw new Error("Pending command could not be saved");
    window.dispatchEvent(new Event(event));
  }
  function clear(command: T) {
    const original = read();
    if (
      !original ||
      original.key !== command.key ||
      JSON.stringify(original) !== JSON.stringify(schema.parse(command))
    )
      throw new Error("Original operation changed");
    storage().removeItem(storageKey);
    if (storage().getItem(storageKey) !== null)
      throw new Error("Pending command could not be cleared");
    window.dispatchEvent(new Event(event));
  }
  return { ...local, persist, clear, read, storageKey };
}
