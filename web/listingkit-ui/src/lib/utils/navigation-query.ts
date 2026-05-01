const INTERNAL_QUERY_KEYS = new Set([
  "_rsc",
  "__nextLocale",
  "__nextDefaultLocale",
]);

export function sanitizedNavigationSearchParams(
  searchParams: URLSearchParams | ReadonlyURLSearchParamsLike,
) {
  const params = new URLSearchParams(searchParams.toString());
  for (const key of Array.from(params.keys())) {
    if (INTERNAL_QUERY_KEYS.has(key) || key.startsWith("__next")) {
      params.delete(key);
    }
  }
  return params;
}

type ReadonlyURLSearchParamsLike = {
  toString(): string;
};
