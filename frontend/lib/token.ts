// With HttpOnly cookies, we don't store tokens in JS.
export function getToken(): string | null {
  return null;
}

export function setToken(_token: string) {}

export function clearToken() {}
