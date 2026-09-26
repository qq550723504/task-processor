import { z } from "zod";
import { invitationInput, memberId, MemberOperation, removeInput, roleInput } from "@/lib/api/members";

const commandSchema = z.discriminatedUnion("kind", [
  z.object({ key: z.string().uuid(), kind: z.literal("invite"), input: invitationInput }).strict(),
  z.object({ key: z.string().uuid(), kind: z.literal("role"), target: memberId, input: roleInput }).strict(),
  z.object({ key: z.string().uuid(), kind: z.literal("remove"), target: memberId, input: removeInput }).strict(),
]);
export type Pending = z.infer<typeof commandSchema>;
const collection = z.array(commandSchema).max(100).refine(items => new Set(items.map(item => item.key)).size === items.length);

export function readPending(raw: string | null): Pending[] {
  if (raw === null) return [];
  if (raw.length > 409600) throw new Error("Pending storage exceeds bound");
  const value: unknown = JSON.parse(raw);
  // Preserve the active single command from the current member UI in place.
  return collection.parse(Array.isArray(value) ? value : [value]);
}

export function savePending(storage: Storage, key: string, command: Pending | string): void {
  const current = readPending(storage.getItem(key));
  let next: Pending[];
  if (typeof command === "string") next = current.filter(item => item.key !== command);
  else {
    const existing = current.find(item => item.key === command.key);
    if (existing && JSON.stringify(existing) !== JSON.stringify(command)) throw new Error("Operation key conflict");
    next = existing ? current : [...current, command];
  }
  const raw = JSON.stringify(collection.parse(next));
  if (next.length) storage.setItem(key, raw); else storage.removeItem(key);
  // Do not send a command unless its recovery record is actually readable.
  if (next.length ? storage.getItem(key) !== raw : storage.getItem(key) !== null) throw new Error("Pending storage failed");
}

export const terminalReceipt = (receipt?: MemberOperation) => !!receipt && ["acknowledged", "rejected"].includes(receipt.status);

// The existing protocol only advances. A late list/GET cannot downgrade known
// ACK/identity evidence or a terminal receipt, even across a query refetch.
export function retainAdvancedReceipt(a: MemberOperation | undefined, b: MemberOperation | undefined): MemberOperation | undefined {
  if (!a) return b;
  if (!b || a.id !== b.id || a.userId !== b.userId || a.organizationId !== b.organizationId) return a;
  const rank = (op: MemberOperation) => terminalReceipt(op) ? 10 : (op.step === "create_authorization" ? 4 : 0) + (op.step === "create_user" && op.userEvidence ? 2 : op.status === "unknown" ? 1 : 0);
  return rank(a) > rank(b) ? a : b;
}
