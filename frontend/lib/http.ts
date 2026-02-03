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
