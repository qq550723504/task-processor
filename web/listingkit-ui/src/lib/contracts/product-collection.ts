import { z } from "zod";
import { isAcquisitionUUID } from "./product-acquisition";

export const COLLECTION_MAX_BYTES = 2 * 1024 * 1024;
export type CollectionRoute = "batches" | "items" | "own" | "detail" | "operation" | "command";
export const collectionID = z.string().refine(isAcquisitionUUID);
const bytes = (max: number) => z.string().refine(value => new TextEncoder().encode(value).length <= max && !value.includes("\0"));
const revision = z.number().int().positive().max(Number.MAX_SAFE_INTEGER);
const count = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const timestamp = z.iso.datetime({ offset: true });
const version = z.string().regex(/^[1-9][0-9]{0,18}$/).refine(value => BigInt(value) <= BigInt("9223372036854775807"));
const name = bytes(200).refine(value => value.length > 0 && value.trim() === value && !/[\r\n]/.test(value));
const httpsURL = bytes(2048).refine(value => { try { const url = new URL(value); return url.protocol === "https:" && url.hostname !== "" && !url.username && !url.password && !url.hash; } catch { return false; } });
const attributes = z.record(bytes(200), bytes(8192)).refine(value => Object.keys(value).length <= 256);
export const ownProductSchema = z.object({
  title: bytes(2000).refine(value => value.trim().length > 0), description: bytes(1024 * 1024), brand: bytes(2000).optional(),
  attributes: attributes.optional(), images: z.array(httpsURL).max(40),
  variants: z.array(z.object({ sourceId: bytes(128), title: bytes(2000), sku: bytes(200), attributes,
    currency: bytes(16), price: z.number().finite().nonnegative(), stock: count }).strict()).max(1000).optional(),
}).strict();
export const collectionCommandSchema = z.discriminatedUnion("action", [
  z.object({ action: z.literal("create_batch"), name }).strict(),
  z.object({ action: z.literal("rename_batch"), batchId: collectionID, expectedRevision: revision, name }).strict(),
  z.object({ action: z.literal("archive_batch"), batchId: collectionID, expectedRevision: revision }).strict(),
  z.object({ action: z.literal("move_item"), itemId: collectionID, targetBatchId: collectionID, expectedRevision: revision }).strict(),
  z.object({ action: z.literal("archive_item"), itemId: collectionID, expectedRevision: revision }).strict(),
  z.object({ action: z.literal("add_acquisition"), batchId: collectionID.optional(), sourceOperationId: collectionID }).strict(),
  z.object({ action: z.literal("create_product"), batchId: collectionID.optional(), product: ownProductSchema }).strict(),
]);
export type CollectionCommand = z.infer<typeof collectionCommandSchema>;
export const collectionBatchSchema = z.object({ id: collectionID, name, kind: z.enum(["acquisition", "own", "manual"]),
  revision, count, createdAt: timestamp, archivedAt: timestamp.optional() });
export const collectionItemSchema = z.object({ id: collectionID, batchId: collectionID, revision, createdAt: timestamp, archivedAt: timestamp.optional(), title: bytes(2000).optional(), thumbnailUrl: bytes(2048).optional(),
  source: z.object({ productKey: bytes(128).min(1), publicationId: bytes(128).min(1), version,
    kind: z.enum(["acquisition", "own"]), operationId: collectionID.optional() }) });
export const collectionReceiptSchema = z.object({ operationId: collectionID, batchId: collectionID.optional(), itemId: collectionID.optional(), revision, replayed: z.boolean() });
const page = <T extends z.ZodType>(schema: T) => z.object({ items: z.array(schema).max(100), total: count, nextCursor: collectionID.optional() });
export const collectionBatchesSchema = page(collectionBatchSchema);
export const collectionItemsSchema = page(collectionItemSchema);
const attribute = z.object({ name: bytes(8192).optional(), value: bytes(8192).optional() });
const image = z.object({ url: bytes(2048), role: bytes(128).optional(), width: count.optional(), height: count.optional() });
const price = z.object({ currency: bytes(16).optional(), amount: z.number().finite().optional(), compare_at: z.number().finite().optional(), cost_price: z.number().finite().optional(), wholesale_min: count.optional() });
const dimensions = z.object({ length: z.number().finite().optional(), width: z.number().finite().optional(), height: z.number().finite().optional(), unit: bytes(128).optional() });
const weight = z.object({ value: z.number().finite().optional(), unit: bytes(128).optional() });
export const collectionDetailSchema = z.object({ item: collectionItemSchema, product: z.object({
  title: bytes(2000).optional(), description: bytes(1024 * 1024).optional(), brand: bytes(2000).optional(), category_path: z.array(bytes(8192)).max(256).optional(),
  selling_points: z.array(bytes(8192)).max(256).optional(), seo_keywords: z.array(bytes(8192)).max(256).optional(), attributes: z.array(attribute).max(4096).optional(),
  images: z.array(image).max(4096).optional(), variants: z.array(z.object({ source_id: bytes(128).optional(), title: bytes(2000).optional(), sku: bytes(200).optional(),
    attributes: z.array(attribute).max(4096).optional(), price: price.optional(), stock: count.optional(), images: z.array(image).max(4096).optional(), barcode: bytes(128).optional(), is_default: z.boolean().optional() })).max(4096).optional(),
  specifications: z.object({ dimensions: dimensions.optional(), weight: weight.optional(), package: z.object({ dimensions: dimensions.optional(), weight: weight.optional(), quantity: count.optional() }).optional(), technical: z.record(bytes(8192), bytes(8192)).optional() }).optional(),
  warnings: z.array(z.object({ code: bytes(128), field: bytes(8192).optional(), message: bytes(8192) })).max(256).optional(),
  sources: z.array(z.object({ type: bytes(128).optional(), platform: bytes(128).optional(), source_id: bytes(128).optional(), url: bytes(2048).optional(), reference_type: bytes(128).optional() })).max(4096).optional(),
}) });
export type CollectionBatch = z.infer<typeof collectionBatchSchema>;
export type CollectionItem = z.infer<typeof collectionItemSchema>;
export type CollectionDetail = z.infer<typeof collectionDetailSchema>;
export type CollectionReceipt = z.infer<typeof collectionReceiptSchema>;

export function collectionPath(method: string, path: string[]): CollectionRoute | null {
  if (path[0] !== "collections") return null;
  if (method === "POST" && path.length === 2 && path[1] === "commands") return "command";
  if (method !== "GET") return null;
  if (path.length === 2 && path[1] === "batches") return "batches";
  if (path.length === 2 && path[1] === "own-products") return "own";
  if (path.length === 4 && path[1] === "batches" && isAcquisitionUUID(path[2]!) && path[3] === "items") return "items";
  if (path.length === 3 && isAcquisitionUUID(path[2]!)) return path[1] === "items" ? "detail" : path[1] === "by-key" ? "operation" : null;
  return null;
}
export function parseCollectionResponse(route: CollectionRoute, payload: unknown, expectedID?: string): unknown | null {
  const schema = route === "batches" ? collectionBatchesSchema : route === "items" || route === "own" ? collectionItemsSchema : route === "detail" ? collectionDetailSchema : collectionReceiptSchema;
  const parsed = schema.safeParse(payload);
  if (!parsed.success || route === "detail" && expectedID && (parsed.data as CollectionDetail).item.id !== expectedID) return null;
  return parsed.data;
}
