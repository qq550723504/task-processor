import { z } from "zod";

import { parseJsonResponse, ResponseJsonParseError } from "@/lib/api/response-json";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  SDSCategory,
  SDSProductDetail,
  SDSProductListResponse,
  SDSShipmentArea,
} from "@/lib/types/sds";

const productVariantSchema = z
  .object({
    id: z.coerce.number().int(),
  })
  .passthrough();

const productSummarySchema = z
  .object({
    id: z.coerce.number().int(),
    name: z.string().nullish().default("").transform((value) => value ?? ""),
    categories: z
      .array(
        z
          .object({
            id: z.coerce.number().int(),
            name: z.string(),
          })
          .passthrough(),
      )
      .optional(),
    subproducts: z
      .object({
        items: z.array(productVariantSchema).optional(),
      })
      .passthrough()
      .optional(),
  })
  .passthrough();

const productListSchema = z
  .object({
    totalCount: z.coerce.number().int().nonnegative().optional(),
    page: z.coerce.number().int().nonnegative().optional(),
    size: z.coerce.number().int().positive().optional(),
    items: z.array(productSummarySchema).optional(),
  })
  .passthrough();

const shipmentAreaSchema = z
  .object({
    value: z.string(),
    label: z.string(),
    totalCount: z.coerce.number().int().nonnegative(),
  })
  .passthrough();

const categorySchema = z
  .object({
    id: z.coerce.number().int(),
    name: z.string(),
    count: z.coerce.number().int().nonnegative(),
  })
  .passthrough();

export async function parseSDSProductListResponse(
  response: Response,
): Promise<SDSProductListResponse> {
  const payload = await parseSDSJsonResponse(
    response,
    "Failed to load SDS products",
  );
  return parseApiResponseShape(
    payload,
    productListSchema,
    "SDS API returned an unexpected product list response",
  );
}

export async function parseSDSProductDetailResponse(
  response: Response,
): Promise<SDSProductDetail> {
  const payload = await parseSDSJsonResponse(
    response,
    "Failed to load SDS product detail",
  );
  return parseApiResponseShape(
    payload,
    productSummarySchema,
    "SDS API returned an unexpected product detail response",
  );
}

export async function parseSDSShipmentAreasResponse(
  response: Response,
): Promise<SDSShipmentArea[]> {
  const payload = await parseSDSJsonResponse(
    response,
    "Failed to load SDS shipment areas",
  );
  return parseApiResponseShape(
    payload,
    z.array(shipmentAreaSchema),
    "SDS API returned an unexpected shipment area response",
  );
}

export async function parseSDSCategoriesResponse(
  response: Response,
): Promise<SDSCategory[]> {
  const payload = await parseSDSJsonResponse(
    response,
    "Failed to load SDS categories",
  );
  return parseApiResponseShape(
    payload,
    z.array(categorySchema),
    "SDS API returned an unexpected category response",
  );
}

async function parseSDSJsonResponse(response: Response, fallbackMessage: string) {
  let payload: unknown;
  try {
    payload = await parseJsonResponse(response);
  } catch (error) {
    if (error instanceof ResponseJsonParseError) {
      throw new Error(`SDS API returned invalid JSON: ${response.status}`);
    }
    throw error;
  }

  if (!response.ok) {
    throw new Error(readErrorMessage(payload) ?? fallbackMessage);
  }

  return payload;
}

function readErrorMessage(payload: unknown) {
  if (!payload || typeof payload !== "object") {
    return undefined;
  }
  const message = (payload as { message?: unknown; error?: unknown }).message;
  if (typeof message === "string" && message.trim()) {
    return message;
  }
  const error = (payload as { error?: unknown }).error;
  return typeof error === "string" && error.trim() ? error : undefined;
}
