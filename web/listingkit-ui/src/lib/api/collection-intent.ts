import { z } from "zod";
import { COLLECTION_MAX_BYTES, collectionID, collectionCommandSchema } from "../contracts/product-collection";
import type { CollectionIntent } from "./product-collection";

const storageKey = "listingkit.collection.intent";
const identity = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const envelope = z.object({ userId: identity, organizationId: identity, key: collectionID, command: collectionCommandSchema }).strict();
export function parseCollectionIntent(raw: string | null): CollectionIntent | null {
  if (!raw || new TextEncoder().encode(raw).length > COLLECTION_MAX_BYTES + 1024) return null;
  try { const parsed = envelope.safeParse(JSON.parse(raw)); return parsed.success ? parsed.data : null; }
  catch { return null; }
}
export function loadCollectionIntent(): CollectionIntent | null {
  try { return parseCollectionIntent(localStorage.getItem(storageKey)); } catch { return null; }
}
export function saveCollectionIntent(intent: CollectionIntent | null): boolean {
  try {
    if (intent === null) { localStorage.removeItem(storageKey); return true; }
    const raw = JSON.stringify(intent);
    if (!parseCollectionIntent(raw)) return false;
    localStorage.setItem(storageKey, raw);
    return true;
  } catch { return false; }
}
