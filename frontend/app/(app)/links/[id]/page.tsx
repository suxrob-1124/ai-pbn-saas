'use client';

import { useCallback, useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import {
  AlertTriangle,
  Check,
  Clock,
  ExternalLink,
  RefreshCw,
  RotateCw,
  Trash2,
  ArrowRight,
  Terminal,
  Code2,
  Link as LinkIcon,
  ChevronRight,
} from 'lucide-react';
import { authFetch } from '@/lib/http';
import { useAuthGuard } from '@/lib/useAuth';
import { showToast } from '@/lib/toastStore';
import { deleteLinkTask, retryLinkTask } from '@/lib/linkTasksApi';
import type { LinkTaskDTO } from '@/types/linkTasks';
import { Badge } from '@/components/Badge';
import {
  getLinkTaskStatusMeta,
  isLinkTaskInProgress,
  normalizeLinkTaskStatus,
} from '@/lib/linkTaskStatus';
import { getLinkTaskSteps } from '@/features/domain-project/services/statusMeta';

type Domain = { id: string; url: string; project_id: string };

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
          setDomain(await authFetch<Domain>(`/api/domains/${current.domain_id}`));
        } catch {
          setDomain({ id: current.domain_id, url: '', project_id: '' });
        }
      }
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить задачу ссылки');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!task || !isLinkTaskInProgress(task.status)) return;
    const timer = window.setInterval(load, 5000);
    return () => window.clearInterval(timer);
  }, [task, load]);

  const handleRetry = async () => {
    if (!task) return;
    const domainLabel = domain?.url || 'домен';
    if (!confirm(`Повторить задачу для ${domainLabel}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await retryLinkTask(task.id);
      showToast({ type: 'success', title: 'Повтор поставлен в очередь', message: domainLabel });
      await load();
    } catch (err: any) {
      setError(err?.message || 'Не удалось повторить задачу');
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!task) return;
    if (!confirm('Удалить задачу ссылки?')) return;
    setLoading(true);
    setError(null);
    try {
      await deleteLinkTask(task.id);
      showToast({ type: 'success', title: 'Задача удалена' });
      router.push(domain?.project_id ? `/projects/${domain.project_id}/queue` : '/queue');
    } catch (err: any) {
      setError(err?.message || 'Не удалось удалить задачу');
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
      setLoading(false);
    }
  };

  const actionLabel = task?.action === 'remove' ? 'Удаление' : 'Вставка';
  const backHref = (
    domain?.project_id ? `/projects/${domain.project_id}/queue?tab=links` : '/queue?tab=links'
  ) as any;

  // Вычисление визуального пайплайна
  const steps = getLinkTaskSteps(task?.action);
  const reached = new Set<string>();
  const normStatus = normalizeLinkTaskStatus(task?.status || '');
  const isRemove = task?.action === 'remove';

  if (
    ['pending', 'searching', 'inserted', 'generated', 'removing', 'removed', 'failed'].includes(
      normStatus,
    )
  )
    reached.add('pending');
  if (['searching', 'inserted', 'generated', 'removing', 'removed'].includes(normStatus))
    reached.add('searching');
  if (!isRemove) {
    if (['inserted'].includes(normStatus)) reached.add('inserted');
    if (['generated'].includes(normStatus)) reached.add('generated');
  } else {
    if (['removing', 'removed'].includes(normStatus)) reached.add('removing');
    if (['removed'].includes(normStatus)) reached.add('removed');
  }

  // Общие классы
  const cardClass =
    'bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm p-6 flex flex-col gap-4';
  const labelClass = 'text-[10px] font-bold uppercase tracking-wider text-slate-500 mb-1 block';

  if (!task && loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-slate-500">
        <RefreshCw className="w-5 h-5 animate-spin mr-2" /> Загрузка задачи...
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Проекты
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              {domain?.project_id && (
                <>
                  <Link
                    href={`/projects/${domain.project_id}`}
                    className="hover:text-indigo-600 transition-colors">
                    Проект
                  </Link>
                  <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
                  <Link
                    href={`/projects/${domain.project_id}/queue?tab=links`}
                    className="hover:text-indigo-600 transition-colors">
                    Очередь ссылок
                  </Link>
                  <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
                </>
              )}
              <span className="text-slate-900 dark:text-slate-200 font-mono text-xs">
                {id.split('-')[0]}...
              </span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Детали задачи{' '}
              <Badge label={actionLabel} tone={task?.action === 'remove' ? 'red' : 'blue'} />
            </h1>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={load}
              disabled={loading}
              className="p-2.5 rounded-xl border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-300 dark:hover:bg-slate-800 transition-all shadow-sm">
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <Link
              href={backHref}
              className="px-5 py-2.5 rounded-xl border border-slate-200 bg-white text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-300 dark:hover:bg-slate-800 transition-all shadow-sm">
              Назад к очереди
            </Link>
            {domain?.url && (
              <Link
                href={`/domains/${domain.id}`}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm active:scale-95">
                Перейти к домену
              </Link>
            )}
          </div>
        </div>
      </header>

      {/* CONTENT AREA */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 flex items-center gap-2">
              <AlertTriangle className="w-5 h-5" /> {error}
            </div>
          )}

          {task && (
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_300px] gap-6">
              {/* ЛЕВАЯ КОЛОНКА */}
              <div className="space-y-6">
                {/* 1. ПАЙПЛАЙН И СТАТУС */}
                <div className={cardClass}>
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                    <div>
                      <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2 mb-1">
                        Текущий статус
                      </h3>
                      <LinkTaskStatusBadge status={task.status} />
                    </div>
                    <div className="flex items-center gap-2">
                      {(normStatus === 'failed' || (normStatus === 'pending' && !!task.error_message)) && (
                        <button
                          onClick={handleRetry}
                          disabled={loading}
                          className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-amber-50 text-amber-700 hover:bg-amber-100 dark:bg-amber-900/20 dark:text-amber-400 font-semibold text-sm transition-colors border border-amber-200 dark:border-amber-800/50">
                          <RotateCw className="w-4 h-4" /> Перезапустить
                        </button>
                      )}
                      <button
                        onClick={handleDelete}
                        disabled={loading}
                        className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 font-semibold text-sm transition-colors border border-red-200 dark:border-red-800/50">
                        <Trash2 className="w-4 h-4" /> Удалить
                      </button>
                    </div>
                  </div>

                  <div className="p-4 bg-slate-50 dark:bg-[#0a1020] rounded-xl border border-slate-100 dark:border-slate-800 flex flex-wrap items-center gap-2">
                    {steps.map((step, i) => {
                      const isReached = reached.has(step.id);
                      return (
                        <div key={step.id} className="flex items-center gap-2">
                          <Badge
                            label={step.label}
                            tone={isReached ? 'emerald' : 'slate'}
                            icon={
                              isReached ? (
                                <Check className="w-3.5 h-3.5" />
                              ) : (
                                <Clock className="w-3.5 h-3.5" />
                              )
                            }
                          />
                          {i < steps.length - 1 && (
                            <ArrowRight className="w-4 h-4 text-slate-300 dark:text-slate-700" />
                          )}
                        </div>
                      );
                    })}
                    {normStatus === 'failed' && (
                      <div className="flex items-center gap-2">
                        <ArrowRight className="w-4 h-4 text-slate-300 dark:text-slate-700" />
                        <Badge
                          label="Сбой"
                          tone="red"
                          icon={<AlertTriangle className="w-3.5 h-3.5" />}
                        />
                      </div>
                    )}
                  </div>

                  {normStatus === 'pending' && !task.error_message && (
                    <div className="p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-xl flex items-start gap-3">
                      <Clock className="w-4 h-4 text-amber-600 mt-0.5 shrink-0" />
                      <div>
                        <div className="text-amber-700 dark:text-amber-400 font-semibold text-sm">Ожидание постановки в очередь</div>
                        <div className="text-amber-600 dark:text-amber-500 text-xs mt-0.5">
                          {new Date(task.scheduled_for) <= new Date()
                            ? 'Задача готова к выполнению — планировщик поставит её в очередь в течение минуты.'
                            : `Запуск запланирован на ${new Date(task.scheduled_for).toLocaleString()}`}
                        </div>
                      </div>
                    </div>
                  )}

                  {task.error_message && (
                    <div className="p-4 bg-red-50 border border-red-200 rounded-xl">
                      <div className="text-red-700 dark:text-red-400 font-bold flex items-center gap-2 mb-2">
                        <AlertTriangle className="w-4 h-4" /> Текст ошибки
                      </div>
                      <code className="text-xs text-red-600 dark:text-red-300">
                        {task.error_message}
                      </code>
                    </div>
                  )}
                </div>

                {/* 2. ПАРАМЕТРЫ ЗАДАЧИ */}
                <div className={cardClass}>
                  <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    <LinkIcon className="w-4 h-4 text-indigo-500" /> Данные для вставки
                  </h3>
                  <div className="grid sm:grid-cols-2 gap-4">
                    <div className="bg-slate-50 dark:bg-[#0a1020] p-4 rounded-xl border border-slate-100 dark:border-slate-800">
                      <label className={labelClass}>Текст анкора</label>
                      <div className="font-medium text-slate-900 dark:text-white">
                        {task.anchor_text || '—'}
                      </div>
                    </div>
                    <div className="bg-slate-50 dark:bg-[#0a1020] p-4 rounded-xl border border-slate-100 dark:border-slate-800">
                      <label className={labelClass}>URL Акцептора</label>
                      <a
                        href={task.target_url}
                        target="_blank"
                        rel="noreferrer"
                        className="font-medium text-indigo-600 dark:text-indigo-400 hover:underline break-all">
                        {task.target_url || '—'}
                      </a>
                    </div>
                    <div className="sm:col-span-2 bg-slate-50 dark:bg-[#0a1020] p-4 rounded-xl border border-slate-100 dark:border-slate-800 flex items-center justify-between">
                      <div>
                        <label className={labelClass}>Расположение файла</label>
                        <div className="font-mono text-sm text-slate-700 dark:text-slate-300">
                          {task.found_location || 'Еще не найдено'}
                        </div>
                      </div>
                      {task.found_location && (
                        <Link
                          href={`/domains/${task.domain_id}/editor?path=${task.found_location.split(':')[0]}`}
                          className="text-xs font-semibold text-indigo-600 hover:text-indigo-500 flex items-center gap-1">
                          <Code2 className="w-4 h-4" /> В редактор
                        </Link>
                      )}
                    </div>
                  </div>
                </div>

                {/* 3. СГЕНЕРИРОВАННЫЙ ТЕКСТ */}
                {task.generated_content && (
                  <div className={`${cardClass} !p-0`}>
                    <div className="p-5 border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-[#0a1020]">
                      <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        <Code2 className="w-4 h-4 text-indigo-500" /> Итоговый контент
                      </h3>
                    </div>
                    <div className="p-5 overflow-x-auto bg-[#1e1e1e] dark:bg-black rounded-b-2xl">
                      <pre className="text-[13px] font-mono text-slate-300 whitespace-pre-wrap leading-relaxed">
                        {task.generated_content}
                      </pre>
                    </div>
                  </div>
                )}

                {/* 4. ТЕРМИНАЛ */}
                {task.log_lines && task.log_lines.length > 0 && (
                  <div className={`${cardClass} !p-0`}>
                    <div className="p-5 border-b border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-[#0a1020]">
                      <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        <Terminal className="w-4 h-4 text-indigo-500" /> Лог воркера
                      </h3>
                    </div>
                    <div className="p-5 overflow-x-auto bg-[#1e1e1e] dark:bg-black rounded-b-2xl max-h-[400px]">
                      <pre className="text-[12px] font-mono text-emerald-400 whitespace-pre-wrap leading-relaxed">
                        {task.log_lines.join('\n')}
                      </pre>
                    </div>
                  </div>
                )}
              </div>

              {/* ПРАВАЯ КОЛОНКА */}
              <div className="space-y-6">
                <div className={cardClass}>
                  <h3 className="text-sm font-bold text-slate-900 dark:text-white mb-4">
                    Метаданные
                  </h3>
                  <div className="space-y-4">
                    <div>
                      <label className={labelClass}>Создано</label>
                      <div className="text-sm text-slate-700 dark:text-slate-300 font-medium">
                        {new Date(task.created_at).toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <label className={labelClass}>Запланировано на</label>
                      <div className="text-sm text-slate-700 dark:text-slate-300 font-medium">
                        {new Date(task.scheduled_for).toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <label className={labelClass}>Фактически завершено</label>
                      <div className="text-sm text-slate-700 dark:text-slate-300 font-medium">
                        {task.completed_at ? new Date(task.completed_at).toLocaleString() : '—'}
                      </div>
                    </div>
                    <div>
                      <label className={labelClass}>Попыток выполнения</label>
                      <div className="text-sm text-slate-700 dark:text-slate-300 font-medium">
                        {task.attempts} / 7
                      </div>
                    </div>
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

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === 'refresh' ? (
      <RefreshCw className="w-3.5 h-3.5" />
    ) : meta.icon === 'check' ? (
      <Check className="w-3.5 h-3.5" />
    ) : meta.icon === 'alert' ? (
      <AlertTriangle className="w-3.5 h-3.5" />
    ) : (
      <Clock className="w-3.5 h-3.5" />
    );
  return <Badge label={meta.text} tone={meta.tone} icon={icon} />;
}
