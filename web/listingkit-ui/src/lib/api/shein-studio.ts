import type {
  SheinStudioGenerateRequest,
  SheinStudioGenerateResponse,
} from "@/lib/types/shein-studio";

export async function generateSheinStudioDesigns(
  body: SheinStudioGenerateRequest,
) {
  const response = await fetch("/api/shein-studio/generate-designs", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    cache: "no-store",
  });
  const payload = (await response.json()) as
    | SheinStudioGenerateResponse
    | { message?: string };
  if (!response.ok) {
    const message =
      "message" in payload ? payload.message : undefined;
    throw new Error(message || "Failed to generate SHEIN studio designs");
  }
  return payload as SheinStudioGenerateResponse;
}
