import Link from "next/link";
import type { UrlObject } from "url";
import type { ReactNode } from "react";
import { FiActivity, FiAlertCircle, FiDollarSign, FiKey, FiList, FiRefreshCw, FiTrash2 } from "react-icons/fi";
import { ActionFlowBanner } from "./ActionFlowBanner";
import type { FlowState } from "../hooks/useFlowState";

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
};

export function ProjectHeaderActionsSection({
  project,
  projectId,
  loading,
  error,
  generationFlow,
  linkFlow,
  onRefresh,
  onDeleteProject
}: ProjectHeaderActionsSectionProps) {
  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">{project?.name || "Проект"}</h2>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Страна: {project?.target_country || "—"} · Язык: {project?.target_language || "—"}
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            href={projectId ? `/projects/${projectId}/queue?tab=domains` : "/projects"}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiList /> Очередь проекта
          </Link>
          {projectId && (
            <Link
              href={{ pathname: "/monitoring/indexing", query: { projectId } } as UrlObject}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiActivity /> Индексация
            </Link>
          )}
          {projectId && (
            <Link
              href={{ pathname: `/projects/${projectId}/usage` } as UrlObject}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiDollarSign /> LLM Usage
            </Link>
          )}
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить
          </button>
          <button
            onClick={onDeleteProject}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
          >
            <FiTrash2 /> Удалить
          </button>
        </div>
      </div>
      {error && <div className="text-red-500 text-sm mt-2">{error}</div>}
      <div className="mt-3 space-y-2">
        <ActionFlowBanner title="Операции доменов" flow={generationFlow} />
        <ActionFlowBanner title="Операции ссылок" flow={linkFlow} />
      </div>

      {project && (
        <div className="mt-4">
          {project.ownerHasApiKey === false ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20 p-3">
              <div className="flex items-start gap-2">
                <FiAlertCircle className="text-amber-600 dark:text-amber-400 mt-0.5" />
                <div className="flex-1">
                  <div className="text-sm font-semibold text-amber-800 dark:text-amber-200">
                    ⚠️ API ключ не настроен
                  </div>
                  <div className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                    Генерация не будет работать без API ключа владельца проекта.{" "}
                    <a href="/me" className="underline hover:no-underline">
                      Настроить в профиле →
                    </a>
                  </div>
                </div>
              </div>
            </div>
          ) : project.ownerHasApiKey === true ? (
            <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/40 p-3">
              <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                <FiKey className="text-emerald-600 dark:text-emerald-400" />
                <span>API ключ настроен. Генерация будет использовать ключ владельца проекта.</span>
              </div>
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

