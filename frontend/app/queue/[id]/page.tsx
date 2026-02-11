"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { authFetch, del, patch } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiClock, FiPlay, FiCheck, FiAlertTriangle, FiRefreshCw, FiTrash2, FiPause, FiX } from "react-icons/fi";
import { useRouter } from "next/navigation";
import { ArtifactsViewer, LogsViewer } from "../../../components/ArtifactsViewer";
import { GenerationResultActions } from "../../../components/GenerationResultActions";
import { computeDisplayProgress } from "../../../lib/pipelineProgress";
import { AuditReport } from "../../../components/AuditReport";
import { Badge } from "../../../components/Badge";

type Generation = {
  id: string;
  domain_id: string;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
  logs?: any;
  artifacts?: Record<string, any>;
};

export default function QueueItemPage() {
  useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const [item, setItem] = useState<Generation | null>(null);
  const [domainUrl, setDomainUrl] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const g = await authFetch<Generation>(`/api/generations/${id}`);
      setItem(g);
      if (g?.domain_id) {
        try {
          const d = await authFetch<any>(`/api/domains/${g.domain_id}`);
          setDomainUrl(d?.url || g.domain_id);
        } catch {
          setDomainUrl(g.domain_id);
        }
      }
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить задачу");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!item || (item.status !== "processing" && item.status !== "pending" && item.status !== "pause_requested" && item.status !== "cancelling")) {
      return;
    }
    const timer = window.setInterval(load, 5000);
    return () => window.clearInterval(timer);
  }, [item, load]);

  const deleteGeneration = async () => {
    if (!confirm("Удалить эту задачу генерации?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/generations/${id}`);
      router.push("/queue");
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить задачу");
    } finally {
      setLoading(false);
    }
  };

  const pauseGeneration = async () => {
    if (!confirm("Приостановить выполнение задачи?")) return;
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${id}`, { action: "pause" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось приостановить задачу");
    } finally {
      setLoading(false);
    }
  };

  const resumeGeneration = async () => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${id}`, { action: "resume" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось возобновить задачу");
    } finally {
      setLoading(false);
    }
  };

  const cancelGeneration = async () => {
    if (!confirm("Отменить выполнение задачи?")) return;
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/generations/${id}`, { action: "cancel" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось отменить задачу");
    } finally {
      setLoading(false);
    }
  };

  const canPauseOrCancel = item && (item.status === "pending" || item.status === "processing" || item.status === "pause_requested" || item.status === "cancelling");
  const canResume = item && item.status === "paused";
  const displayProgress = item ? computeDisplayProgress(item.artifacts, item.progress, item.status) : 0;

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Задача генерации</h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">Статус конкретного запуска.</p>
            <div className="mt-3 text-slate-700 dark:text-slate-200 font-mono text-sm">ID: {id}</div>
            {domainUrl && (
              <div className="mt-1 text-sm">
                Домен:{" "}
                <Link href={`/domains/${item?.domain_id || ""}`} className="text-indigo-600 hover:underline">
                  {domainUrl}
                </Link>
              </div>
            )}
          </div>
          <div className="flex gap-2">
            {canResume && (
              <button
                onClick={resumeGeneration}
                disabled={loading}
                className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300 disabled:opacity-50"
              >
                <FiPlay /> Возобновить
              </button>
            )}
            {canPauseOrCancel && (
              <>
                {item?.status !== "cancelling" && (
                  <button
                    onClick={pauseGeneration}
                    disabled={loading || item?.status === "pause_requested"}
                    className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:bg-slate-800 dark:text-amber-300 disabled:opacity-50"
                  >
                    <FiPause /> {item?.status === "pause_requested" ? "Пауза запрошена..." : "Пауза"}
                  </button>
                )}
                <button
                  onClick={cancelGeneration}
                  disabled={loading || item?.status === "cancelling"}
                  className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
                >
                  <FiX /> {item?.status === "cancelling" ? "Отмена..." : "Отменить"}
                </button>
              </>
            )}
            {item?.status === "cancelled" && (
              <button
                onClick={cancelGeneration}
                disabled={loading}
                className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
              >
                <FiX /> Отменить
              </button>
            )}
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <button
              onClick={deleteGeneration}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 disabled:opacity-50"
            >
              <FiTrash2 /> Удалить
            </button>
            <Link href="/queue" className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500">
              ← К очереди
            </Link>
          </div>
        </div>
        {error && <div className="mt-2 text-red-500 text-sm">{error}</div>}
      </div>

      {item && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
          <div className="flex items-center gap-2">
            <StatusBadge status={item.status} />
            <span className="text-sm text-slate-600 dark:text-slate-300">{displayProgress}%</span>
          </div>
          <div className="text-sm text-slate-600 dark:text-slate-300 space-y-1">
            <div>Создано: {item.created_at ? new Date(item.created_at).toLocaleString() : "—"}</div>
            <div>Обновлено: {item.updated_at ? new Date(item.updated_at).toLocaleString() : "—"}</div>
            <div>Старт: {item.started_at ? new Date(item.started_at).toLocaleString() : "—"}</div>
            <div>Финиш: {item.finished_at ? new Date(item.finished_at).toLocaleString() : "—"}</div>
          </div>
          {item.error && <div className="text-red-500 text-sm">Ошибка: {item.error}</div>}
          {item.status === "success" && <GenerationResultActions artifacts={item.artifacts} />}
          <AuditReport report={item.artifacts?.audit_report} />
          <LogsViewer logs={item.logs} />
          <ArtifactsViewer artifacts={item.artifacts} />
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: "amber" | "yellow" | "slate" | "orange" | "red" | "green"; icon: ReactNode }> = {
    pending: { text: "В очереди", tone: "amber", icon: <FiClock /> },
    processing: { text: "В работе", tone: "amber", icon: <FiPlay /> },
    pause_requested: { text: "Пауза запрошена", tone: "yellow", icon: <FiPause /> },
    paused: { text: "Приостановлено", tone: "slate", icon: <FiPause /> },
    cancelling: { text: "Отмена...", tone: "orange", icon: <FiX /> },
    cancelled: { text: "Отменено", tone: "red", icon: <FiX /> },
    success: { text: "Готово", tone: "green", icon: <FiCheck /> },
    error: { text: "Ошибка", tone: "red", icon: <FiAlertTriangle /> },
  };
  const cfg = map[status] || { text: status, tone: "slate" as const, icon: <FiClock /> };
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}
