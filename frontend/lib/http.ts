export const apiBase = () =>
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8080";

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {
      // ignore
    }
    throw new Error(message);
  }
  return (await res.json()) as T;
}

export async function refreshTokens(): Promise<boolean> {
  const res = await fetch(`${apiBase()}/api/refresh`, {
    method: "POST",
    credentials: "include"
  });
  return res.ok;
}

type CacheEntry<T> = {
  expiresAt: number;
  value: T;
};

const authCache = new Map<string, CacheEntry<any>>();
const authInFlight = new Map<string, Promise<any>>();

export async function authFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const url = path.startsWith("http") ? path : `${apiBase()}${path}`;
  const attempt = async (): Promise<{ ok: boolean; res: Response }> => {
    const res = await fetch(url, { credentials: "include", ...init });
    return { ok: res.ok, res };
  };

  let { ok, res } = await attempt();
  if (!ok && res.status === 401) {
    const refreshed = await refreshTokens();
    if (refreshed) {
      const retry = await attempt();
      ok = retry.ok;
      res = retry.res;
    }
  }
  return handle<T>(res);
}

export async function authFetchCached<T>(
  path: string,
  init?: RequestInit,
  opts?: { ttlMs?: number; bypassCache?: boolean; key?: string }
): Promise<T> {
  const method = (init?.method || "GET").toUpperCase();
  if (method !== "GET") {
    return authFetch<T>(path, init);
  }
  const key = opts?.key || `${method}:${path}`;
  const now = Date.now();
  const ttlMs = opts?.ttlMs ?? 15000;
  if (!opts?.bypassCache) {
    const cached = authCache.get(key);
    if (cached && cached.expiresAt > now) {
      return cached.value as T;
    }
    const inFlight = authInFlight.get(key);
    if (inFlight) {
      return inFlight as Promise<T>;
    }
  }
  const request = authFetch<T>(path, init)
    .then((value) => {
      authCache.set(key, { expiresAt: now + ttlMs, value });
      return value;
    })
    .finally(() => {
      authInFlight.delete(key);
    });
  authInFlight.set(key, request);
  return request;
}

export function invalidateAuthCache(prefix?: string) {
  if (!prefix) {
    authCache.clear();
    authInFlight.clear();
    return;
  }
  for (const key of authCache.keys()) {
    if (key.includes(prefix)) {
      authCache.delete(key);
    }
  }
  for (const key of authInFlight.keys()) {
    if (key.includes(prefix)) {
      authInFlight.delete(key);
    }
  }
}

export async function post<T>(path: string, body?: any): Promise<T> {
  const res = await fetch(`${apiBase()}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: body ? JSON.stringify(body) : undefined
  });
  return handle<T>(res);
}

export async function patch<T>(path: string, body?: any): Promise<T> {
  const res = await fetch(`${apiBase()}${path}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: body ? JSON.stringify(body) : undefined
  });
  return handle<T>(res);
}

export async function del<T>(path: string): Promise<T> {
  const res = await fetch(`${apiBase()}${path}`, {
    method: "DELETE",
    credentials: "include"
  });
  return handle<T>(res);
}
