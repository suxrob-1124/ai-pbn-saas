"use client";

import "./globals.css";
import { ReactNode } from "react";
import { Navbar } from "../components/Navbar";
import { ToastHost } from "../components/ToastHost";

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ru">
      <body className="min-h-screen flex justify-center px-4 bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-50">
        <div className="w-full max-w-6xl mx-auto py-10 space-y-8">
          <Navbar />
          <header className="space-y-2">
            <h1 className="text-2xl font-bold">Панель управления</h1>
            <p className="text-sm muted">
              Базовая админка: аутентификация, профиль, заготовки под проекты и мониторинг.
            </p>
          </header>
          {children}
        </div>
        <ToastHost />
      </body>
    </html>
  );
}
