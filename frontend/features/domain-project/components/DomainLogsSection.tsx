import Link from "next/link";
import type { ReactNode } from "react";
import { ArtifactsViewer, LogsViewer } from "../../../components/ArtifactsViewer";
import { AuditReport } from "../../../components/AuditReport";

type GenerationLike = {
  id: string;
  status: string;
  updated_at?: string;
};

type GenerationDetailLike = {
  logs?: unknown;
  artifacts?: Record<string, unknown>;
};

type DomainLogsSectionProps = {
  currentAttempt: GenerationLike | null;
  latestSuccess: GenerationLike | null;
  latestAttemptDetail?: GenerationDetailLike;
  latestSuccessDetail?: GenerationDetailLike;
  latestDisplayProgress: number;
  renderStatusBadge: (status: string) => ReactNode;
};

export function DomainLogsSection({
  currentAttempt,
  latestSuccess,
  latestAttemptDetail,
  latestSuccessDetail,
  latestDisplayProgress,
  renderStatusBadge
}: DomainLogsSectionProps) {
  return (
    <>
      {currentAttempt && currentAttempt.status !== "success" && (
        <div id="domain-artifacts" className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold">Последняя попытка</h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Статус: {renderStatusBadge(currentAttempt.status)} · Прогресс: {latestDisplayProgress}%
              </p>
            </div>
            <Link href={`/queue/${currentAttempt.id}`} className="text-sm text-indigo-600 hover:underline">
              Открыть карточку запуска
            </Link>
          </div>
          <AuditReport report={latestAttemptDetail?.artifacts?.audit_report} />
          <LogsViewer logs={latestAttemptDetail?.logs} />
          <ArtifactsViewer artifacts={latestAttemptDetail?.artifacts} />
        </div>
      )}

      {latestSuccess && (
        <div
          id={currentAttempt && currentAttempt.status !== "success" ? undefined : "domain-artifacts"}
          className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4"
        >
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold">Последний успешный запуск</h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Статус: {renderStatusBadge(latestSuccess.status)} · Обновлено:{" "}
                {latestSuccess.updated_at ? new Date(latestSuccess.updated_at).toLocaleString() : "—"}
              </p>
            </div>
            <Link href={`/queue/${latestSuccess.id}`} className="text-sm text-indigo-600 hover:underline">
              Открыть карточку запуска
            </Link>
          </div>
          <AuditReport report={latestSuccessDetail?.artifacts?.audit_report} />
          <LogsViewer logs={latestSuccessDetail?.logs} />
          <ArtifactsViewer artifacts={latestSuccessDetail?.artifacts} />
        </div>
      )}
    </>
  );
}

