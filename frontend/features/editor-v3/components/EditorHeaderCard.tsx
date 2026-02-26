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
    <div className="rounded-xl border border-slate-200 bg-white/80 p-4 shadow dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="flex items-center gap-2 text-xl font-semibold">
            <FiFolder /> Редактор сайта v2
          </h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {summary?.domain.url || "—"} • Проект: {summary?.project_name || "—"}
          </p>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Роль: {summary?.my_role || "viewer"} {readOnly ? "(только чтение)" : "(редактирование включено)"}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Link
            href={`/domains/${domainId}`}
            onClick={(event) => {
              if (!confirmLeaveWithDirty()) {
                event.preventDefault();
              }
            }}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiArrowLeft /> К домену
          </Link>
          {summary?.domain.project_id && (
            <Link
              href={`/projects/${summary.domain.project_id}`}
              onClick={(event) => {
                if (!confirmLeaveWithDirty()) {
                  event.preventDefault();
                }
              }}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            >
              <FiArrowLeft /> К проекту
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
