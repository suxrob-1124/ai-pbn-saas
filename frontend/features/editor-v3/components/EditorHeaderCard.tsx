"use client";

import Link from "next/link";
import { FiArrowLeft, FiFolder } from "react-icons/fi";

import type { DomainSummaryResponse } from "../types/editor";

type EditorHeaderCardProps = {
  domainId: string;
  summary: DomainSummaryResponse | null;
  readOnly: boolean;
  confirmLeaveWithDirty: () => boolean;
};

export function EditorHeaderCard({ domainId, summary, readOnly, confirmLeaveWithDirty }: EditorHeaderCardProps) {
  return (
    <div className="flex flex-1 flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-bold tracking-tight text-slate-900 dark:text-white">
            <FiFolder className="h-5 w-5" /> Редактор сайта
          </h1>
          <p className="text-xs text-slate-500 dark:text-slate-400">
            {summary?.domain.url || "—"} · Проект: {summary?.project_name || "—"}
            {" · "}
            <span className={readOnly ? "text-amber-600 dark:text-amber-400" : "text-emerald-600 dark:text-emerald-400"}>
              {summary?.my_role || "viewer"} ({readOnly ? "чтение" : "редактирование"})
            </span>
          </p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Link
          href={`/domains/${domainId}`}
          onClick={(event) => {
            if (!confirmLeaveWithDirty()) {
              event.preventDefault();
            }
          }}
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <FiArrowLeft className="h-3.5 w-3.5" /> К домену
        </Link>
        {summary?.domain.project_id && (
          <Link
            href={`/projects/${summary.domain.project_id}`}
            onClick={(event) => {
              if (!confirmLeaveWithDirty()) {
                event.preventDefault();
              }
            }}
            className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-indigo-500"
          >
            <FiArrowLeft className="h-3.5 w-3.5" /> К проекту
          </Link>
        )}
      </div>
    </div>
  );
}
