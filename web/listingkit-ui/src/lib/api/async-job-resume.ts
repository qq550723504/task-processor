const STORAGE_PREFIX = "listingkit.async-job.";
const ENTRY_TTL_MS = 2 * 60 * 60 * 1000;

type AsyncJobResumeEntry = {
  jobId: string;
  source: "backend" | "next";
  updatedAt: number;
};

export function buildAsyncJobResumeKey(path: string, body: unknown) {
  return `${STORAGE_PREFIX}${path}:${stableSerialize(body)}`;
}

export function loadAsyncJobResumeEntry(key: string): AsyncJobResumeEntry | null {
  const storage = getStorage();
  if (!storage) {
    return null;
  }

  const raw = storage.getItem(key);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<AsyncJobResumeEntry>;
    if (
      typeof parsed.jobId !== "string" ||
      !parsed.jobId ||
      typeof parsed.updatedAt !== "number"
    ) {
      storage.removeItem(key);
      return null;
    }
    if (Date.now() - parsed.updatedAt > ENTRY_TTL_MS) {
      storage.removeItem(key);
      return null;
    }
    return {
      jobId: parsed.jobId,
      source: parsed.source === "backend" ? "backend" : "next",
      updatedAt: parsed.updatedAt,
    };
  } catch {
    storage.removeItem(key);
    return null;
  }
}

export function saveAsyncJobResumeEntry(
  key: string,
  jobId: string,
  source: AsyncJobResumeEntry["source"] = "next",
) {
  const storage = getStorage();
  if (!storage || !jobId.trim()) {
    return;
  }
  storage.setItem(
    key,
    JSON.stringify({
      jobId,
      source,
      updatedAt: Date.now(),
    } satisfies AsyncJobResumeEntry),
  );
}

export function clearAsyncJobResumeEntry(key: string) {
  const storage = getStorage();
  storage?.removeItem(key);
}

function getStorage() {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage;
}

function stableSerialize(value: unknown): string {
  return JSON.stringify(normalizeValue(value));
}

function normalizeValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeValue(item));
  }
  if (value && typeof value === "object") {
    return Object.keys(value as Record<string, unknown>)
      .sort()
      .reduce<Record<string, unknown>>((result, key) => {
        result[key] = normalizeValue((value as Record<string, unknown>)[key]);
        return result;
      }, {});
  }
  return value;
}
