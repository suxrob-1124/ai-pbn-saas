'use client';

import { useCallback, useMemo, useState } from 'react';
import { X, Loader2, CheckCircle, AlertTriangle, Server } from 'lucide-react';
import { previewLegacyImport } from '@/lib/legacyImportApi';
import type { LegacyImportPreviewItem } from '@/types/legacyImport';

interface DomainItem {
  id: string;
  url: string;
  deployment_mode?: string | null;
  server_id?: string | null;
}

interface Props {
  projectId: string;
  domains: DomainItem[];
  onClose: () => void;
  onStart: (domainIds: string[], force: boolean) => void;
}

export default function LegacyImportModal({ projectId, domains, onClose, onStart }: Props) {
  const [step, setStep] = useState<'select' | 'preview'>('select');
  const [selected, setSelected] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    for (const d of domains) {
      if (d.server_id && d.deployment_mode === 'ssh_remote') {
        initial.add(d.id);
      }
    }
    return initial;
  });
  const [force, setForce] = useState(false);
  const [previews, setPreviews] = useState<LegacyImportPreviewItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sshDomains = useMemo(
    () => domains.filter((d) => d.server_id),
    [domains]
  );

  const toggle = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      if (prev.size === sshDomains.length) return new Set();
      return new Set(sshDomains.map((d) => d.id));
    });
  }, [sshDomains]);

  const handleNext = useCallback(async () => {
    if (selected.size === 0) return;
    setLoading(true);
    setError(null);
    try {
      const data = await previewLegacyImport(projectId, Array.from(selected));
      setPreviews(data);
      setStep('preview');
    } catch (err: any) {
      setError(err?.message || 'Ошибка загрузки preview');
    } finally {
      setLoading(false);
    }
  }, [projectId, selected]);

  const handleStart = useCallback(() => {
    onStart(Array.from(selected), force);
  }, [selected, force, onStart]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
      <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-3xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800/60 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/20">
          <h3 className="text-lg font-bold text-slate-900 dark:text-white">
            Legacy Import
          </h3>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {step === 'select' && (
          <>
            <div className="p-6">
              {sshDomains.length === 0 ? (
                <div className="text-center py-8 text-slate-500 dark:text-slate-400">
                  <Server className="w-8 h-8 mx-auto mb-2 opacity-40" />
                  <p className="text-sm">Нет доменов с настроенным сервером</p>
                  <p className="text-xs mt-1 opacity-60">
                    Для Legacy Import нужен SSH-сервер
                  </p>
                </div>
              ) : (
                <>
                  <div className="flex items-center justify-between mb-3">
                    <p className="text-sm text-slate-600 dark:text-slate-300">
                      Выберите домены для импорта ({selected.size} из {sshDomains.length})
                    </p>
                    <button
                      onClick={toggleAll}
                      className="text-xs text-indigo-500 hover:text-indigo-600 dark:text-indigo-400"
                    >
                      {selected.size === sshDomains.length ? 'Снять все' : 'Выбрать все'}
                    </button>
                  </div>
                  <div className="max-h-[40vh] overflow-y-auto space-y-1">
                    {sshDomains.map((d) => (
                      <label
                        key={d.id}
                        className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-800/40 cursor-pointer transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={selected.has(d.id)}
                          onChange={() => toggle(d.id)}
                          className="rounded border-slate-300 dark:border-slate-600 text-indigo-500 focus:ring-indigo-500/20"
                        />
                        <span className="text-sm text-slate-700 dark:text-slate-300 font-mono">
                          {d.url}
                        </span>
                      </label>
                    ))}
                  </div>
                </>
              )}
              {error && (
                <p className="mt-3 text-sm text-red-500">{error}</p>
              )}
            </div>
            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800/60 flex justify-end gap-3">
              <button
                onClick={onClose}
                className="px-5 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors"
              >
                Отмена
              </button>
              <button
                onClick={handleNext}
                disabled={loading || selected.size === 0}
                className="px-6 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-xl disabled:opacity-50 transition-all shadow-sm active:scale-95 flex items-center gap-2"
              >
                {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                Далее
              </button>
            </div>
          </>
        )}

        {step === 'preview' && (
          <>
            <div className="p-6">
              <p className="text-sm text-slate-600 dark:text-slate-300 mb-4">
                Предварительная проверка доменов:
              </p>
              <div className="max-h-[40vh] overflow-y-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-slate-100 dark:border-slate-800/60 text-left text-xs text-slate-500 dark:text-slate-400">
                      <th className="py-2 font-medium">Домен</th>
                      <th className="py-2 font-medium">Статус</th>
                    </tr>
                  </thead>
                  <tbody>
                    {previews.map((p) => (
                      <tr key={p.domain_id} className="border-b border-slate-50 dark:border-slate-800/30">
                        <td className="py-2 text-slate-800 dark:text-slate-200 font-mono text-xs">
                          {p.domain_url}
                        </td>
                        <td className="py-2">
                          <PreviewStatus item={p} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <label className="flex items-center gap-3 mt-4 px-3 py-2.5 rounded-lg bg-amber-50/50 dark:bg-amber-900/10 border border-amber-100 dark:border-amber-800/30 cursor-pointer">
                <input
                  type="checkbox"
                  checked={force}
                  onChange={(e) => setForce(e.target.checked)}
                  className="rounded border-slate-300 dark:border-slate-600 text-amber-500 focus:ring-amber-500/20"
                />
                <div>
                  <span className="text-sm text-slate-700 dark:text-slate-300 font-medium">
                    Перезаписать существующие данные
                  </span>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                    Ссылки и артефакты будут пересозданы, даже если уже существуют
                  </p>
                </div>
              </label>
            </div>
            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800/60 flex justify-between">
              <button
                onClick={() => setStep('select')}
                className="px-5 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors"
              >
                Назад
              </button>
              <button
                onClick={handleStart}
                className="px-6 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-xl transition-all shadow-sm active:scale-95"
              >
                Запустить импорт
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function PreviewStatus({ item }: { item: LegacyImportPreviewItem }) {
  const notes: JSX.Element[] = [];

  if (!item.has_files && !item.has_link && !item.has_legacy_artifacts) {
    notes.push(
      <span key="new" className="flex items-center gap-1 text-emerald-500">
        <CheckCircle className="w-3 h-3" /> Полный импорт
      </span>
    );
  } else {
    if (item.has_files) {
      notes.push(
        <span key="files" className="text-slate-500">
          Файлы уже есть (будут обновлены)
        </span>
      );
    }
    if (item.has_link) {
      notes.push(
        <span key="link" className="flex items-center gap-1 text-amber-500">
          <AlertTriangle className="w-3 h-3" />
          Ссылка: {item.link_anchor} → {item.link_target}
        </span>
      );
    }
    if (item.has_legacy_artifacts) {
      notes.push(
        <span key="art" className="text-slate-500">
          Legacy артефакты существуют
        </span>
      );
    }
    if (item.has_non_legacy_gen) {
      notes.push(
        <span key="gen" className="flex items-center gap-1 text-amber-500">
          <AlertTriangle className="w-3 h-3" />
          Пользовательские генерации
        </span>
      );
    }
  }

  return (
    <div className="space-y-0.5 text-xs">
      {notes}
    </div>
  );
}
