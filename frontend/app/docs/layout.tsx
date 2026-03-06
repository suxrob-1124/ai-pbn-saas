import { ReactNode } from 'react';
import { DocsSidebar } from '@/components/DocsSidebar';

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="max-w-7xl mx-auto flex flex-col lg:flex-row gap-8 items-start animate-in fade-in duration-500">
      {/* Левый сайдбар документации (Прилипает при скролле) */}
      <div className="w-full lg:w-64 flex-shrink-0 lg:sticky lg:top-6">
        <DocsSidebar />
      </div>

      {/* Основной контент статьи */}
      <div className="flex-1 w-full rounded-3xl border border-slate-200 dark:border-slate-700/60 bg-white dark:bg-[#0f1523] p-8 md:p-12 shadow-sm">
        {children}
      </div>
    </div>
  );
}
