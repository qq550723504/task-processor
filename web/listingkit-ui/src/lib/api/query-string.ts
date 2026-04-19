type QueryValue =
  | string
  | number
  | boolean
  | null
  | undefined
  | Array<string | number | boolean>;

export function buildQueryString(input: Record<string, QueryValue>) {
  const params = new URLSearchParams();
  const entries = Object.entries(input).sort(([left], [right]) =>
    left.localeCompare(right),
  );

  for (const [key, value] of entries) {
    if (
      value === undefined ||
      value === null ||
      value === "" ||
      (Array.isArray(value) && value.length === 0)
    ) {
      continue;
    }

    if (Array.isArray(value)) {
      value.forEach((item) => params.append(key, String(item)));
      continue;
    }

    params.set(key, String(value));
  }

  return params.toString();
}
