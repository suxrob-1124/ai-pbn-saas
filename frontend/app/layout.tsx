import './globals.css';
import { ReactNode } from 'react';
import { ToastHost } from '@/components/ToastHost';

export const metadata = {
  title: {
    default: 'SiteGen AI',
    template: '%s — SiteGen AI',
  },
  description: 'Платформа для генерации и управления сайтами.',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <head>
        {/* Синхронный скрипт: применяет тему ДО первого рендера, без мигания */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('ui-theme');if(t==='dark'||(!t)){document.documentElement.classList.add('dark')}}catch(e){document.documentElement.classList.add('dark')}})()`,
          }}
        />
      </head>
      <body className="bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-50 antialiased">
        {children}
        <ToastHost />
      </body>
    </html>
  );
}
