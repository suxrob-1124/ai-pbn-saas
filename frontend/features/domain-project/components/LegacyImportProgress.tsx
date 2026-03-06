'use client';

import { X, CheckCircle, XCircle, AlertTriangle, Loader2 } from 'lucide-react';
import type { LegacyImportJobDetail, LegacyImportItem } from '@/types/legacyImport';
import { getStepLabel, getStatusLabel, getStatusColor } from '../services/legacyImportLabels';

interface Props {
  job: LegacyImportJobDetail;
  items: LegacyImportItem[];
  onClose: () => void;
}

export default function LegacyImportProgress({ job, items, onClose }: Props) {
  const processed = job.CompletedItems + job.FailedItems + job.SkippedItems;
  const total = job.TotalItems || 1;
  const pct = Math.round((processed / total) * 100);
  const isFinished = job.Status === 'completed' || job.Status === 'failed';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
      <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-4xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800/60 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/20">
          <div className="flex items-center gap-3">
            <h3 className="text-lg font-bold text-slate-900 dark:text-white">
              Legacy Import
            </h3>
            {!isFinished && (
              <Loader2 className="w-4 h-4 text-indigo-500 animate-spin" />
            )}
            {isFinished && job.Status === 'completed' && (
              <CheckCircle className="w-4 h-4 text-emerald-500" />
            )}
            {isFinished && job.Status === 'failed' && (
              <XCircle className="w-4 h-4 text-red-500" />
            )}
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Progress bar */}
        <div className="px-6 pt-4 pb-2">
          <div className="flex justify-between text-xs text-slate-500 dark:text-slate-400 mb-1.5">
            <span>{processed} / {job.TotalItems} доменов</span>
            <span>{pct}%</span>
          </div>
          <div className="w-full h-2 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
            <div
              className="h-full bg-indigo-500 rounded-full transition-all duration-500"
              style={{ width: `${pct}%` }}
            />
          </div>
          {isFinished && (
            <div className="mt-2 text-sm text-slate-600 dark:text-slate-300">
              {job.CompletedItems > 0 && <span className="text-emerald-500 font-medium">{job.CompletedItems} успешно</span>}
              {job.FailedItems > 0 && <span className="text-red-500 font-medium ml-3">{job.FailedItems} ошибок</span>}
              {job.SkippedItems > 0 && <span className="text-amber-500 font-medium ml-3">{job.SkippedItems} пропущено</span>}
            </div>
          )}
        </div>

        {/* Items table */}
        <div className="px-6 pb-4 max-h-[50vh] overflow-y-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-100 dark:border-slate-800/60 text-left text-xs text-slate-500 dark:text-slate-400">
                <th className="py-2 font-medium">Домен</th>
                <th className="py-2 font-medium">Шаг</th>
                <th className="py-2 font-medium w-32">Прогресс</th>
                <th className="py-2 font-medium">Статус</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.ID} className="border-b border-slate-50 dark:border-slate-800/30">
                  <td className="py-2 text-slate-800 dark:text-slate-200 font-mono text-xs">
                    {item.DomainURL}
                  </td>
                  <td className="py-2 text-slate-600 dark:text-slate-400 text-xs">
                    {getStepLabel(item.Step)}
                  </td>
                  <td className="py-2">
                    <div className="w-full h-1.5 bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-indigo-400 rounded-full transition-all duration-300"
                        style={{ width: `${item.Progress}%` }}
                      />
                    </div>
                  </td>
                  <td className={`py-2 text-xs font-medium ${getStatusColor(item.Status)}`}>
                    <div className="flex items-center gap-1.5">
                      {item.Status === 'running' && <Loader2 className="w-3 h-3 animate-spin" />}
                      {item.Status === 'success' && <CheckCircle className="w-3 h-3" />}
                      {item.Status === 'failed' && <XCircle className="w-3 h-3" />}
                      {item.Status === 'skipped' && <AlertTriangle className="w-3 h-3" />}
                      {getStatusLabel(item.Status)}
                    </div>
                    {item.Error?.Valid && item.Error.String && (
                      <div className="text-[10px] text-red-400 mt-0.5 truncate max-w-[200px]" title={item.Error.String}>
                        {item.Error.String}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Footer */}
        <div className="px-6 py-3 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800/60 flex justify-end">
          <button
            onClick={onClose}
            className="px-5 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors"
          >
            {isFinished ? 'Закрыть' : 'Свернуть'}
          </button>
        </div>
      </div>
    </div>
  );
}
