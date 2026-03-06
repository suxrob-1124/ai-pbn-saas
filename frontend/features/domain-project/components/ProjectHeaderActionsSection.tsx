import Link from 'next/link';
import type { UrlObject } from 'url';
import { List, Activity, DollarSign, Trash2, AlertTriangle, UploadCloud } from 'lucide-react';
import { ActionFlowBanner } from './ActionFlowBanner';
import type { FlowState } from '../hooks/useFlowState';

type ProjectHeaderProject = {
  id: string;
  name: string;
  target_country?: string;
  target_language?: string;
  ownerHasApiKey?: boolean;
};

type ProjectHeaderActionsSectionProps = {
  project: ProjectHeaderProject | null;
  projectId: string;
  loading: boolean;
  error: string | null;
  generationFlow: FlowState;
  linkFlow: FlowState;
  onRefresh: () => void;
  onDeleteProject: () => void;
  onLegacyImport?: () => void;
};

export function ProjectHeaderActionsSection({
  project,
  projectId,
  loading,
  generationFlow,
  linkFlow,
  onDeleteProject,
  onLegacyImport,
}: ProjectHeaderActionsSectionProps) {
  return (
    <div className="flex items-center gap-2 relative">
      {/* Очередь */}
      <Link
        href={projectId ? `/projects/${projectId}/queue?tab=domains` : '/projects'}
        className="inline-flex items-center gap-1.5 px-3 py-2.5 rounded-xl bg-white border border-slate-200 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 transition-colors shadow-sm">
        <List className="w-4 h-4 text-slate-500" />
        <span className="hidden xl:inline">Очередь</span>
      </Link>

      {/* Индексация */}
      {projectId && (
        <Link
          href={{ pathname: '/monitoring/indexing', query: { projectId } } as UrlObject}
          className="inline-flex items-center gap-1.5 px-3 py-2.5 rounded-xl bg-white border border-slate-200 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 transition-colors shadow-sm">
          <Activity className="w-4 h-4 text-slate-500" />
          <span className="hidden xl:inline">Индексация</span>
        </Link>
      )}

      {/* LLM Usage */}
      {projectId && (
        <Link
          href={{ pathname: `/projects/${projectId}/usage` } as UrlObject}
          className="inline-flex items-center gap-1.5 px-3 py-2.5 rounded-xl bg-white border border-slate-200 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 transition-colors shadow-sm">
          <DollarSign className="w-4 h-4 text-slate-500" />
          <span className="hidden xl:inline">LLM Usage</span>
        </Link>
      )}

      {/* Legacy Import */}
      {onLegacyImport && (
        <button
          onClick={onLegacyImport}
          disabled={loading}
          className="inline-flex items-center gap-1.5 px-3 py-2.5 rounded-xl bg-white border border-slate-200 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 transition-colors shadow-sm">
          <UploadCloud className="w-4 h-4 text-slate-500" />
          <span className="hidden xl:inline">Legacy Import</span>
        </button>
      )}

      {/* Удалить */}
      <button
        onClick={onDeleteProject}
        disabled={loading}
        className="inline-flex items-center gap-1.5 px-3 py-2.5 rounded-xl border border-red-200 bg-red-50 text-sm font-medium text-red-600 hover:bg-red-100 disabled:opacity-50 dark:bg-red-900/20 dark:border-red-800/50 dark:text-red-400 dark:hover:bg-red-900/40 transition-colors shadow-sm">
        <Trash2 className="w-4 h-4" />
        <span className="hidden xl:inline">Удалить</span>
      </button>

      {/* Всплывающие уведомления (Banners), выведенные из потока шапки */}
      <div className="absolute top-[120%] right-0 w-80 flex flex-col gap-2 pointer-events-none z-50">
        {/* Окно прогресса операций (появляется только когда что-то происходит) */}
        <div className="pointer-events-auto">
          <ActionFlowBanner title="Генерация" flow={generationFlow} />
        </div>
        <div className="pointer-events-auto">
          <ActionFlowBanner title="Ссылки" flow={linkFlow} />
        </div>

        {/* Предупреждение о ключе (если его нет у владельца) */}
        {project && project.ownerHasApiKey === false && (
          <div className="pointer-events-auto rounded-xl border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-[#1a1305] p-4 shadow-xl">
            <div className="flex items-start gap-3">
              <AlertTriangle className="w-5 h-5 text-amber-500 mt-0.5 flex-shrink-0" />
              <div className="flex-1">
                <div className="text-sm font-bold text-amber-800 dark:text-amber-300">
                  API ключ не настроен
                </div>
                <div className="text-xs text-amber-700 dark:text-amber-400/80 mt-1.5 leading-relaxed">
                  Генерация не будет работать без ключа Gemini у владельца проекта.{' '}
                  <Link href="/me" className="underline hover:text-amber-500 font-medium">
                    Настроить в профиле
                  </Link>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
