import { randomUUID } from "node:crypto";

import { NextResponse } from "next/server";

import { persistGeneratedDesignAsset } from "@/lib/server/shein-studio-assets";
import { ensureRepoEnvLoaded, getRepoEnvValue } from "@/lib/server/repo-env";
import { generateNanobananaImage } from "@/lib/server/nanobanana";
import type { SheinStudioGenerateResponse } from "@/lib/types/shein-studio";

export const dynamic = "force-dynamic";

type GenerateRequestBody = {
  prompt?: string;
  count?: number;
  printableWidth?: number;
  printableHeight?: number;
};

type OpenAIImageData = {
  b64_json?: string;
  revised_prompt?: string;
};

type OpenAIImageResponse = {
  data?: OpenAIImageData[];
};

ensureRepoEnvLoaded();

function isUsableSecret(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed) {
    return false;
  }
  if (
    trimmed === "你的key" ||
    trimmed === "your_api_key" ||
    trimmed === "replace-me"
  ) {
    return false;
  }
  return /^[\x00-\x7F]+$/.test(trimmed);
}

function firstUsableValue(...values: Array<string | undefined>) {
  return values.find((value) => isUsableSecret(value))?.trim();
}

function resolveImageConfig() {
  const apiKey = firstUsableValue(
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY"),
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_API_KEY"),
    getRepoEnvValue("OPENAI_API_KEY"),
  );
  const baseURL =
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL") ||
    getRepoEnvValue("OPENAI_BASE_URL") ||
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_BASE_URL") ||
    "https://api.openai.com/v1";
  const model =
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL") ||
    getRepoEnvValue("OPENAI_IMAGE_MODEL") ||
    getRepoEnvValue("OPENAI_MODEL_IMAGE") ||
    "gpt-image-1";
  const apiStyle =
    getRepoEnvValue("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE") ||
    getRepoEnvValue("OPENAI_API_STYLE") ||
    "openai";
  return { apiKey, apiStyle, baseURL, model };
}

function resolveImageSize(width?: number, height?: number) {
  if (!width || !height) {
    return "1024x1024";
  }
  const ratio = width / height;
  if (ratio >= 1.18) {
    return "1536x1024";
  }
  if (ratio <= 0.84) {
    return "1024x1536";
  }
  return "1024x1024";
}

function buildPrompt(prompt: string, printableWidth?: number, printableHeight?: number) {
  const cleaned = prompt.trim();
  const printableHint =
    printableWidth && printableHeight
      ? `Target print area: ${printableWidth} by ${printableHeight} pixels.`
      : "";
  return [
    "Create a single apparel print graphic for ecommerce POD use.",
    "The final asset must be a PNG-style graphic with a fully transparent background / alpha channel.",
    "Do not place the artwork on white, black, colored, textured, gradient, paper, canvas, studio, room, or garment backgrounds.",
    "Return a flat design only, not a mockup, not a model, not a folded garment.",
    "Keep the composition centered and marketplace-safe, with strong readability at print size.",
    "Keep empty space outside the artwork fully transparent, not painted white.",
    "Avoid extra scene elements outside the artwork itself.",
    "Optimize for real garment printing, heat-transfer, or POD production.",
    "Use bold, clean shapes and clear silhouette separation.",
    "Avoid tiny text, dense paragraphs, micro details, thin outlines, delicate line art, halftone noise, and scattered miniature elements.",
    "Avoid semi-transparent effects, weak low-contrast details, photographic backgrounds, and edge-to-edge clutter.",
    "If typography is used, keep it minimal, large, legible, and secondary to the main graphic.",
    "Favor one strong focal concept with simple layering and enough negative space.",
    printableHint,
    `Theme prompt: ${cleaned}`,
  ]
    .filter(Boolean)
    .join(" ");
}

async function generateOneDesign({
  apiKey,
  baseURL,
  model,
  prompt,
  size,
}: {
  apiKey: string;
  baseURL: string;
  model: string;
  prompt: string;
  size: string;
}) {
  const response = await fetch(`${baseURL.replace(/\/$/, "")}/images/generations`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      model,
      prompt,
      size,
      n: 1,
      background: "transparent",
      output_format: "png",
      response_format: "b64_json",
    }),
    cache: "no-store",
  });

  const payload = (await response.json()) as OpenAIImageResponse & {
    error?: { message?: string };
  };

  if (!response.ok) {
    throw new Error(payload.error?.message || "OpenAI image generation failed");
  }

  const first = payload.data?.[0];
  if (!first?.b64_json) {
    throw new Error("OpenAI image generation returned no image data");
  }

  return {
    dataUrl: `data:image/png;base64,${first.b64_json}`,
    revisedPrompt: first.revised_prompt,
  };
}

export async function POST(request: Request) {
  const body = (await request.json().catch(() => ({}))) as GenerateRequestBody;
  const prompt = body.prompt?.trim() || "";
  const count = Math.max(1, Math.min(Number(body.count) || 1, 8));
  const printableWidth = Number(body.printableWidth) || undefined;
  const printableHeight = Number(body.printableHeight) || undefined;

  if (!prompt) {
    return NextResponse.json(
      { message: "Theme prompt is required." },
      { status: 400 },
    );
  }

  const { apiKey, apiStyle, baseURL, model } = resolveImageConfig();
  if (!apiKey) {
    return NextResponse.json(
      {
        message:
          "Image generation credentials are not configured for SHEIN Studio.",
      },
      { status: 500 },
    );
  }

  try {
    const resolvedPrompt = buildPrompt(prompt, printableWidth, printableHeight);
    const size = resolveImageSize(printableWidth, printableHeight);
    const images = await Promise.all(
      Array.from({ length: count }, async () => {
        const id = randomUUID();
        const generated =
          apiStyle === "nanobanana"
            ? await generateNanobananaImage({
                apiKey,
                model,
                prompt: resolvedPrompt,
                size,
                submitURL: baseURL,
              })
            : await generateOneDesign({
                apiKey,
                baseURL,
                model,
                prompt: resolvedPrompt,
                size,
              });
        const asset = await persistGeneratedDesignAsset({
          id,
          dataUrl: generated.dataUrl,
        });
        return {
          id,
          imageUrl: asset.imageUrl,
          revisedPrompt: generated.revisedPrompt,
        };
      }),
    );

    const payload: SheinStudioGenerateResponse = {
      prompt,
      printableWidth,
      printableHeight,
      images,
    };

    return NextResponse.json(payload);
  } catch (error) {
    return NextResponse.json(
      {
        message:
          error instanceof Error ? error.message : "Unknown SHEIN Studio error",
      },
      { status: 502 },
    );
  }
}
