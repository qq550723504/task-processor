import { beforeEach, describe, expect, it, vi } from "vitest";

import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  mapStudioSessionDetailToDraft,
  replaceSheinStudioSessionDesigns,
} from "@/lib/api/shein-studio-sessions";
import { apiAsyncRequest, apiRequest } from "@/lib/api/client";

vi.mock("@/lib/api/client", () => ({
  apiAsyncRequest: vi.fn(),
  apiRequest: vi.fn(),
}));

const mockedApiAsyncRequest = vi.mocked(apiAsyncRequest);
const mockedApiRequest = vi.mocked(apiRequest);

describe("shein studio design metadata", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("stamps generated designs with prompt and generation model metadata", async () => {
    mockedApiAsyncRequest.mockResolvedValueOnce({
      prompt: "retro botanical clock",
      image_model: "gpt-image-2",
      transparent_background: true,
      images: [
        {
          id: "design-1",
          image_url: "https://oss.example.com/design-1.png",
          revised_prompt: "revised",
        },
      ],
    });

    const result = await generateSheinStudioDesigns({
      prompt: "retro botanical clock",
      count: 1,
      variationIntensity: "light",
      imageModel: "gpt-image-2",
      transparentBackground: true,
    });

    expect(result.images[0]).toMatchObject({
      id: "design-1",
      prompt: "retro botanical clock",
      imageModel: "gpt-image-2",
      transparentBackground: true,
      variationIntensity: "light",
      revisedPrompt: "revised",
    });
  });

  it("persists design prompt and model metadata in studio sessions", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      session: { id: "session-1" },
      designs: [],
    });

    await replaceSheinStudioSessionDesigns("session-1", {
      approvedDesignIds: ["design-1"],
      designs: [
        {
          id: "design-1",
          imageUrl: "https://oss.example.com/design-1.png",
          prompt: "retro botanical clock",
          imageModel: "gpt-image-2",
          transparentBackground: true,
          variationIntensity: "light",
        },
      ],
    });

    expect(mockedApiRequest).toHaveBeenCalledWith(
      "/studio/sessions/session-1/designs",
      expect.objectContaining({
        body: expect.objectContaining({
          designs: [
            expect.objectContaining({
              prompt: "retro botanical clock",
              image_model: "gpt-image-2",
              transparent_background: true,
              variation_intensity: "light",
            }),
          ],
        }),
      }),
    );
  });

  it("loads design prompt and model metadata from studio sessions", () => {
    const draft = mapStudioSessionDetailToDraft({
      session: {
        id: "session-1",
        prompt: "fallback session prompt",
      },
      designs: [
        {
          id: "design-1",
          image_url: "https://oss.example.com/design-1.png",
          prompt: "retro botanical clock",
          image_model: "gpt-image-2",
          transparent_background: true,
          variation_intensity: "light",
        },
      ],
    });

    expect(draft?.designs[0]).toMatchObject({
      prompt: "retro botanical clock",
      imageModel: "gpt-image-2",
      transparentBackground: true,
      variationIntensity: "light",
    });
  });
});
