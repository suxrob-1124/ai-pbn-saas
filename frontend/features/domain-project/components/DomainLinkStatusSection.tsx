import { useMemo, useState } from 'react';
import Link from 'next/link';
import type { UrlObject } from 'url';
import {
  AlertTriangle,
  Check,
  Clock,
  Download,
  ExternalLink,
  Play,
  RefreshCw,
  Trash2,
  Terminal,
  Code2,
  Edit3,
  ArrowRight,
  History,
} from 'lucide-react';

import { Badge } from '../../../components/Badge';
import { authFetch } from '../../../lib/http';
import { getLinkTaskSteps } from '../services/statusMeta';
import { normalizeLinkTaskStatus } from '../../../lib/linkTaskStatus';
import { LinkTaskStatusBadge } from './DomainStatusBadges';

type DomainLike = { link_status?: string; link_status_effective?: string };
type LinkTask = {
  id: string;
  anchor_text: string;
  target_url: string;
  scheduled_for: string;
  action?: string;
  status: string;
  found_location?: string;
  generated_content?: string;
  error_message?: string;
  log_lines?: string[];
  attempts: number;
  created_at: string;
};
type LinkDiffEntry = { filePath: string; line: number; before: string; after: string };

type DomainLinkStatusSectionProps = {
  domainId: string;
  domain: DomainLike | null;
  linkTasks: LinkTask[];
  linkTasksLoading: boolean;
  linkTasksError: string | null;
  linkNotice: string | null;
  linkAnchor: string;
  linkAcceptor: string;
  linkInProgress: boolean;
  canRemoveLink: boolean;
  linkActionLabel: string;
  onRefreshLinkTasks: () => Promise<void>;
  onRunLinkTask: () => Promise<void>;
  onRemoveLinkTask: () => Promise<void>;
};

export function DomainLinkStatusSection({
  domainId,
  domain,
  linkTasks,
  linkTasksLoading,
  linkTasksError,
  linkNotice,
  linkAnchor,
  linkAcceptor,
  linkInProgress,
  canRemoveLink,
  linkActionLabel,
  onRefreshLinkTasks,
  onRunLinkTask,
  onRemoveLinkTask,
}: DomainLinkStatusSectionProps) {
  const [linkTab, setLinkTab] = useState<'summary' | 'logs'>('summary');
  const [showAllLinkTasks, setShowAllLinkTasks] = useState(false);
  const [linkDiffs, setLinkDiffs] = useState<Record<string, LinkDiffEntry>>({});

  const visibleLinkTasks = useMemo(
    () => (showAllLinkTasks ? linkTasks : linkTasks.slice(0, 3)),
    [linkTasks, showAllLinkTasks],
  );

  // --- УТИЛИТЫ ---
  const buildFileUrl = (path: string) =>
    `/api/domains/${domainId}/files/${path
      .split('/')
      .map((part) => encodeURIComponent(part))
      .join('/')}`;

  const buildEditorUrl = (filePath: string, line?: number): UrlObject => {
    const query: Record<string, string> = { path: filePath };
    if (line && line > 0) query.line = String(line);
    return { pathname: `/domains/${domainId}/editor`, query };
  };

  const parseFoundLocation = (value?: string) => {
    if (!value) return null;
    const [filePathRaw, lineRaw] = value.split(':');
    const filePath = (filePathRaw || '').trim();
    if (!filePath) return null;
    return { filePath, line: parseInt(lineRaw || '0', 10) || 1 };
  };

  const computeSnippet = (lines: string[], lineIndex: number, padding = 2) => {
    return lines
      .slice(Math.max(0, lineIndex - padding), Math.min(lines.length, lineIndex + padding + 1))
      .join('\n');
  };

  const stripAnchorTag = (text: string, anchor: string, target: string) => {
    if (!anchor || !target) return text;
    const escapedAnchor = anchor.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const escapedTarget = target.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(`<a[^>]*href=["']${escapedTarget}["'][^>]*>${escapedAnchor}</a>`, 'gi');
    return text.replace(re, anchor);
  };

  const renderDiffLines = (before: string, after: string, mode: 'before' | 'after') => {
    const beforeLines = before.split('\n');
    const afterLines = after.split('\n');
    const max = Math.max(beforeLines.length, afterLines.length);
    const rows = [];
    for (let i = 0; i < max; i += 1) {
      const b = beforeLines[i] ?? '';
      const a = afterLines[i] ?? '';
      const changed = b !== a;
      const text = mode === 'before' ? b : a;

      let cls = 'text-slate-400';
      if (changed) {
        cls =
          mode === 'before' ? 'bg-red-500/10 text-red-400' : 'bg-emerald-500/10 text-emerald-400';
      }

      rows.push(
        <div
          key={`${mode}-${i}`}
          className={`whitespace-pre-wrap font-mono text-[10px] px-2 py-0.5 ${cls}`}>
          {text || ' '}
        </div>,
      );
    }
    return rows;
  };

  const loadLinkDiff = async (task: LinkTask) => {
    if (!task.found_location || linkDiffs[task.id]) return;
    const loc = parseFoundLocation(task.found_location);
    if (!loc) return;
    try {
      const fileResp = await authFetch<{ content: string }>(buildFileUrl(loc.filePath));
      const lines = (fileResp?.content ?? '').split('\n');
      const idx = Math.max(0, loc.line - 1);
      const afterSnippet = computeSnippet(lines, idx, 2);
      const beforeSnippet = stripAnchorTag(afterSnippet, task.anchor_text, task.target_url);
      setLinkDiffs((prev) => ({
        ...prev,
        [task.id]: {
          filePath: loc.filePath,
          line: loc.line,
          before: beforeSnippet,
          after: afterSnippet,
        },
      }));
    } catch {}
  };

  // Короткий лейбл для кнопки запуска (чтобы влезало)
  const shortActionLabel = linkActionLabel.includes('Обновить') ? 'Обновить' : 'Запустить';

  return (
    <div className="flex flex-col space-y-5 animate-in fade-in">
      {/* 1. ПАНЕЛЬ УПРАВЛЕНИЯ (Компактная) */}
      <div className="flex flex-wrap items-center justify-between gap-y-3 gap-x-2">
        {/* Табы */}
        <div className="flex bg-slate-100 dark:bg-slate-800/80 p-1 rounded-xl">
          <button
            onClick={() => setLinkTab('summary')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-all ${linkTab === 'summary' ? 'bg-white dark:bg-[#1a2235] text-slate-900 dark:text-white shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-white'}`}>
            Сводка
          </button>
          <button
            onClick={() => setLinkTab('logs')}
            className={`px-3 py-1.5 text-xs font-semibold rounded-lg transition-all ${linkTab === 'logs' ? 'bg-white dark:bg-[#1a2235] text-slate-900 dark:text-white shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-white'}`}>
            Логи
          </button>
        </div>

        {/* Кнопки действий (Только иконки + 1 главная кнопка) */}
        <div className="flex items-center gap-1.5 ml-auto">
          <button
            onClick={onRefreshLinkTasks}
            disabled={linkTasksLoading}
            className="p-2 rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-400 dark:hover:bg-slate-800 transition-all shadow-sm"
            title="Обновить">
            <RefreshCw className={`w-4 h-4 ${linkTasksLoading ? 'animate-spin' : ''}`} />
          </button>

          {canRemoveLink && (
            <button
              onClick={onRemoveLinkTask}
              disabled={linkTasksLoading}
              className="p-2 rounded-lg border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 disabled:opacity-50 dark:border-red-900/50 dark:bg-red-500/10 dark:text-red-400 transition-colors shadow-sm"
              title="Удалить ссылку">
              <Trash2 className="w-4 h-4" />
            </button>
          )}

          <button
            onClick={onRunLinkTask}
            disabled={
              linkTasksLoading || linkInProgress || !linkAnchor.trim() || !linkAcceptor.trim()
            }
            className="inline-flex items-center justify-center gap-1.5 bg-indigo-600 text-white px-3 py-2 rounded-lg text-xs font-bold hover:bg-indigo-500 transition-all shadow-sm active:scale-95 disabled:opacity-50">
            {linkInProgress ? (
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Play className="w-3.5 h-3.5 fill-current" />
            )}
            {shortActionLabel}
          </button>
        </div>
      </div>

      {/* Алерты */}
      {linkNotice && (
        <div className="p-3 bg-emerald-50 text-emerald-600 text-[11px] font-medium rounded-lg border border-emerald-100 dark:bg-emerald-500/10 dark:border-emerald-500/20 dark:text-emerald-400">
          {linkNotice}
        </div>
      )}
      {linkTasksError && (
        <div className="p-3 bg-red-50 text-red-600 text-[11px] font-medium rounded-lg border border-red-100 dark:bg-red-500/10 dark:border-red-500/20 dark:text-red-400">
          {linkTasksError}
        </div>
      )}

      {/* ================================================= */}
      {/* ТАБ: СВОДКА */}
      {/* ================================================= */}
      {linkTab === 'summary' && (
        <div className="space-y-5 animate-in fade-in">
          {/* Последняя задача */}
          {linkTasks[0] ? (
            <div className="bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/60 rounded-xl p-4 flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div>
                  <h4 className="text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1">
                    Статус последней задачи
                  </h4>
                  <LinkTaskStatusBadge status={linkTasks[0].status} />
                </div>
                <Link
                  href={`/links/${linkTasks[0].id}`}
                  className="text-[11px] font-semibold text-indigo-600 dark:text-indigo-400 hover:underline flex items-center gap-1">
                  Подробнее <ExternalLink className="w-3 h-3" />
                </Link>
              </div>
              <div className="text-[10px] text-slate-500 font-mono">
                Создано: {new Date(linkTasks[0].created_at).toLocaleString()}
              </div>
            </div>
          ) : (
            <div className="p-6 text-center border border-dashed border-slate-200 dark:border-slate-700 rounded-xl text-slate-500 text-xs">
              Задач еще нет.
            </div>
          )}

          {/* История задач (Компактный список) */}
          {linkTasks.length > 0 && (
            <div>
              <div className="flex items-center justify-between mb-2">
                <h4 className="text-[10px] font-bold uppercase tracking-wider text-slate-400 flex items-center gap-1">
                  <History className="w-3 h-3" /> История операций
                </h4>
                {linkTasks.length > 3 && (
                  <button
                    onClick={() => setShowAllLinkTasks((v) => !v)}
                    className="text-[10px] font-semibold text-indigo-500 hover:text-indigo-400 transition-colors">
                    {showAllLinkTasks ? 'Скрыть' : `Все (${linkTasks.length})`}
                  </button>
                )}
              </div>

              <div className="space-y-2">
                {visibleLinkTasks.map((task) => (
                  <div
                    key={task.id}
                    className="p-3 border border-slate-100 dark:border-slate-800 rounded-xl bg-white dark:bg-[#0a1020] flex items-center justify-between gap-3 shadow-sm">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 mb-1">
                        <LinkTaskStatusBadge status={task.status} />
                        <span className="text-xs font-bold text-slate-900 dark:text-slate-200">
                          {task.action === 'remove' ? 'Удаление' : 'Вставка'}
                        </span>
                      </div>
                      <div
                        className="text-[10px] text-slate-500 truncate"
                        title={task.found_location || task.error_message}>
                        {task.error_message ? (
                          <span className="text-red-500">{task.error_message}</span>
                        ) : task.found_location ? (
                          `В файле: ${task.found_location.split(':')[0]}`
                        ) : task.generated_content ? (
                          'Текст сгенерирован'
                        ) : (
                          '—'
                        )}
                      </div>
                    </div>
                    <div className="text-[10px] font-mono text-slate-400 flex-shrink-0 text-right">
                      {new Date(task.scheduled_for).toLocaleDateString()}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ================================================= */}
      {/* ТАБ: ЛОГИ И DIFF */}
      {/* ================================================= */}
      {linkTab === 'logs' && (
        <div className="space-y-5 animate-in fade-in">
          {linkTasks.length === 0 ? (
            <div className="text-xs text-slate-500 text-center p-6 border border-dashed border-slate-200 dark:border-slate-700 rounded-xl">
              Нет логов для отображения.
            </div>
          ) : (
            visibleLinkTasks.map((task, idx) => {
              const isRemove = (task.action || 'insert') === 'remove';
              const foundMeta = parseFoundLocation(task.found_location);
              const steps = getLinkTaskSteps(task.action);
              const reached = new Set<string>();
              const normStatus = normalizeLinkTaskStatus(task.status) || task.status;

              if (
                [
                  'pending',
                  'searching',
                  'inserted',
                  'generated',
                  'removing',
                  'removed',
                  'failed',
                ].includes(normStatus)
              )
                reached.add('pending');
              if (
                ['searching', 'inserted', 'generated', 'removing', 'removed'].includes(normStatus)
              )
                reached.add('searching');
              if (!isRemove) {
                if (['inserted'].includes(normStatus)) reached.add('inserted');
                if (['generated'].includes(normStatus)) reached.add('generated');
              } else {
                if (['removing', 'removed'].includes(normStatus)) reached.add('removing');
                if (['removed'].includes(normStatus)) reached.add('removed');
              }

              return (
                <div
                  key={task.id}
                  className="rounded-xl border border-slate-200 dark:border-slate-700/60 bg-white dark:bg-[#0a1020] shadow-sm overflow-hidden">
                  {/* Шапка лога */}
                  <div className="px-4 py-3 border-b border-slate-100 dark:border-slate-800/60 flex items-center justify-between">
                    <div className="font-bold text-[11px] uppercase tracking-wider text-slate-500 flex items-center gap-1.5">
                      <Terminal className="w-3.5 h-3.5" /> Лог #{idx + 1}
                    </div>
                    <LinkTaskStatusBadge status={task.status} />
                  </div>

                  <div className="p-4 space-y-4">
                    {/* Визуальный прогресс-бар */}
                    <div className="flex flex-wrap items-center gap-1.5">
                      {steps.map((step, i) => {
                        const isReached = reached.has(step.id);
                        return (
                          <div key={step.id} className="flex items-center gap-1.5">
                            <Badge
                              label={step.label}
                              tone={isReached ? 'emerald' : 'slate'}
                              icon={
                                isReached ? (
                                  <Check className="w-3 h-3" />
                                ) : (
                                  <Clock className="w-3 h-3" />
                                )
                              }
                              className="text-[9px] px-1.5 py-0.5"
                            />
                            {i < steps.length - 1 && (
                              <ArrowRight className="w-3 h-3 text-slate-300 dark:text-slate-700" />
                            )}
                          </div>
                        );
                      })}
                      {task.status === 'failed' && (
                        <div className="flex items-center gap-1.5">
                          <ArrowRight className="w-3 h-3 text-slate-300 dark:text-slate-700" />
                          <Badge
                            label="Сбой"
                            tone="red"
                            icon={<AlertTriangle className="w-3 h-3" />}
                            className="text-[9px] px-1.5 py-0.5"
                          />
                        </div>
                      )}
                    </div>

                    {/* Ошибки */}
                    {task.error_message && (
                      <div className="p-2.5 rounded-lg bg-red-50 border border-red-100 text-[11px] text-red-600 dark:bg-red-500/10 dark:border-red-500/20 dark:text-red-400 font-medium">
                        {task.error_message}
                      </div>
                    )}

                    {/* Diff */}
                    {task.found_location && (
                      <div className="space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-[10px] font-mono text-slate-500 bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 rounded border border-slate-200 dark:border-slate-700">
                            {task.found_location}
                          </span>

                          <button
                            onClick={() => loadLinkDiff(task)}
                            className="text-[10px] font-semibold text-indigo-600 hover:text-indigo-500 transition-colors flex items-center gap-1">
                            <Code2 className="w-3 h-3" /> Diff
                          </button>
                        </div>

                        {linkDiffs[task.id] && (
                          <div className="flex flex-col gap-2">
                            <div className="bg-[#1e1e1e] dark:bg-black/80 rounded-lg overflow-hidden border border-slate-700">
                              <div className="px-2 py-1 bg-[#2d2d2d] dark:bg-slate-900 border-b border-black/50 text-[9px] font-bold text-red-400 uppercase">
                                До
                              </div>
                              <div className="p-2 overflow-x-auto max-h-[150px]">
                                {renderDiffLines(
                                  linkDiffs[task.id].before,
                                  linkDiffs[task.id].after,
                                  'before',
                                )}
                              </div>
                            </div>
                            <div className="bg-[#1e1e1e] dark:bg-black/80 rounded-lg overflow-hidden border border-slate-700">
                              <div className="px-2 py-1 bg-[#2d2d2d] dark:bg-slate-900 border-b border-black/50 text-[9px] font-bold text-emerald-400 uppercase">
                                После
                              </div>
                              <div className="p-2 overflow-x-auto max-h-[150px]">
                                {renderDiffLines(
                                  linkDiffs[task.id].before,
                                  linkDiffs[task.id].after,
                                  'after',
                                )}
                              </div>
                            </div>
                          </div>
                        )}
                      </div>
                    )}

                    {/* Аккордеоны с сырыми логами */}
                    {(task.generated_content || (task.log_lines && task.log_lines.length > 0)) && (
                      <div className="flex flex-col gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
                        {task.generated_content && (
                          <details className="group">
                            <summary className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 cursor-pointer hover:text-indigo-500 transition-colors select-none flex items-center gap-1">
                              <ArrowRight className="w-3 h-3 group-open:rotate-90 transition-transform" />{' '}
                              Сгенерированный текст
                            </summary>
                            <pre className="mt-2 p-2.5 rounded-lg bg-slate-50 dark:bg-black border border-slate-200 dark:border-slate-800 text-[10px] font-mono text-slate-700 dark:text-slate-300 whitespace-pre-wrap">
                              {task.generated_content}
                            </pre>
                          </details>
                        )}

                        {task.log_lines && task.log_lines.length > 0 && (
                          <details className="group">
                            <summary className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 cursor-pointer hover:text-indigo-500 transition-colors select-none flex items-center gap-1">
                              <ArrowRight className="w-3 h-3 group-open:rotate-90 transition-transform" />{' '}
                              Лог воркера
                            </summary>
                            <pre className="mt-2 p-2.5 rounded-lg bg-[#1e1e1e] dark:bg-black text-[10px] font-mono text-emerald-400 whitespace-pre-wrap overflow-x-auto shadow-inner">
                              {task.log_lines.join('\n')}
                            </pre>
                          </details>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
