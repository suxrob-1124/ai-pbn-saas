"use client";

import { FormEvent, Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { FiEye, FiEyeOff } from "react-icons/fi";
import { post } from "../../../lib/http";

function ResetConfirmContent() {
  const search = useSearchParams();
  const router = useRouter();
  const [token, setToken] = useState(search.get("token") || "");
  const [newPassword, setNewPassword] = useState("");
  const [showNew, setShowNew] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const t = search.get("token");
    if (t) {
      setToken(t);
    }
  }, [search]);

  const confirm = async (e: FormEvent) => {
    e.preventDefault();
    if (!token) {
      setError("Нет токена сброса");
      return;
    }
    setLoading(true);
    setError(null);
    setStatus(null);
    try {
      await post("/api/password/reset/confirm", { token, newPassword });
      setStatus("Пароль обновлён, переходим на страницу входа...");
      setTimeout(() => router.replace("/login"), 800);
    } catch (err: any) {
      setError(err?.message || "Ошибка");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-xl bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
      <h2 className="text-lg font-semibold">Применить токен сброса</h2>
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
        <div className="space-y-1">
          <label className="text-sm text-slate-500 dark:text-slate-400">Новый пароль</label>
          <div className="relative">
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 pr-10 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              type={showNew ? "text" : "password"}
              autoComplete="new-password"
              required
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 dark:text-slate-400"
              onClick={() => setShowNew((v) => !v)}
            >
              {showNew ? <FiEyeOff /> : <FiEye />}
            </button>
          </div>
        </div>
        {error && <div className="text-red-400 text-sm">{error}</div>}
        {status && <div className="text-emerald-500 text-sm">{status}</div>}
        <button
          className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          disabled={loading}
        >
          {loading ? "Применяем..." : "Сбросить пароль"}
        </button>
      </form>
    </div>
  );
}

export default function ResetConfirmPage() {
  return (
    <Suspense fallback={<div className="text-slate-500">Загрузка...</div>}>
      <ResetConfirmContent />
    </Suspense>
  );
}
