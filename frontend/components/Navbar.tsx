"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import {
  FiLogIn,
  FiUser,
  FiMoon,
  FiSun,
  FiUserPlus,
  FiGrid,
  FiShield,
  FiClock,
  FiActivity,
  FiChevronDown
} from "react-icons/fi";
import { useTheme } from "../lib/useTheme";
import { apiBase, post } from "../lib/http";

type UserState = {
  email: string | null;
  role?: string | null;
};

export function Navbar() {
  const { theme, toggle } = useTheme();
  const [user, setUser] = useState<UserState>({ email: null });
  const [monitoringOpen, setMonitoringOpen] = useState(false);
  const monitoringRef = useRef<HTMLDivElement | null>(null);
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

  useEffect(() => {
    setMonitoringOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!monitoringOpen) {
      return;
    }
    const onClick = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (monitoringRef.current && target && monitoringRef.current.contains(target)) {
        return;
      }
      setMonitoringOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMonitoringOpen(false);
      }
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [monitoringOpen]);

  const logout = async () => {
    await post("/api/logout").catch(() => {});
    setUser({ email: null });
    window.location.href = "/login";
  };

  return (
    <div className="border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/80 rounded-xl px-4 py-3 flex items-center justify-between backdrop-blur">
      <Link href="/" className="flex items-center gap-3">
        <img
          src="/brand.svg"
          alt="SiteGen AI"
          className="h-8 w-auto select-none dark:invert"
        />
      </Link>
      <div className="flex items-center gap-2">
        <Link
          href="/queue"
          className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
          <FiClock /> Очередь
        </Link>
        {user.email ? (
          <>
            <Link
              href="/projects"
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100">
              <FiGrid /> Проекты
            </Link>
            {user.role && user.role.toLowerCase() === "admin" && (
              <>
                <div className="relative" ref={monitoringRef}>
                  <button
                    type="button"
                    aria-expanded={monitoringOpen}
                    onClick={() => setMonitoringOpen((prev) => !prev)}
                    className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiActivity /> Мониторинг <FiChevronDown className="text-xs" />
                  </button>
                  {monitoringOpen && (
                    <div className="absolute left-0 top-full pt-2 min-w-[180px]">
                      <div className="rounded-xl border border-slate-200 bg-white shadow-lg dark:border-slate-800 dark:bg-slate-900">
                        <Link
                          href={{ pathname: "/monitoring/indexing" }}
                          className="block px-3 py-2 text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-100 dark:hover:bg-slate-800"
                        >
                          Индексация
                        </Link>
                        <Link
                          href={{ pathname: "/monitoring/llm-usage" }}
                          className="block px-3 py-2 text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-100 dark:hover:bg-slate-800"
                        >
                          LLM Usage
                        </Link>
                      </div>
                    </div>
                  )}
                </div>
                <Link
                  href="/admin"
                  className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:border-indigo-500/50 dark:bg-indigo-500/10 dark:text-indigo-100">
                  <FiShield /> Админ
                </Link>
              </>
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
