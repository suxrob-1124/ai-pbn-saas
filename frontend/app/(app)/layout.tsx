'use client';

import { ReactNode, useState, useEffect } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuthGuard } from '@/lib/useAuth';
import {
  FolderGit2,
  Settings,
  Activity,
  ListChecks,
  ShieldAlert,
  Database,
  TerminalSquare,
  LogOut,
  AlertTriangle,
  BookOpen,
  PanelLeftClose,
  PanelLeftOpen,
} from 'lucide-react';
import { post } from '@/lib/http';
import type { ComponentProps } from 'react';
import { ThemeToggle } from '@/components/ThemeToggle';

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { me, loading } = useAuthGuard();
  const pathname = usePathname();
  const router = useRouter();

  // Состояние свернутости сайдбара
  const [isCollapsed, setIsCollapsed] = useState(false);

  // Сохраняем состояние в localStorage, чтобы оно не сбрасывалось при F5
  useEffect(() => {
    const saved = localStorage.getItem('sidebar_collapsed');
    if (saved === 'true') setIsCollapsed(true);
  }, []);

  const toggleSidebar = () => {
    const newState = !isCollapsed;
    setIsCollapsed(newState);
    localStorage.setItem('sidebar_collapsed', String(newState));
  };

  const handleLogout = () => {
    post('/api/logout')
      .catch(() => {})
      .finally(() => {
        router.replace('/login');
      });
  };

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-50 dark:bg-slate-950 text-slate-500">
        Загрузка приложения...
      </div>
    );
  }

  const isEditor = me?.role === 'editor';
  const isAdmin = me?.role === 'admin';
  const isOwner = me?.role === 'owner';
  const hasFullAccess = isAdmin || isOwner || me?.role === 'manager';
  const showApiKeyAlert = hasFullAccess && me?.hasApiKey === false;

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50 dark:bg-slate-950">
      {/* ЛЕВЫЙ САЙДБАР */}
      <aside
        className={`${isCollapsed ? 'w-[72px]' : 'w-64'} flex-shrink-0 border-r border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950 flex flex-col z-20 transition-all duration-300 ease-in-out relative`}>
        {/* Шапка Сайдбара */}
        <div className="h-14 flex items-center justify-between px-4 border-b border-slate-200 dark:border-slate-800 flex-shrink-0">
          {!isCollapsed && (
            <span className="font-bold text-lg tracking-tight text-slate-900 dark:text-white truncate">
              SiteGen AI
            </span>
          )}
          {/* Кнопка скрытия (Если свернуто — выравниваем по центру) */}
          <button
            onClick={toggleSidebar}
            className={`p-1.5 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 dark:hover:text-slate-200 dark:hover:bg-slate-800 transition-colors ${isCollapsed ? 'mx-auto' : ''}`}
            title={isCollapsed ? 'Развернуть меню' : 'Свернуть меню'}>
            {isCollapsed ? (
              <PanelLeftOpen className="w-5 h-5" />
            ) : (
              <PanelLeftClose className="w-5 h-5" />
            )}
          </button>
        </div>

        {/* Навигация */}
        <nav className="flex-1 overflow-y-auto overflow-x-hidden p-3 space-y-6 scrollbar-hide">
          <div>
            {!isCollapsed && (
              <p className="px-3 text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2 mt-2">
                Рабочее пространство
              </p>
            )}
            <div className="space-y-1">
              <NavItem
                href="/projects"
                icon={<FolderGit2 className="w-4 h-4" />}
                label={isEditor ? 'Мои сайты' : 'Проекты'}
                currentPath={pathname}
                isCollapsed={isCollapsed}
              />
            </div>
          </div>

          {hasFullAccess && (
            <>
              <div>
                {!isCollapsed && (
                  <p className="px-3 text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">
                    Очереди
                  </p>
                )}
                <div className="space-y-1">
                  <NavItem
                    href="/queue?tab=domains"
                    icon={<ListChecks className="w-4 h-4" />}
                    label="Глобальная очередь"
                    currentPath={pathname}
                    isCollapsed={isCollapsed}
                  />
                  <NavItem
                    href="/queue?tab=links"
                    icon={<Database className="w-4 h-4" />}
                    label="Очередь ссылок"
                    currentPath={pathname}
                    isCollapsed={isCollapsed}
                  />
                </div>
              </div>

              <div>
                {!isCollapsed && (
                  <p className="px-3 text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">
                    Мониторинг
                  </p>
                )}
                <div className="space-y-1">
                  <NavItem
                    href="/monitoring/indexing"
                    icon={<Activity className="w-4 h-4" />}
                    label="Индексация"
                    currentPath={pathname}
                    isCollapsed={isCollapsed}
                  />
                  <NavItem
                    href="/monitoring/llm-usage"
                    icon={<TerminalSquare className="w-4 h-4" />}
                    label="LLM Usage"
                    currentPath={pathname}
                    isCollapsed={isCollapsed}
                  />
                  <NavItem
                    href="/monitoring"
                    icon={<Settings className="w-4 h-4" />}
                    label="Статус систем"
                    currentPath={pathname}
                    isCollapsed={isCollapsed}
                  />
                </div>
              </div>
            </>
          )}

          {isAdmin && (
            <div>
              {!isCollapsed && (
                <p className="px-3 text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-2">
                  Система
                </p>
              )}
              <div className="space-y-1">
                <NavItem
                  href="/admin"
                  icon={<ShieldAlert className="w-4 h-4" />}
                  label="Админ-панель"
                  currentPath={pathname}
                  isCollapsed={isCollapsed}
                />
              </div>
            </div>
          )}
        </nav>

        {/* Подвал Сайдбара (Доки, Тема, Профиль) */}
        <div className="border-t border-slate-200 dark:border-slate-800 p-3 flex flex-col gap-1">
          {/* Документация */}
          <Link
            href="/docs"
            target="_blank"
            className={`flex items-center rounded-lg text-slate-500 hover:text-slate-900 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-white dark:hover:bg-slate-800 transition-colors ${isCollapsed ? 'justify-center p-2.5' : 'gap-3 px-3 py-2 text-sm font-medium'}`}
            title={isCollapsed ? 'Документация' : undefined}>
            <BookOpen className="w-4 h-4 flex-shrink-0" />
            {!isCollapsed && <span className="truncate">Документация</span>}
          </Link>

          <div className="w-full h-px bg-slate-100 dark:bg-slate-800 my-1"></div>

          {/* Профиль и Тема */}
          <div
            className={`flex ${isCollapsed ? 'flex-col items-center gap-3 py-2' : 'items-center justify-between px-1'}`}>
            <Link
              href="/me"
              className="flex items-center gap-3 hover:opacity-80 transition-opacity min-w-0"
              title={isCollapsed ? 'Профиль' : undefined}>
              <div className="w-8 h-8 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center font-bold flex-shrink-0 text-sm">
                {me?.email?.charAt(0).toUpperCase()}
              </div>
              {!isCollapsed && (
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium truncate text-slate-900 dark:text-slate-200">
                    {me?.name || me?.email?.split('@')[0]}
                  </p>
                  <p className="text-[10px] text-slate-500 uppercase tracking-wider truncate">
                    {me?.role}
                  </p>
                </div>
              )}
            </Link>

            <div className={`flex items-center gap-1 ${isCollapsed ? 'flex-col' : ''}`}>
              <ThemeToggle />
              <button
                onClick={handleLogout}
                className="p-2 rounded-lg text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
                title="Выйти">
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>
      </aside>

      {/* ГЛАВНАЯ ОБЛАСТЬ */}
      <div className="flex-1 flex flex-col min-w-0 bg-slate-50 dark:bg-[#080b13]">
        {showApiKeyAlert && (
          <div className="bg-amber-50 dark:bg-amber-900/30 border-b border-amber-200 dark:border-amber-800 px-6 py-2.5 flex items-center justify-between z-20 shadow-sm">
            <div className="flex items-center gap-2 text-sm text-amber-800 dark:text-amber-200">
              <AlertTriangle className="w-4 h-4" />
              <span>
                <strong>Внимание:</strong> Для генерации сайтов необходим API-ключ Gemini.
              </span>
            </div>
            <Link
              href="/me"
              className="text-xs font-bold px-3 py-1 bg-amber-200 text-amber-800 dark:bg-amber-800 dark:text-amber-100 rounded-md hover:bg-amber-300 transition-colors">
              Настроить
            </Link>
          </div>
        )}
        <main className="flex-1 overflow-y-auto relative">{children}</main>
      </div>
    </div>
  );
}

// Вытаскиваем точный тип href из Next.js Link
type LinkHref = ComponentProps<typeof Link>['href'];

function NavItem({
  href,
  icon,
  label,
  currentPath,
  isCollapsed,
}: {
  href: LinkHref;
  icon: ReactNode;
  label: string;
  currentPath: string;
  isCollapsed: boolean;
}) {
  const hrefString = typeof href === 'string' ? href : href.pathname || '';
  let isActive = false;

  if (hrefString === '/projects') {
    isActive = currentPath === '/projects' || currentPath.startsWith('/projects/');
  } else if (hrefString === '/queue?tab=domains' || hrefString === '/queue?tab=links') {
    const baseHref = hrefString.split('?')[0];
    isActive = currentPath === baseHref;
  } else {
    isActive = currentPath === hrefString;
  }

  return (
    <Link
      href={href}
      title={isCollapsed ? label : undefined}
      className={`flex items-center rounded-xl transition-all font-medium ${
        isCollapsed ? 'justify-center p-2.5' : 'gap-3 px-3 py-2 text-sm'
      } ${
        isActive
          ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-400'
          : 'text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:text-slate-200 dark:hover:bg-slate-800/50'
      }`}>
      <span className="flex-shrink-0">{icon}</span>
      {!isCollapsed && <span className="truncate">{label}</span>}
    </Link>
  );
}
