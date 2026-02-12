"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { FiAlertTriangle, FiCheck, FiClock, FiExternalLink, FiRefreshCw, FiRotateCw, FiTrash2 } from "react-icons/fi";
import { authFetch } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { showToast } from "../../../lib/toastStore";
import { deleteLinkTask, retryLinkTask } from "../../../lib/linkTasksApi";
import type { LinkTaskDTO } from "../../../types/linkTasks";
import { Badge } from "../../../components/Badge";

type Domain = {
  id: string;
  url: string;
  project_id: string;
};

export default function LinkTaskPage() {
  useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [task, setTask] = useState<LinkTaskDTO | null>(null);
  const [domain, setDomain] = useState<Domain | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const current = await authFetch<LinkTaskDTO>(`/api/links/${id}`);
      setTask(current);
      if (current?.domain_id) {
        try {
          const d = await authFetch<Domain>(`/api/domains/${current.domain_id}`);
          setDomain(d);
        } catch {
          setDomain({ id: current.domain_id, url: "", project_id: "" });
        }
      }
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить задачу ссылки");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!task || !["pending", "searching", "removing"].includes(task.status)) {
      return;
    }
    const timer = window.setInterval(load, 5000);
    return () => window.clearInterval(timer);
  }, [task, load]);

  const handleRetry = async () => {
    if (!task) return;
    const domainLabel = domain?.url || "домен";
    if (!confirm(`Повторить задачу ссылки для домена ${domainLabel}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await retryLinkTask(task.id);
      showToast({
        type: "success",
        title: "Повтор поставлен в очередь",
        message: domainLabel
      });
      await load();
    } catch (err: any) {
      const msg = err?.message || "Не удалось повторить задачу";
      setError(msg);
      showToast({ type: "error", title: "Ошибка повтора", message: msg });
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!task) return;
    if (!confirm("Удалить задачу ссылки?")) return;
    setLoading(true);
    setError(null);
    try {
      await deleteLinkTask(task.id);
      showToast({
        type: "success",
        title: "Задача ссылки удалена",
        message: domain?.url || "Домен"
      });
      if (domain?.project_id) {
        router.push(`/projects/${domain.project_id}/queue`);
      } else {
        router.push("/queue");
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить задачу";
      setError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setLoading(false);
    }
  };

  const actionLabel = task?.action === "remove" ? "Удаление" : "Вставка";
  const lastLog = task?.log_lines?.length ? task.log_lines[task.log_lines.length - 1] : "";
  const backHref = domain?.project_id ? `/projects/${domain.project_id}/queue` : "/queue";

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Задача ссылки</h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">Статус обработки линкбилдинга.</p>
            <div className="mt-1 text-sm">
              Домен:{" "}
              {domain?.url ? (
                <Link href={{ pathname: `/domains/${domain.id}` }} className="text-indigo-600 hover:underline">
                  {domain.url}
                </Link>
              ) : (
                <span className="text-slate-500 dark:text-slate-400">—</span>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            {task?.status === "failed" && (
              <button
                onClick={handleRetry}
                disabled={loading}
                className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:bg-slate-800 dark:text-amber-300"
              >
                <FiRotateCw /> Повторить
              </button>
            )}
            <button
              onClick={handleDelete}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-700 dark:bg-slate-800 dark:text-red-300"
            >
              <FiTrash2 /> Удалить
            </button>
            <Link
              href={{ pathname: backHref }}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            >
              ← К очереди
            </Link>
            {domain?.url && (
              <Link
                href={{ pathname: `/domains/${domain.id}` }}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiExternalLink /> Домен
              </Link>
            )}
          </div>
        </div>
        {error && <div className="mt-2 text-red-500 text-sm">{error}</div>}
      </div>

      {task && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
          <div className="flex items-center gap-2">
            <LinkTaskStatusBadge status={task.status} />
            <span className="text-xs text-slate-500 dark:text-slate-400">Попытки: {task.attempts}</span>
          </div>
          <div className="grid gap-2 md:grid-cols-2 text-sm text-slate-600 dark:text-slate-300">
            <div>Действие: {actionLabel}</div>
            <div>Запланировано: {new Date(task.scheduled_for).toLocaleString()}</div>
            <div>Создано: {new Date(task.created_at).toLocaleString()}</div>
            <div>Завершено: {task.completed_at ? new Date(task.completed_at).toLocaleString() : "—"}</div>
            <div>Анкор: {task.anchor_text}</div>
            <div>Акцептор: {task.target_url}</div>
            <div>Найдено: {task.found_location || "—"}</div>
            <div>Последний лог: {lastLog || "—"}</div>
          </div>
          {task.error_message && <div className="text-red-500 text-sm">Ошибка: {task.error_message}</div>}
          {task.generated_content && (
            <details className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60 p-3">
              <summary className="cursor-pointer text-sm font-semibold">Сгенерированный текст</summary>
              <pre className="mt-2 whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-300">
                {task.generated_content}
              </pre>
            </details>
          )}
          {task.log_lines && task.log_lines.length > 0 && (
            <details className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/60 p-3">
              <summary className="cursor-pointer text-sm font-semibold">Логи воркера</summary>
              <pre className="mt-2 whitespace-pre-wrap text-xs text-slate-600 dark:text-slate-300">
                {task.log_lines.join("\n")}
              </pre>
            </details>
          )}
        </div>
      )}
    </div>
  );
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: "amber" | "blue" | "orange" | "sky" | "green" | "yellow" | "slate" | "red"; icon: ReactNode }> = {
    pending: { text: "Ожидает", tone: "amber", icon: <FiClock /> },
    searching: { text: "Поиск", tone: "blue", icon: <FiRefreshCw /> },
    removing: { text: "Удаление", tone: "orange", icon: <FiRefreshCw /> },
    found: { text: "Найдено", tone: "sky", icon: <FiCheck /> },
    inserted: { text: "Вставлено", tone: "green", icon: <FiCheck /> },
    generated: { text: "Вставлено (ген. текст)", tone: "yellow", icon: <FiCheck /> },
    removed: { text: "Удалено", tone: "slate", icon: <FiCheck /> },
    failed: { text: "Ошибка", tone: "red", icon: <FiAlertTriangle /> },
  };
  const cfg = map[status] || {
    text: status,
    tone: "slate" as const,
    icon: <FiClock />,
  };
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}
