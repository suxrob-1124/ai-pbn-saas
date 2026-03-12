"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { authFetch, del, patch } from "@/lib/http";
import { useAuthGuard } from "@/lib/useAuth";
import {
  FiClock,
  FiPlay,
  FiCheck,
  FiAlertTriangle,
  FiRefreshCw,
  FiTrash2,
  FiPause,
  FiX,
} from "react-icons/fi";
import {
  ChevronRight,
  ListOrdered,
  Activity,
  FileText,
  Package,
  Globe,
  Calendar,
  Timer,
  AlertCircle,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { ArtifactsViewer, LogsViewer } from "@/components/ArtifactsViewer";
import { GenerationResultActions } from "@/components/GenerationResultActions";
import { computeDisplayProgress } from "@/lib/pipelineProgress";
import { AuditReport } from "@/components/AuditReport";
import { Badge } from "@/components/Badge";

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
  const [activeTab, setActiveTab] = useState<"logs" | "artifacts">("logs");

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
          setDomainUrl(d?.url || "");
        } catch {
          setDomainUrl("");
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
    if (
      !item ||
      (item.status !== "processing" &&
        item.status !== "pending" &&
        item.status !== "pause_requested" &&
        item.status !== "cancelling")
    ) {
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

  const canPauseOrCancel =
    item &&
    (item.status === "pending" ||
      item.status === "processing" ||
      item.status === "pause_requested" ||
      item.status === "cancelling");
  const canResume = item && item.status === "paused";
  const displayProgress = item
    ? computeDisplayProgress(item.artifacts, item.progress, item.status)
    : 0;

  const isActive =
    item?.status === "processing" ||
    item?.status === "pending" ||
    item?.status === "pause_requested";

  if (!item && loading) {
    return (
      <div className="p-10 flex justify-center text-slate-500">
        <FiRefreshCw className="w-5 h-5 animate-spin mr-2" /> Загрузка
        задачи...
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-20">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">
              <Link
                href="/queue"
                className="hover:text-indigo-600 transition-colors"
              >
                Очередь
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span className="text-slate-400 dark:text-slate-500 truncate max-w-[200px]">
                {id?.slice(0, 8)}...
              </span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Задача генерации
              {item && <StatusBadge status={item.status} />}
            </h1>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {canResume && (
              <button
                onClick={resumeGeneration}
                disabled={loading}
                className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold bg-emerald-600 text-white hover:bg-emerald-500 active:scale-95 transition-all shadow-sm disabled:opacity-50"
              >
                <FiPlay className="w-4 h-4" /> Возобновить
              </button>
            )}
            {canPauseOrCancel && (
              <>
                {item?.status !== "cancelling" && (
                  <button
                    onClick={pauseGeneration}
                    disabled={loading || item?.status === "pause_requested"}
                    className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold border border-amber-200 bg-white text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:bg-slate-800 dark:text-amber-300 active:scale-95 transition-all disabled:opacity-50"
                  >
                    <FiPause className="w-4 h-4" />{" "}
                    {item?.status === "pause_requested"
                      ? "Запрошена..."
                      : "Пауза"}
                  </button>
                )}
                <button
                  onClick={cancelGeneration}
                  disabled={loading || item?.status === "cancelling"}
                  className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold border border-red-200 bg-white text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300 active:scale-95 transition-all disabled:opacity-50"
                >
                  <FiX className="w-4 h-4" />{" "}
                  {item?.status === "cancelling" ? "Отмена..." : "Отменить"}
                </button>
              </>
            )}
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 p-2.5 rounded-xl border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 transition-all disabled:opacity-50"
              title="Обновить"
            >
              <FiRefreshCw
                className={`w-4 h-4 ${loading ? "animate-spin" : ""}`}
              />
            </button>
            <button
              onClick={deleteGeneration}
              disabled={loading}
              className="inline-flex items-center gap-2 p-2.5 rounded-xl border border-red-200 bg-white text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-400 transition-all disabled:opacity-50"
              title="Удалить"
            >
              <FiTrash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </header>

      {/* MAIN CONTENT */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto">
          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 mb-6 flex items-center gap-2">
              <AlertCircle className="w-5 h-5 shrink-0" /> {error}
            </div>
          )}

          {item && (
            <div className="grid grid-cols-1 xl:grid-cols-[1fr_360px] gap-6 items-start">
              {/* LEFT COLUMN */}
              <div className="space-y-6 min-w-0">
                {/* Progress card (only for active tasks) */}
                {isActive && (
                  <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                    <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
                      <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        <Activity className="w-4 h-4 text-indigo-500" />{" "}
                        Прогресс
                      </h3>
                      <span className="text-xs font-mono text-slate-500">
                        {displayProgress}%
                      </span>
                    </div>
                    <div className="p-5">
                      <div className="w-full h-3 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-gradient-to-r from-indigo-500 to-indigo-600 rounded-full transition-all duration-700 ease-out"
                          style={{ width: `${displayProgress}%` }}
                        />
                      </div>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-2">
                        {item.status === "pending"
                          ? "Задача ожидает в очереди..."
                          : item.status === "pause_requested"
                            ? "Запрос на паузу..."
                            : "Генерация в процессе..."}
                      </p>
                    </div>
                  </div>
                )}

                {/* Error block */}
                {item.error && (
                  <div className="bg-white dark:bg-[#0f1523] border border-red-200 dark:border-red-900/50 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                    <div className="p-5 border-b border-red-100 dark:border-red-900/30 bg-red-50/50 dark:bg-red-950/20 flex items-center gap-2">
                      <h3 className="font-bold text-red-700 dark:text-red-400 flex items-center gap-2">
                        <AlertCircle className="w-4 h-4" /> Ошибка выполнения
                      </h3>
                    </div>
                    <div className="p-5">
                      <pre className="text-sm text-red-600 dark:text-red-400 whitespace-pre-wrap font-mono bg-red-50/50 dark:bg-red-950/20 rounded-xl p-4 border border-red-100 dark:border-red-900/30">
                        {item.error}
                      </pre>
                    </div>
                  </div>
                )}

                {/* Result actions for success */}
                {item.status === "success" && (
                  <div className="animate-in fade-in">
                    <GenerationResultActions artifacts={item.artifacts} />
                  </div>
                )}

                {/* Audit report */}
                {item.artifacts?.audit_report && (
                  <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                    <div className="p-5">
                      <AuditReport report={item.artifacts.audit_report} />
                    </div>
                  </div>
                )}

                {/* Tabs: Logs / Artifacts */}
                <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                  <div className="flex border-b border-slate-100 dark:border-slate-800/60 bg-slate-50 dark:bg-[#0a1020]">
                    <button
                      onClick={() => setActiveTab("logs")}
                      className={`flex-1 py-3 text-sm font-semibold transition-colors flex items-center justify-center gap-2 ${
                        activeTab === "logs"
                          ? "text-indigo-600 border-b-2 border-indigo-600"
                          : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                      }`}
                    >
                      <FileText className="w-4 h-4" /> Логи
                    </button>
                    <button
                      onClick={() => setActiveTab("artifacts")}
                      className={`flex-1 py-3 text-sm font-semibold transition-colors flex items-center justify-center gap-2 ${
                        activeTab === "artifacts"
                          ? "text-indigo-600 border-b-2 border-indigo-600"
                          : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
                      }`}
                    >
                      <Package className="w-4 h-4" /> Артефакты
                    </button>
                  </div>
                  <div className="p-5">
                    {activeTab === "logs" && <LogsViewer logs={item.logs} />}
                    {activeTab === "artifacts" && (
                      <ArtifactsViewer artifacts={item.artifacts} />
                    )}
                  </div>
                </div>
              </div>

              {/* RIGHT COLUMN — Info sidebar */}
              <div className="space-y-6">
                {/* Details card */}
                <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                  <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                    <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                      <ListOrdered className="w-4 h-4 text-indigo-500" />{" "}
                      Информация
                    </h3>
                  </div>
                  <div className="p-5 space-y-4">
                    {/* Status */}
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                        Статус
                      </span>
                      <StatusBadge status={item.status} />
                    </div>

                    {/* Progress */}
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                        Прогресс
                      </span>
                      <span className="text-sm font-mono font-semibold text-slate-900 dark:text-white">
                        {displayProgress}%
                      </span>
                    </div>

                    <hr className="border-slate-100 dark:border-slate-800/60" />

                    {/* Domain */}
                    <div>
                      <span className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider flex items-center gap-1.5 mb-1.5">
                        <Globe className="w-3.5 h-3.5" /> Домен
                      </span>
                      {domainUrl ? (
                        <Link
                          href={`/domains/${item.domain_id}`}
                          className="text-sm font-medium text-indigo-600 hover:text-indigo-500 dark:text-indigo-400 dark:hover:text-indigo-300 transition-colors"
                        >
                          {domainUrl}
                        </Link>
                      ) : (
                        <span className="text-sm text-slate-400">—</span>
                      )}
                    </div>

                    <hr className="border-slate-100 dark:border-slate-800/60" />

                    {/* Timestamps */}
                    <div className="space-y-3">
                      <TimestampRow
                        icon={<Calendar className="w-3.5 h-3.5" />}
                        label="Создано"
                        value={item.created_at}
                      />
                      <TimestampRow
                        icon={<FiRefreshCw className="w-3.5 h-3.5" />}
                        label="Обновлено"
                        value={item.updated_at}
                      />
                      <TimestampRow
                        icon={<FiPlay className="w-3.5 h-3.5" />}
                        label="Старт"
                        value={item.started_at}
                      />
                      <TimestampRow
                        icon={<Timer className="w-3.5 h-3.5" />}
                        label="Финиш"
                        value={item.finished_at}
                      />
                    </div>

                    {/* Duration */}
                    {item.started_at && item.finished_at && (
                      <>
                        <hr className="border-slate-100 dark:border-slate-800/60" />
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                            Длительность
                          </span>
                          <span className="text-sm font-mono text-slate-700 dark:text-slate-300">
                            {formatDuration(item.started_at, item.finished_at)}
                          </span>
                        </div>
                      </>
                    )}
                  </div>
                </div>

                {/* Quick actions */}
                <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
                  <div className="p-5 space-y-2">
                    <Link
                      href="/queue"
                      className="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold bg-indigo-600 text-white hover:bg-indigo-500 active:scale-95 transition-all shadow-sm"
                    >
                      <ListOrdered className="w-4 h-4" /> К очереди
                    </Link>
                    {domainUrl && (
                      <Link
                        href={`/domains/${item.domain_id}`}
                        className="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-all"
                      >
                        <Globe className="w-4 h-4" /> К домену
                      </Link>
                    )}
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function TimestampRow({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value?: string;
}) {
  return (
    <div>
      <span className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider flex items-center gap-1.5 mb-1">
        {icon} {label}
      </span>
      <span className="text-sm text-slate-700 dark:text-slate-300">
        {value ? new Date(value).toLocaleString() : "—"}
      </span>
    </div>
  );
}

function formatDuration(start: string, end: string): string {
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (ms < 0) return "—";
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes === 0) return `${seconds}с`;
  return `${minutes}м ${seconds}с`;
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<
    string,
    { text: string; tone: "amber" | "yellow" | "slate" | "orange" | "red" | "green"; icon: ReactNode }
  > = {
    pending: { text: "В очереди", tone: "amber", icon: <FiClock /> },
    processing: { text: "В работе", tone: "amber", icon: <FiPlay /> },
    pause_requested: {
      text: "Пауза запрошена",
      tone: "yellow",
      icon: <FiPause />,
    },
    paused: { text: "Приостановлено", tone: "slate", icon: <FiPause /> },
    cancelling: { text: "Отмена...", tone: "orange", icon: <FiX /> },
    cancelled: { text: "Отменено", tone: "red", icon: <FiX /> },
    success: { text: "Готово", tone: "green", icon: <FiCheck /> },
    error: {
      text: "Ошибка",
      tone: "red",
      icon: <FiAlertTriangle />,
    },
  };
  const cfg = map[status] || {
    text: status,
    tone: "slate" as const,
    icon: <FiClock />,
  };
  return (
    <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />
  );
}
