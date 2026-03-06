'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { LogIn, UserPlus, Sun, Moon, ArrowRight } from 'lucide-react';
import { useTheme } from '@/lib/useTheme';
import { apiBase } from '@/lib/http';

type UserState = {
  email: string | null;
};

export function Navbar() {
  const { theme, toggle } = useTheme();
  const [user, setUser] = useState<UserState>({ email: null });

  useEffect(() => {
    const fetchMe = async () => {
      try {
        const res = await fetch(`${apiBase()}/api/me`, { credentials: 'include' });
        if (!res.ok) {
          setUser({ email: null });
          return;
        }
        const data = await res.json();
        if (data?.email) {
          setUser({ email: data.email });
        } else {
          setUser({ email: null });
        }
      } catch {
        /* ignore */
      }
    };
    fetchMe();
  }, []);

  return (
    <div className="border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/80 rounded-2xl px-6 py-3 flex items-center justify-between backdrop-blur-md shadow-sm">
      <Link href="/" className="flex items-center gap-3">
        <img src="/brand.svg" alt="SiteGen AI" className="h-8 w-auto select-none dark:invert" />
        {/* Опционально: текстовое лого, если brand.svg это только иконка */}
        {/* <span className="font-bold text-lg hidden sm:block">SiteGen AI</span> */}
      </Link>

      <div className="flex items-center gap-3">
        {user.email ? (
          <Link
            href="/projects"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-indigo-600 text-white hover:bg-indigo-500 font-medium text-sm transition-colors">
            В панель управления <ArrowRight className="w-4 h-4" />
          </Link>
        ) : (
          <>
            <Link
              href="/login"
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800 font-medium text-sm transition-colors">
              <LogIn className="w-4 h-4" /> Войти
            </Link>
            <Link
              href="/register"
              className="hidden sm:inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-900 text-white hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100 font-medium text-sm transition-colors">
              <UserPlus className="w-4 h-4" /> Создать аккаунт
            </Link>
          </>
        )}

        <div className="w-px h-6 bg-slate-200 dark:bg-slate-700 mx-1"></div>

        <button
          className="inline-flex items-center justify-center w-10 h-10 rounded-lg text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800 transition-colors"
          onClick={toggle}
          type="button"
          aria-label="Переключить тему">
          {theme === 'dark' ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
        </button>
      </div>
    </div>
  );
}
