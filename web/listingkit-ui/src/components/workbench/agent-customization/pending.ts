import { z } from "zod";
import { customId, customVersion, customInput, customUpdate } from "@/lib/api/agent-customization";
export type CustomIntent = {
    key: string;
    body: string;
    path: string;
    version?: string;
};
export const customizationPending = z.strictObject({ key: customId, path: z.string().max(64), version: customVersion.optional(), digest: z.string().regex(/^[a-f0-9]{64}$/), body: z.string().max(131072).optional() }).refine(v => v.path === "" && !v.version || /^\/[a-f0-9-]{36}\/progress$/.test(v.path) && customId.safeParse(v.path.split("/")[1]).success && !!v.version);
export type CustomPending = z.infer<typeof customizationPending>;
// Large files remain in feature-local memory across navigation. A small session
// marker survives reload and blocks a new command if its bytes are unavailable.
// Small commands can be recovered in full without storing large contact files.
const payloads = new Map<string, {
    digest: string;
    body: string;
}>();
const memoryBound = 18 * 1024 * 1024;
const memoryKey = (storageKey: string, key: string) => JSON.stringify([storageKey, key]);
export async function freezeCustomization(storageKey: string, i: CustomIntent): Promise<CustomPending> {
    const bytes = new TextEncoder().encode(i.body);
    if (bytes.length > 9 * 1024 * 1024)
        throw new Error("Command exceeds bound");
    const parsed = JSON.parse(i.body);
    if (!(i.path === "" ? customInput : customUpdate).safeParse(parsed).success)
        throw new Error("Invalid command");
    const digest = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)), v => v.toString(16).padStart(2, "0")).join("");
    const record = customizationPending.parse({ ...i, body: i.body.length <= 131072 ? i.body : undefined, digest });
    const key = memoryKey(storageKey, i.key), original = payloads.get(key);
    if (original && original.digest !== digest)
        throw new Error("Original command differs");
    if (record.body === undefined) {
        let size = i.body.length;
        for (const [other, value] of payloads)
            if (other !== key)
                size += value.body.length;
        for (const [other, value] of payloads) {
            if (size <= memoryBound)
                break;
            if (other !== key) {
                payloads.delete(other);
                size -= value.body.length;
            }
        }
        payloads.set(key, { digest, body: i.body });
    }
    return record;
}
export function restoreCustomization(storageKey: string, record: CustomPending | null): CustomIntent | null {
    if (!record)
        return null;
    const cached = payloads.get(memoryKey(storageKey, record.key)), body = record.body ?? (cached?.digest === record.digest ? cached.body : undefined);
    return body === undefined ? null : { key: record.key, path: record.path, version: record.version, body };
}
export function forgetCustomization(storageKey: string, key: string) { payloads.delete(memoryKey(storageKey, key)); }
