import "./globals.css";
import { ReactNode } from "react";
import { Navbar } from "../components/Navbar";
import { ToastHost } from "../components/ToastHost";

export const metadata = {
  title: {
    default: "SiteGen AI",
    template: "%s — SiteGen AI"
  },
  description:
    "Платформа для генерации, управления и публикации сайтов с поддержкой ИИ и расписаний.",
  robots: {
    index: false,
    follow: false
  },
  openGraph: {
    title: "SiteGen AI",
    description:
      "Платформа для генерации, управления и публикации сайтов с поддержкой ИИ и расписаний.",
    type: "website"
  },
  twitter: {
    card: "summary_large_image",
    title: "SiteGen AI",
    description:
      "Платформа для генерации, управления и публикации сайтов с поддержкой ИИ и расписаний."
  }
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ru">
      <body className="min-h-screen flex justify-center px-4 bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-50">
        <div className="w-full mx-auto py-10 space-y-8">
          <Navbar />
          <header className="space-y-2">
            <h1 className="text-2xl font-bold">SiteGen AI</h1>
            <p className="text-sm muted">
              Генерация и управление сайтами с поддержкой ИИ, расписаний и ссылок.
            </p>
          </header>
          {children}
        </div>
        <ToastHost />
      </body>
    </html>
  );
}
