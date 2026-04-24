type NanobananaResult = {
  dataUrl: string;
  revisedPrompt?: string;
};

type NanobananaSubmitResponse = {
  code?: number;
  msg?: string;
  data?: {
    id?: string;
  };
};

type NanobananaPollResponse = {
  code?: number;
  msg?: string;
  data?: {
    status?: string;
    failure_reason?: string;
    error?: string;
    results?: Array<{
      url?: string;
      content?: string;
    }>;
  };
};

function nanobananaAspectRatio(size: string) {
  switch (size) {
    case "1024x1024":
    case "1536x1536":
    case "2048x2048":
      return "1:1";
    default:
      return "auto";
  }
}

function nanobananaImageSize(size: string) {
  switch (size) {
    case "2048x2048":
      return "2K";
    case "4096x4096":
      return "4K";
    default:
      return "1K";
  }
}

function buildNanobananaResultURL(submitURL: string) {
  const parsed = new URL(submitURL);
  parsed.pathname = `${parsed.pathname.replace(/\/[^/]*$/, "")}/result`;
  return parsed.toString();
}

async function submitNanobananaJob(input: {
  apiKey: string;
  model: string;
  prompt: string;
  size: string;
  submitURL: string;
}) {
  const response = await fetch(input.submitURL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${input.apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      model: input.model,
      prompt: input.prompt,
      aspectRatio: nanobananaAspectRatio(input.size),
      imageSize: nanobananaImageSize(input.size),
      webHook: "-1",
      shutProgress: true,
    }),
    cache: "no-store",
  });

  const payload = (await response.json()) as NanobananaSubmitResponse;
  if (!response.ok) {
    throw new Error(payload.msg || `submit nanobanana job returned status ${response.status}`);
  }
  if (payload.code !== 0 || !payload.data?.id) {
    throw new Error(payload.msg || "submit nanobanana job failed");
  }
  return payload.data.id;
}

async function pollNanobananaResult(input: {
  apiKey: string;
  submitURL: string;
  jobId: string;
}) {
  const resultURL = buildNanobananaResultURL(input.submitURL);

  for (;;) {
    const response = await fetch(resultURL, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${input.apiKey}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ id: input.jobId }),
      cache: "no-store",
    });

    const payload = (await response.json()) as NanobananaPollResponse;
    if (!response.ok) {
      throw new Error(payload.msg || `poll nanobanana job returned status ${response.status}`);
    }
    if (payload.code !== 0) {
      throw new Error(payload.msg || "poll nanobanana job failed");
    }

    const status = payload.data?.status ?? "";
    if (status === "succeeded") {
      return payload.data;
    }
    if (status === "failed") {
      throw new Error(
        payload.data?.failure_reason || payload.data?.error || "nanobanana job failed",
      );
    }

    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

export async function generateNanobananaImage(input: {
  apiKey: string;
  model: string;
  prompt: string;
  size: string;
  submitURL: string;
}): Promise<NanobananaResult> {
  const jobId = await submitNanobananaJob(input);
  const result = await pollNanobananaResult({
    apiKey: input.apiKey,
    submitURL: input.submitURL,
    jobId,
  });
  if (!result) {
    throw new Error("nanobanana result is empty");
  }

  const first = result.results?.[0];
  if (!first?.url) {
    throw new Error("nanobanana result missing image url");
  }

  const imageResponse = await fetch(first.url, { cache: "no-store" });
  if (!imageResponse.ok) {
    throw new Error(`download nanobanana image returned status ${imageResponse.status}`);
  }

  const arrayBuffer = await imageResponse.arrayBuffer();
  const base64 = Buffer.from(arrayBuffer).toString("base64");
  const contentType =
    imageResponse.headers.get("content-type")?.split(";")[0]?.trim() ||
    "image/png";

  return {
    dataUrl: `data:${contentType};base64,${base64}`,
    revisedPrompt: first.content,
  };
}
