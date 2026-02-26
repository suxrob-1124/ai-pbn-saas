type QueryReader = {
  get: (name: string) => string | null;
};

export function readPositiveIntParam(params: QueryReader, key: string, fallback: number): number {
  const raw = Number(params.get(key) || fallback);
  return Number.isFinite(raw) && raw > 0 ? raw : fallback;
}

export function readStringParam(params: QueryReader, key: string, fallback = ""): string {
  const raw = params.get(key);
  return raw === null ? fallback : raw;
}

export function readEnumParam<T extends string>(
  params: QueryReader,
  key: string,
  allowed: readonly T[],
  fallback: T
): T {
  const raw = (params.get(key) || "").trim() as T;
  return allowed.includes(raw) ? raw : fallback;
}

export function setOptionalParam(
  params: URLSearchParams,
  key: string,
  value: string,
  defaultValue = ""
) {
  const normalized = value.trim();
  if (!normalized || normalized === defaultValue) {
    params.delete(key);
    return;
  }
  params.set(key, normalized);
}

export function setOptionalNumberParam(
  params: URLSearchParams,
  key: string,
  value: number,
  defaultValue: number
) {
  if (!Number.isFinite(value) || value <= 0 || value === defaultValue) {
    params.delete(key);
    return;
  }
  params.set(key, String(value));
}
