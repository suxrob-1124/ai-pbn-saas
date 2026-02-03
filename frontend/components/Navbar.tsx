"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { FiLogIn, FiUser, FiMoon, FiSun, FiUserPlus, FiGrid, FiShield } from "react-icons/fi";
import { useTheme } from "../lib/useTheme";
import { apiBase, post } from "../lib/http";

type UserState = {
  email: string | null;
  role?: string | null;
};

export function Navbar() {
  const { theme, toggle } = useTheme();
  const [user, setUser] = useState<UserState>({ email: null });
  const pathname = usePathname();

  useEffect(() => {
    const fetchMe = async () => {
      try {
        const res = await fetch(`${apiBase()}/api/me`, { credentials: "include" });
        if (!res.ok) {
          setUser({ email: null });
          return;
        }
        const data = await res.json();
        if (data?.email) {
          setUser({ email: data.email, role: data.role });
        } else {
          setUser({ email: null });
        }
      } catch {
        /* ignore */
      }
    };
    fetchMe();
  }, [pathname]);

  const logout = async () => {
    await post("/api/logout").catch(() => {});
    setUser({ email: null });
    window.location.href = "/login";
  };

  return (
    <div className="border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/80 rounded-xl px-4 py-3 flex items-center justify-between backdrop-blur">
      <div className="flex items-center gap-2 font-semibold text-lg">
        <div className="h-8 w-8 rounded-lg bg-indigo-600 text-white flex items-center justify-center font-bold">
          A
        </div>
        <span>Control Panel</span>
      </div>
      <div className="flex items-center gap-2">
        {user.email ? (
          <>
            <Link
              href="/projects"
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
              <FiGrid /> Проекты
            </Link>
            {user.role && user.role.toLowerCase() === "admin" && (
              <Link
                href="/admin"
                className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:border-indigo-500/50 dark:bg-indigo-500/10 dark:text-indigo-100">
                <FiShield /> Админ
              </Link>
            )}
            {/* <button
              onClick={logout}
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-red-200 bg-red-50 text-red-700 hover:bg-red-100 dark:border-red-800 dark:bg-red-900/40 dark:text-red-100"
              type="button">
              Выйти
            </button> */}
            <Link
              href="/me"
              className="inline-flex items-center gap-2 px-3 py-3 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
              <FiUser />
              {/* Профиль */}
            </Link>
          </>
        ) : (
          <>
            <Link
              href="/login"
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
              <FiLogIn /> Войти
            </Link>
            <Link
              href="/register"
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
              <FiUserPlus /> Регистрация
            </Link>
          </>
        )}
        <button
          className="inline-flex items-center justify-center px-3 py-3 rounded-lg border border-slate-200 bg-slate-100 text-slate-800 hover:bg-slate-200 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700"
          onClick={toggle}
          type="button"
          aria-label="Переключить тему">
          {theme === 'dark' ? <FiSun /> : <FiMoon />}
        </button>
      </div>
    </div>
  );
}
