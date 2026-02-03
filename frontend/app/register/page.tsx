"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { FiEye, FiEyeOff } from "react-icons/fi";
import { post } from "../../lib/http";
import { useCaptcha } from "../../lib/useCaptcha";

export default function RegisterPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [showPwd, setShowPwd] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const { captchaId, question, ttl, remainingAttempts, answer, setAnswer, refresh: refreshCaptcha } = useCaptcha(true);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      setError("Пароли не совпадают");
      return;
    }
    setLoading(true);
    setError(null);
    setStatus(null);
    try {
      await post("/api/register", { email, password, captchaId, captchaAnswer: answer });
      setStatus("Аккаунт создан. Проверьте почту для подтверждения.");
    } catch (err: any) {
      setError(err?.message || "Ошибка регистрации");
      refreshCaptcha();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
        <h2 className="text-xl font-semibold">Регистрация</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Создайте аккаунт, далее подтвердите email по ссылке из письма и войдите.
        </p>
        <form onSubmit={onSubmit} className="space-y-3">
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
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Пароль</label>
            <div className="relative">
              <input
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 pr-10 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                type={showPwd ? "text" : "password"}
                autoComplete="new-password"
                required
              />
              <button
                type="button"
                className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 dark:text-slate-400"
                onClick={() => setShowPwd((v) => !v)}
              >
                {showPwd ? <FiEyeOff /> : <FiEye />}
              </button>
            </div>
          </div>
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Повторите пароль</label>
            <div className="relative">
              <input
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 pr-10 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                type={showConfirm ? "text" : "password"}
                autoComplete="new-password"
                required
              />
              <button
                type="button"
                className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 dark:text-slate-400"
                onClick={() => setShowConfirm((v) => !v)}
              >
                {showConfirm ? <FiEyeOff /> : <FiEye />}
              </button>
            </div>
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
            {loading ? "Создаём..." : "Зарегистрироваться"}
          </button>
        </form>
      </div>
      <div className="text-xs text-slate-500 dark:text-slate-400 flex flex-col gap-1">
        <Link className="underline hover:text-indigo-600 dark:hover:text-indigo-400" href="/login">
          Уже есть аккаунт? Войти
        </Link>
      </div>
    </div>
  );
}
