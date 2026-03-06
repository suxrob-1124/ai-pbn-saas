"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { FiEye, FiEyeOff } from "react-icons/fi";
import { post, del, authFetch } from "@/lib/http";
import { useAuthGuard } from "@/lib/useAuth";

export default function MePage() {
  const router = useRouter();
  const { me, error, loading, refresh } = useAuthGuard();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [pwdStatus, setPwdStatus] = useState<string | null>(null);
  const [pwdError, setPwdError] = useState<string | null>(null);
  const [changing, setChanging] = useState(false);
  const [name, setName] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [profileStatus, setProfileStatus] = useState<string | null>(null);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileSaving, setProfileSaving] = useState(false);
  const [newEmail, setNewEmail] = useState("");
  const [emailStatus, setEmailStatus] = useState<string | null>(null);
  const [emailError, setEmailError] = useState<string | null>(null);
  const [emailSending, setEmailSending] = useState(false);
  useEffect(() => {
    if (me) {
      setName(me.name || "");
      setAvatarUrl(me.avatarUrl || "");
    }
  }, [me]);

  const logout = () => {
    post("/api/logout")
      .catch(() => {
        /* ignore */
      })
      .finally(() => {
        router.replace("/login");
        router.refresh();
      });
  };

  const changePassword = async () => {
    setChanging(true);
    setPwdStatus(null);
    setPwdError(null);
    try {
      await post("/api/password", { currentPassword, newPassword });
      setPwdStatus("Пароль обновлён");
      setCurrentPassword("");
      setNewPassword("");
    } catch (err: any) {
      setPwdError(err?.message || "Ошибка смены пароля");
    } finally {
      setChanging(false);
    }
  };

  const saveProfile = async () => {
    setProfileSaving(true);
    setProfileError(null);
    setProfileStatus(null);
    try {
      await post("/api/profile", { name, avatarUrl });
      setProfileStatus("Профиль обновлён");
      refresh().catch(() => {});
    } catch (err: any) {
      setProfileError(err?.message || "Не удалось сохранить профиль");
    } finally {
      setProfileSaving(false);
    }
  };

  const requestEmailChange = async () => {
    setEmailSending(true);
    setEmailStatus(null);
    setEmailError(null);
    try {
      await post("/api/email/change/request", { newEmail });
      setEmailStatus("Письмо для смены почты отправлено");
    } catch (err: any) {
      setEmailError(err?.message || "Не удалось отправить письмо");
    } finally {
      setEmailSending(false);
    }
  };

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto">
          <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Профиль</h2>
        <button
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
          onClick={logout}
        >
          Выйти
        </button>
      </div>
      {error && <div className="text-red-400 text-sm">{error}</div>}
      {loading ? (
        <div className="text-slate-500 dark:text-slate-400">Загрузка...</div>
      ) : me ? (
        <div className="space-y-4">
          <div className="text-slate-800 dark:text-slate-300">Почта: {me.email}</div>

          <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
            <div className="text-sm font-semibold">Основные данные</div>
            <div className="grid sm:grid-cols-2 gap-3">
              <div className="space-y-1">
                <label className="text-xs text-slate-500 dark:text-slate-400">Имя</label>
                <input
                  className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Ваше имя"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs text-slate-500 dark:text-slate-400">Avatar URL</label>
                <input
                  className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={avatarUrl}
                  onChange={(e) => setAvatarUrl(e.target.value)}
                  placeholder="https://..."
                />
              </div>
            </div>
            {profileError && <div className="text-red-400 text-xs">{profileError}</div>}
            {profileStatus && <div className="text-emerald-500 text-xs">{profileStatus}</div>}
            <button
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-slate-900 text-white px-3 py-2 text-sm font-semibold hover:bg-slate-800 dark:bg-slate-700 dark:hover:bg-slate-600 disabled:opacity-50"
              onClick={saveProfile}
              disabled={profileSaving}
            >
              {profileSaving ? "Сохраняем..." : "Сохранить профиль"}
            </button>
          </div>

          <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
            <div className="text-sm font-semibold">Смена почты</div>
            <div className="space-y-1">
              <label className="text-xs text-slate-500 dark:text-slate-400">Новая почта</label>
              <input
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                type="email"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                placeholder="new@example.com"
              />
            </div>
            {emailError && <div className="text-red-400 text-xs">{emailError}</div>}
            {emailStatus && <div className="text-emerald-500 text-xs">{emailStatus}</div>}
            <button
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-slate-900 text-white px-3 py-2 text-sm font-semibold hover:bg-slate-800 dark:bg-slate-700 dark:hover:bg-slate-600 disabled:opacity-50"
              onClick={requestEmailChange}
              disabled={emailSending || !newEmail}
            >
              {emailSending ? "Отправляем..." : "Отправить подтверждение"}
            </button>
            <div className="text-xs text-slate-500 dark:text-slate-400">
              После перехода по ссылке из письма почта обновится, а сессия будет перевыпущена автоматически.
            </div>
          </div>

          <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
            <div className="text-sm font-semibold">Смена пароля</div>
            <div className="space-y-2">
              <div className="relative">
                <input
                  className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 pr-10 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  placeholder="Текущий пароль"
                  type={showCurrent ? "text" : "password"}
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                />
                <button
                  type="button"
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-500 dark:text-slate-400"
                  onClick={() => setShowCurrent((v) => !v)}
                >
                  {showCurrent ? <FiEyeOff /> : <FiEye />}
                </button>
              </div>
              <div className="relative">
                <input
                  className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 pr-10 text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  placeholder="Новый пароль"
                  type={showNew ? "text" : "password"}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
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
            {pwdError && <div className="text-red-400 text-xs">{pwdError}</div>}
            {pwdStatus && <div className="text-emerald-500 text-xs">{pwdStatus}</div>}
            <button
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
              onClick={changePassword}
              disabled={changing || !currentPassword || !newPassword}
            >
              {changing ? "Обновляем..." : "Обновить пароль"}
            </button>
          </div>

        </div>
      ) : (
        <div className="text-slate-500 dark:text-slate-400">Нет данных</div>
      )}
          </div>
        </div>
      </main>
    </div>
  );
}
