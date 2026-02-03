"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { post } from "../../lib/http";
import { useCaptcha } from "../../lib/useCaptcha";

export default function ResetRequestPage() {
  const [email, setEmail] = useState("");
  const { captchaId, question, ttl, remainingAttempts, answer, setAnswer, refresh: refreshCaptcha } = useCaptcha(true);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const request = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setStatus(null);
    try {
      await post<{ status: string }>("/api/password/reset/request", { email, captchaId, captchaAnswer: answer });
      setStatus("Письмо отправлено. Проверьте почту и перейдите по ссылке.");
    } catch (err: any) {
      setError(err?.message || "Ошибка");
      refreshCaptcha();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-xl bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
      <h2 className="text-lg font-semibold">Сброс пароля</h2>
      <p className="text-sm text-slate-500 dark:text-slate-400">Укажите email, мы отправим ссылку для сброса пароля.</p>
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
        {captchaId && (
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Капча (обязательно)</label>
            <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
              <span>{question}</span>
              <button type="button" className="underline hover:text-indigo-600 dark:hover:text-indigo-400" onClick={refreshCaptcha}>
                Обновить
              </button>
            </div>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              placeholder="Ответ на капчу"
              required
            />
            <div className="text-xs text-slate-500 dark:text-slate-400">Истекает через {ttl}s</div>
            {remainingAttempts !== undefined && (
              <div className="text-xs text-slate-500 dark:text-slate-400">Осталось попыток: {remainingAttempts}</div>
            )}
          </div>
        )}
        {error && <div className="text-red-400 text-sm">{error}</div>}
        {status && <div className="text-emerald-500 text-sm">{status}</div>}
        <button
          className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
          disabled={loading}
        >
          {loading ? "Отправляем..." : "Отправить письмо"}
        </button>
      </form>
      <div className="text-xs text-slate-500 dark:text-slate-400">
        Уже есть токен?{" "}
        <Link className="underline hover:text-indigo-600 dark:hover:text-indigo-400" href="/reset/confirm">
          Перейти к сбросу
        </Link>
      </div>
    </div>
  );
}
