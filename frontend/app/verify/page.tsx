"use client";

import { FormEvent, Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useRouter } from "next/navigation";
import { post } from "../../lib/http";

export default function VerifyPage() {
  return (
    <Suspense fallback={<div className="text-slate-500">Загрузка...</div>}>
      <VerifyContent />
    </Suspense>
  );
}

function VerifyContent() {
  const search = useSearchParams();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [token, setToken] = useState(search.get("token") || "");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const t = search.get("token");
    if (t) {
      setToken(t);
      confirmToken(t);
    }
  }, [search]);

  const request = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setStatus(null);
    try {
      await post<{ status: string }>("/api/verify/request", { email });
      setStatus("Письмо отправлено. Проверьте почту");
    } catch (err: any) {
      setError(err?.message || "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  const confirm = async (e: FormEvent) => {
    e.preventDefault();
    await confirmToken(token);
  };

  const confirmToken = async (t: string) => {
    if (!t) return;
    setLoading(true);
    setError(null);
    setStatus(null);
    try {
      await post("/api/verify/confirm", { token: t });
      setStatus("Email подтверждён, перенаправляем...");
      setTimeout(() => router.replace("/projects"), 800);
    } catch (err: any) {
      setError(err?.message || "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
        <h2 className="text-lg font-semibold">Запрос подтверждения</h2>
        <form onSubmit={request} className="space-y-3">
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Email</label>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              type="email"
              autoComplete="username"
              required
            />
          </div>
          {error && <div className="text-red-400 text-sm">{error}</div>}
          {status && <div className="text-emerald-500 text-sm">{status}</div>}
          <button
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            disabled={loading}
          >
            {loading ? "Отправляем..." : "Отправить письмо"}
          </button>
        </form>
      </div>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
        <h2 className="text-lg font-semibold">Подтвердить токен</h2>
        <form onSubmit={confirm} className="space-y-3">
          {!search.get("token") && (
            <div className="space-y-1">
              <label className="text-sm text-slate-500 dark:text-slate-400">Токен</label>
              <input
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoComplete="one-time-code"
                required
              />
            </div>
          )}
          {search.get("token") && (
            <div className="text-xs text-slate-500 dark:text-slate-400">Токен получен из ссылки, вводить не нужно.</div>
          )}
          {error && <div className="text-red-400 text-sm">{error}</div>}
          {status && <div className="text-emerald-500 text-sm">{status}</div>}
          <button
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
            disabled={loading}
          >
            {loading ? "Подтверждаем..." : "Подтвердить"}
          </button>
        </form>
      </div>
    </div>
  );
}
