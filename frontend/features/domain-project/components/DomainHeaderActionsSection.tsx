import type { ReactNode } from 'react';
import { Play, Pause, X, RefreshCw, Loader2 } from 'lucide-react';
import { DOMAIN_PROJECT_CTA } from '../services/statusCta';
import { ActionFlowBanner } from './ActionFlowBanner';
import type { FlowState } from '../hooks/useFlowState';

type DomainHeaderDomain = {
  url: string;
  status: string;
  main_keyword?: string;
  server_id?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  updated_at?: string;
  project_id?: string;
};

type AttemptState = {
  id: string;
  status: string;
} | null;

type DomainHeaderActionsSectionProps = {
  domain: DomainHeaderDomain | null;
  projectName: string;
  error: string | null;
  currentAttempt: AttemptState;
  mainButtonText: string;
  mainButtonIcon: ReactNode;
  mainButtonDisabled: boolean;
  loading: boolean;
  canOpenEditor: boolean;
  editorHref: any; // UrlObject
  generationFlow: FlowState;
  linkFlow: FlowState;
  renderStatusBadge: (status: string) => ReactNode;
  onMainAction: () => void;
  onResumeGeneration: (generationId: string) => void;
  onPauseGeneration: (generationId: string) => void;
  onCancelGeneration: (generationId: string) => void;
  onRefresh: () => void;
};

export function DomainHeaderActionsSection({
  currentAttempt,
  mainButtonText,
  mainButtonDisabled,
  loading,
  generationFlow,
  linkFlow,
  onMainAction,
  onResumeGeneration,
  onPauseGeneration,
  onCancelGeneration,
  onRefresh,
}: DomainHeaderActionsSectionProps) {
  // Умное скрытие баннеров
  const showGenBanner = generationFlow.status !== 'idle' && generationFlow.status !== 'done';
  const showLinkBanner = linkFlow.status !== 'idle' && linkFlow.status !== 'done';

  return (
    <div className="flex items-center gap-2">
      {/* ГЛАВНАЯ КНОПКА ГЕНЕРАЦИИ */}
      <button
        onClick={onMainAction}
        disabled={mainButtonDisabled || loading}
        className="inline-flex items-center gap-2 rounded-xl bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50 transition-all shadow-sm active:scale-95">
        <Play className="w-4 h-4 fill-current" /> {mainButtonText}
      </button>

      {/* КНОПКИ УПРАВЛЕНИЯ ПАЙПЛАЙНОМ */}
      {currentAttempt && (
        <>
          {currentAttempt.status === 'paused' && (
            <button
              onClick={() => onResumeGeneration(currentAttempt.id)}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2.5 text-sm font-semibold text-emerald-700 hover:bg-emerald-100 dark:border-emerald-900/50 dark:bg-emerald-500/10 dark:text-emerald-400 transition-colors">
              <Play className="w-4 h-4" /> {DOMAIN_PROJECT_CTA.generationResume}
            </button>
          )}

          {(currentAttempt.status === 'pending' ||
            currentAttempt.status === 'processing' ||
            currentAttempt.status === 'pause_requested' ||
            currentAttempt.status === 'cancelling') && (
            <>
              {currentAttempt.status !== 'cancelling' && (
                <button
                  onClick={() => onPauseGeneration(currentAttempt.id)}
                  disabled={loading || currentAttempt.status === 'pause_requested'}
                  className="inline-flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm font-semibold text-amber-700 hover:bg-amber-100 dark:border-amber-900/50 dark:bg-amber-500/10 dark:text-amber-400 transition-colors">
                  <Pause className="w-4 h-4" />
                  {currentAttempt.status === 'pause_requested'
                    ? 'Остановка...'
                    : DOMAIN_PROJECT_CTA.generationPause}
                </button>
              )}
              <button
                onClick={() => onCancelGeneration(currentAttempt.id)}
                disabled={loading || currentAttempt.status === 'cancelling'}
                className="inline-flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-3 py-2.5 text-sm font-semibold text-red-700 hover:bg-red-100 dark:border-red-900/50 dark:bg-red-500/10 dark:text-red-400 transition-colors">
                <X className="w-4 h-4" />
                {currentAttempt.status === 'cancelling'
                  ? 'Отмена...'
                  : DOMAIN_PROJECT_CTA.generationCancel}
              </button>
            </>
          )}
        </>
      )}

      {/* КНОПКА ОБНОВЛЕНИЯ СТРАНИЦЫ */}
      <button
        onClick={onRefresh}
        disabled={loading}
        className="inline-flex items-center justify-center p-2.5 rounded-xl border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-300 dark:hover:bg-slate-800 transition-all"
        title="Обновить данные">
        {loading ? (
          <Loader2 className="w-4 h-4 animate-spin text-slate-400" />
        ) : (
          <RefreshCw className="w-4 h-4" />
        )}
      </button>

      {/* ПЛАВАЮЩИЕ УВЕДОМЛЕНИЯ О ПРОЦЕССЕ */}
      {(showGenBanner || showLinkBanner) && (
        <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2">
          {showGenBanner && <ActionFlowBanner title="Генерация" flow={generationFlow} />}
          {showLinkBanner && <ActionFlowBanner title="Ссылки" flow={linkFlow} />}
        </div>
      )}
    </div>
  );
}
