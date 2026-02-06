"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { FiEye, FiEyeOff } from "react-icons/fi";
import { post } from "../../lib/http";
import { getToken } from "../../lib/token";
import { useCaptcha } from "../../lib/useCaptcha";

type LoginResponse = { token: string; email: string };

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [captcha, setCaptcha] = useState("");
  const { captchaId, question, ttl, remainingAttempts, answer, setAnswer, refresh: refreshCaptcha } = useCaptcha(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);

  useEffect(() => {
    if (getToken()) {
      router.replace("/projects");
    }
  }, [router]);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await post<LoginResponse>("/api/login", {
        email,
        password,
        captchaId: captchaId || undefined,
        captchaAnswer: answer || undefined
      });
      router.replace("/projects");
      router.refresh();
    } catch (err: any) {
      setError(err?.message || "Ошибка входа");
      refreshCaptcha();
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
        <h2 className="text-xl font-semibold">Вход</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">Используйте почту и пароль. При включенной капче добавьте токен.</p>
        <form onSubmit={onSubmit} className="space-y-3">
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Почта</label>
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
              type={showPassword ? "text" : "password"}
              autoComplete="current-password"
              required
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 dark:text-slate-400"
              onClick={() => setShowPassword((v) => !v)}
            >
              {showPassword ? <FiEyeOff /> : <FiEye />}
            </button>
          </div>
        </div>
        {captchaId && (
          <div className="space-y-1">
            <label className="text-sm text-slate-500 dark:text-slate-400">Капча (обязательно)</label>
            <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
              <span>{question || "Введите ответ на капчу"}</span>
              <button type="button" onClick={refreshCaptcha} className="underline hover:text-indigo-600 dark:hover:text-indigo-400">
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
          <button
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            disabled={loading}
          >
            {loading ? "Входим..." : "Войти"}
          </button>
        </form>
      </div>
      <div className="text-xs text-slate-500 dark:text-slate-400 flex flex-col gap-1">
        <Link className="underline hover:text-indigo-600 dark:hover:text-indigo-400" href="/verify">
          Подтвердить почту
        </Link>
        <Link className="underline hover:text-indigo-600 dark:hover:text-indigo-400" href="/reset">
          Забыли пароль? Сбросить
        </Link>
      </div>
    </div>
  );
}
