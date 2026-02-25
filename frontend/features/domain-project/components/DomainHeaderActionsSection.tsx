import Link from "next/link";
import type { UrlObject } from "url";
import type { ReactNode } from "react";
import { FiEdit3, FiPause, FiPlay, FiRefreshCw, FiX } from "react-icons/fi";
import { DOMAIN_PROJECT_CTA } from "../services/statusCta";
import { ActionFlowBanner } from "./ActionFlowBanner";
import type { FlowState } from "../hooks/useFlowState";

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
  editorHref: UrlObject;
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
  domain,
  projectName,
  error,
  currentAttempt,
  mainButtonText,
  mainButtonIcon,
  mainButtonDisabled,
  loading,
  canOpenEditor,
  editorHref,
  generationFlow,
  linkFlow,
  renderStatusBadge,
  onMainAction,
  onResumeGeneration,
  onPauseGeneration,
  onCancelGeneration,
  onRefresh
}: DomainHeaderActionsSectionProps) {
  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">Домен</h1>
          {domain && (
            <>
              <div className="mt-1 text-lg font-semibold">{domain.url}</div>
              <div className="text-sm text-slate-500 dark:text-slate-400">
                Проект: {projectName || "—"} · Статус: {renderStatusBadge(domain.status)}
              </div>
              <div className="text-sm text-slate-500 dark:text-slate-400 mt-1">Ключевое слово: {domain.main_keyword || "—"}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400">
                Сервер: {domain.server_id || "—"} · Страна: {domain.target_country || "—"} · Язык: {domain.target_language || "—"}
              </div>
              {domain.exclude_domains && (
                <div className="text-xs text-slate-500 dark:text-slate-400">Исключить: {domain.exclude_domains}</div>
              )}
              <div className="text-xs text-slate-500 dark:text-slate-400">
                Обновлено: {domain.updated_at ? new Date(domain.updated_at).toLocaleString() : "—"}
              </div>
            </>
          )}
        </div>
        <div className="flex gap-2">
          <button
            onClick={onMainAction}
            disabled={mainButtonDisabled}
            className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          >
            {mainButtonIcon} {mainButtonText}
          </button>
          {currentAttempt && (
            <>
              {currentAttempt.status === "paused" && (
                <button
                  onClick={() => onResumeGeneration(currentAttempt.id)}
                  disabled={loading}
                  className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300 disabled:opacity-50"
                >
                  <FiPlay /> {DOMAIN_PROJECT_CTA.generationResume}
                </button>
              )}
              {(currentAttempt.status === "pending" ||
                currentAttempt.status === "processing" ||
                currentAttempt.status === "pause_requested" ||
                currentAttempt.status === "cancelling") && (
                <>
                  {currentAttempt.status !== "cancelling" && (
                    <button
                      onClick={() => onPauseGeneration(currentAttempt.id)}
                      disabled={loading || currentAttempt.status === "pause_requested"}
                      className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:bg-slate-800 dark:text-amber-300 disabled:opacity-50"
                    >
                      <FiPause />{" "}
                      {currentAttempt.status === "pause_requested"
                        ? DOMAIN_PROJECT_CTA.generationPauseRequested
                        : DOMAIN_PROJECT_CTA.generationPause}
                    </button>
                  )}
                  <button
                    onClick={() => onCancelGeneration(currentAttempt.id)}
                    disabled={loading || currentAttempt.status === "cancelling"}
                    className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
                  >
                    <FiX />{" "}
                    {currentAttempt.status === "cancelling"
                      ? DOMAIN_PROJECT_CTA.generationCancelling
                      : DOMAIN_PROJECT_CTA.generationCancel}
                  </button>
                </>
              )}
              {currentAttempt.status === "cancelled" && (
                <button
                  onClick={() => onCancelGeneration(currentAttempt.id)}
                  disabled={loading}
                  className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
                >
                  <FiX /> {DOMAIN_PROJECT_CTA.generationCancel}
                </button>
              )}
            </>
          )}
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить
          </button>
          {canOpenEditor ? (
            <Link
              href={editorHref}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiEdit3 /> Открыть в редакторе
            </Link>
          ) : (
            <span
              title="Редактор доступен после публикации и синхронизации файлов"
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-sm font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
            >
              <FiEdit3 /> Открыть в редакторе
            </span>
          )}
          {domain?.project_id ? (
            <Link
              href={`/projects/${domain.project_id}`}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            >
              ← К проекту
            </Link>
          ) : (
            <button
              disabled
              className="inline-flex items-center gap-2 rounded-lg bg-slate-300 px-3 py-2 text-sm font-semibold text-slate-600 cursor-not-allowed"
            >
              ← К проекту
            </button>
          )}
        </div>
      </div>
      {error && <div className="mt-2 text-red-500 text-sm">{error}</div>}
      <div className="mt-3 space-y-2">
        <ActionFlowBanner title="Генерация" flow={generationFlow} />
        <ActionFlowBanner title="Ссылки" flow={linkFlow} />
      </div>
    </div>
  );
}

