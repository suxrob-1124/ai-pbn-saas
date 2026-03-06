'use client';

import { useEffect, useMemo, useState } from 'react';
import { RefreshCw, Save, Trash2, Code2, Info, X, Maximize2 } from 'lucide-react';
import Editor from '@monaco-editor/react';

import { authFetch } from '../lib/http';
import { showToast } from '../lib/toastStore';
import { useTheme } from '../lib/useTheme';
import { Badge } from './Badge';
import { PromptVariablesHelp } from './PromptVariablesHelp';

type PromptOverrideDTO = {
  id: string;
  scope_type: string;
  scope_id: string;
  stage: string;
  body: string;
  model?: string;
};
type ResolvedPromptDTO = {
  stage: string;
  source: 'domain' | 'project' | 'global' | string;
  prompt_id?: string;
  override_id?: string;
  body: string;
  model?: string;
};
type PromptResponseDTO = { overrides: PromptOverrideDTO[]; resolved: ResolvedPromptDTO[] };

type PromptOverridesPanelProps = {
  title: string;
  endpoint: string;
  canEdit: boolean;
  mode?: 'full' | 'modal'; // "full" для админки, "modal" для настроек сайдбара
  layout?: 'list' | 'single-stage'; // Оставлен для легаси совместимости, но теперь не так важен
};

const SOURCE_LABELS: Record<string, string> = {
  domain: 'Домен',
  project: 'Проект',
  global: 'Базовый',
};
const STAGE_LABELS: Record<string, string> = {
  competitor_analysis: 'Анализ конкурентов',
  technical_spec: 'Техническое задание',
  content_generation: 'Генерация контента',
  design_architecture: 'Дизайн-архитектура',
  logo_generation: 'Генерация логотипа',
  html_generation: 'Генерация HTML',
  css_generation: 'Генерация CSS',
  js_generation: 'Генерация JavaScript',
  image_prompt_generation: 'Промпты для изображений',
  '404_page': 'Генерация 404',
  editor_file_edit: 'AI: редактирование файла',
  editor_page_create: 'AI: создание страницы',
};

const MODEL_OPTIONS = [
  {
    value: '',
    label: `По умолчанию (${process.env.NEXT_PUBLIC_GEMINI_DEFAULT_MODEL || 'gemini-2.5-pro'})`,
  },
  { value: 'gemini-3-pro-preview', label: 'gemini-3-pro-preview' },
  { value: 'gemini-2.5-pro', label: 'gemini-2.5-pro' },
  { value: 'gemini-2.5-flash', label: 'gemini-2.5-flash' },
  { value: 'gemini-2.5-flash-image', label: 'gemini-2.5-flash-image' },
  { value: 'gemini-1.5-pro', label: 'gemini-1.5-pro' },
  { value: 'gemini-1.5-flash', label: 'gemini-1.5-flash' },
];

const PROMPT_VARIABLES = [
  { name: '{{ keyword }}', desc: 'Главное ключевое слово сайта' },
  { name: '{{ country }}', desc: 'Страна (гео), напр.: US, SE' },
  { name: '{{ language }}', desc: 'Язык текста, напр.: en-US, sv-SE' },
  { name: '{{ analysis_data }}', desc: 'Анализ конкурентов (JSON)' },
  { name: '{{ tech_spec }}', desc: 'Готовое ТЗ для копирайтера' },
  { name: '{{ contents_data }}', desc: 'Структура статьи (H2-H3 заголовки)' },
  { name: '{{ html_content }}', desc: 'Контент страницы в HTML' },
];

export function PromptOverridesPanel({
  title,
  endpoint,
  canEdit,
  mode = 'full',
}: PromptOverridesPanelProps) {
  const { theme } = useTheme();
  const [isOpen, setIsOpen] = useState(false); // Для модального режима
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<PromptResponseDTO>({ overrides: [], resolved: [] });
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [modelDrafts, setModelDrafts] = useState<Record<string, string>>({});
  const [selectedStage, setSelectedStage] = useState('');

  const overrideByStage = useMemo(() => {
    const map: Record<string, PromptOverrideDTO> = {};
    response.overrides.forEach((item) => {
      map[item.stage] = item;
    });
    return map;
  }, [response.overrides]);

  const load = async () => {
    if (!endpoint) return;
    setLoading(true);
    try {
      const data = await authFetch<PromptResponseDTO>(endpoint);
      setResponse({
        overrides: Array.isArray(data?.overrides) ? data.overrides : [],
        resolved: Array.isArray(data?.resolved) ? data.resolved : [],
      });
      setDrafts((prev) => {
        const next = { ...prev };
        (Array.isArray(data?.resolved) ? data.resolved : []).forEach((item) => {
          const fromOverride = (Array.isArray(data?.overrides) ? data.overrides : []).find(
            (ov) => ov.stage === item.stage,
          );
          if (!next[item.stage]) next[item.stage] = (fromOverride?.body || item.body || '').trim();
        });
        return next;
      });
      setModelDrafts((prev) => {
        const next = { ...prev };
        (Array.isArray(data?.resolved) ? data.resolved : []).forEach((item) => {
          const fromOverride = (Array.isArray(data?.overrides) ? data.overrides : []).find(
            (ov) => ov.stage === item.stage,
          );
          if (typeof next[item.stage] === 'undefined')
            next[item.stage] = (fromOverride?.model || item.model || '').trim();
        });
        return next;
      });
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: 'Не удалось загрузить промпты' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    // В модальном режиме грузим данные только когда открываем окно
    if (mode === 'full' || isOpen) {
      void load();
    }
  }, [endpoint, mode, isOpen]);

  useEffect(() => {
    if (!response.resolved.length) return;
    const hasCurrent = response.resolved.some((item) => item.stage === selectedStage);
    if (!hasCurrent) setSelectedStage(response.overrides[0]?.stage || response.resolved[0].stage);
  }, [response.overrides, response.resolved, selectedStage]);

  const onSave = async (stage: string) => {
    const body = (drafts[stage] || '').trim();
    const model = (modelDrafts[stage] || '').trim();
    if (!body)
      return showToast({
        type: 'error',
        title: 'Пустой оверрайд',
        message: 'Введите текст промпта.',
      });
    setLoading(true);
    try {
      await authFetch(`${endpoint}/${stage}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ body, model }),
      });
      showToast({ type: 'success', title: 'Сохранено', message: STAGE_LABELS[stage] || stage });
      await load();
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setLoading(false);
    }
  };

  const onReset = async (stage: string) => {
    setLoading(true);
    try {
      await authFetch(`${endpoint}/${stage}`, { method: 'DELETE' });
      setDrafts((prev) => ({ ...prev, [stage]: '' }));
      showToast({ type: 'success', title: 'Удалено', message: STAGE_LABELS[stage] || stage });
      await load();
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка удаления', message: err?.message });
    } finally {
      setLoading(false);
    }
  };

  const visibleResolved = useMemo(() => {
    return selectedStage ? response.resolved.filter((item) => item.stage === selectedStage) : [];
  }, [response.resolved, selectedStage]);

  // Считаем, сколько всего кастомных оверрайдов сделано
  const activeOverridesCount = response.overrides.length;

  // --- КОМПОНЕНТ САМОГО РЕДАКТОРА (Чтобы не дублировать код) ---
  const EditorContent = () => (
    <div className="flex flex-col h-full bg-white dark:bg-[#0f1523] rounded-2xl shadow-sm overflow-hidden flex-1 xl:flex-row">
      {/* ЛЕВЫЙ САЙДБАР: Список этапов */}
      <div className="w-full xl:w-72 border-r border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-[#0a1020] flex flex-col max-h-[300px] xl:max-h-full">
        <div className="p-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
          <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2 text-sm">
            <Code2 className="w-4 h-4 text-indigo-500" /> {title}
          </h3>
          <button
            onClick={() => void load()}
            disabled={loading}
            className="text-slate-400 hover:text-indigo-500 transition-colors">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
        <div className="p-3 space-y-1 overflow-y-auto flex-1">
          {response.resolved.map((item) => {
            const hasOverride = Boolean(overrideByStage[item.stage]);
            const isSelected = selectedStage === item.stage;

            return (
              <button
                key={item.stage}
                onClick={() => setSelectedStage(item.stage)}
                className={`w-full text-left px-3 py-2 rounded-xl text-sm transition-all flex items-center justify-between ${
                  isSelected
                    ? 'bg-indigo-600 text-white shadow-md'
                    : 'text-slate-600 hover:bg-slate-200 dark:text-slate-400 dark:hover:bg-slate-800/50'
                }`}>
                <div className="truncate pr-2">
                  <div className="font-medium truncate">
                    {STAGE_LABELS[item.stage] || item.stage}
                  </div>
                  <div
                    className={`text-[10px] mt-0.5 ${isSelected ? 'text-indigo-200' : 'text-slate-400 dark:text-slate-500'}`}>
                    {SOURCE_LABELS[item.source] || item.source}
                  </div>
                </div>
                {hasOverride && (
                  <div
                    className={`w-2 h-2 rounded-full flex-shrink-0 ${isSelected ? 'bg-white' : 'bg-indigo-500'}`}
                    title="Есть переопределение"
                  />
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* ПРАВАЯ ЧАСТЬ: Редактор */}
      <div className="flex-1 flex flex-col relative min-h-[500px]">
        {selectedStage ? (
          visibleResolved.map((item) => {
            const override = overrideByStage[item.stage];
            const draft = drafts[item.stage] ?? (override?.body || '');
            const modelDraft = modelDrafts[item.stage] ?? (override?.model || item.model || '');
            const isOverridden = Boolean(override);

            return (
              <div key={item.stage} className="flex flex-col h-full animate-in fade-in">
                {/* Шапка редактора */}
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <h4 className="text-lg font-bold text-slate-900 dark:text-white">
                      {STAGE_LABELS[item.stage] || item.stage}
                    </h4>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-1 font-mono">
                      {item.stage}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <select
                      value={modelDraft}
                      disabled={!canEdit}
                      onChange={(e) =>
                        setModelDrafts((prev) => ({ ...prev, [item.stage]: e.target.value }))
                      }
                      className="bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 px-3 py-1.5 rounded-lg text-sm outline-none focus:border-indigo-500 transition-all dark:text-slate-200">
                      {MODEL_OPTIONS.map((opt) => (
                        <option key={opt.value || 'def'} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                {/* Рабочая зона */}
                <div className="flex-1 flex flex-col xl:flex-row min-h-0">
                  <div className="flex-1 flex flex-col bg-[#1e1e1e] dark:bg-[#060d18] relative min-h-[300px]">
                    <div className="p-2 border-b border-[#2d2d2d] dark:border-slate-800 flex justify-between items-center text-[10px] font-mono text-[#858585]">
                      <span>
                        {isOverridden ? 'override.txt' : 'system_prompt.txt'} (Read-only:{' '}
                        {item.source})
                      </span>
                      <span className="bg-[#2d2d2d] px-2 py-0.5 rounded text-white">
                        {isOverridden ? 'EDITING' : 'VIEWING'}
                      </span>
                    </div>
                    <div className="flex-1 relative">
                      <Editor
                        height="100%"
                        language="handlebars"
                        theme={theme === 'dark' ? 'vs-dark' : 'vs-dark'}
                        value={canEdit ? draft : item.body}
                        onChange={(val) =>
                          setDrafts((prev) => ({ ...prev, [item.stage]: val || '' }))
                        }
                        options={{
                          minimap: { enabled: false },
                          wordWrap: 'on',
                          fontSize: 13,
                          padding: { top: 16 },
                          readOnly: !canEdit,
                          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
                        }}
                      />
                    </div>
                  </div>

                  {/* Шпаргалка */}
                  <div className="w-full xl:w-64 border-l border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] p-4 overflow-y-auto hidden md:block max-h-[300px] xl:max-h-full">
                    <h4 className="text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-3 flex items-center gap-1.5">
                      <Info className="w-3.5 h-3.5" /> Переменные
                    </h4>
                    <div className="space-y-3">
                      {PROMPT_VARIABLES.map((v) => (
                        <div key={v.name} className="group">
                          <code className="text-xs font-bold text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded select-all block w-fit mb-1">
                            {v.name}
                          </code>
                          <p className="text-[11px] text-slate-500 leading-snug">{v.desc}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                {/* Подвал с кнопками */}
                {canEdit && (
                  <div className="p-4 border-t border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-900/20 flex items-center justify-end gap-3 flex-shrink-0">
                    {isOverridden && (
                      <button
                        onClick={() => void onReset(item.stage)}
                        disabled={loading}
                        className="px-4 py-2 rounded-xl text-sm font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 transition-colors mr-auto">
                        Сбросить до системного
                      </button>
                    )}
                    <button
                      onClick={() => void onSave(item.stage)}
                      disabled={loading}
                      className="px-6 py-2 rounded-xl text-sm font-semibold bg-indigo-600 text-white hover:bg-indigo-500 transition-all shadow-sm active:scale-95 flex items-center gap-2">
                      <Save className="w-4 h-4" /> {loading ? 'Сохранение...' : 'Сохранить'}
                    </button>
                  </div>
                )}
              </div>
            );
          })
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 p-10 bg-slate-50/50 dark:bg-transparent">
            <Code2 className="w-16 h-16 mb-4 opacity-20" />
            <h3 className="text-lg font-medium text-slate-900 dark:text-white mb-1">
              Prompt Studio
            </h3>
            <p className="text-sm max-w-sm text-center">
              Выберите этап генерации слева для настройки инструкций AI.
            </p>
          </div>
        )}
      </div>
    </div>
  );

  // --- РЕЖИМ: ADMIM PAGE (Полноэкранный, как было) ---
  if (mode === 'full') {
    if (response.resolved.length === 0 && !loading)
      return <div className="text-sm text-slate-500">Промпты еще не настроены.</div>;
    return <EditorContent />;
  }

  // --- РЕЖИМ: MODAL (Для сайдбара Проекта / Домена) ---
  return (
    <>
      <button
        onClick={() => setIsOpen(true)}
        className="w-full flex items-center justify-between px-4 py-3 bg-slate-50 dark:bg-[#0a1020] border border-slate-200 dark:border-slate-700/80 rounded-xl hover:border-indigo-400 dark:hover:border-indigo-500/50 transition-all group">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-lg bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
            <Code2 className="w-4 h-4" />
          </div>
          <div className="text-left">
            <div className="text-sm font-bold text-slate-900 dark:text-white leading-tight">
              Prompt Studio
            </div>
            <div className="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
              {activeOverridesCount > 0 ? (
                <span className="text-amber-600 dark:text-amber-400 font-medium">
                  {activeOverridesCount} кастомных правил
                </span>
              ) : (
                'Все промпты системные'
              )}
            </div>
          </div>
        </div>
        <Maximize2 className="w-4 h-4 text-slate-400 group-hover:text-indigo-500 transition-colors" />
      </button>

      {/* Огромное модальное окно */}
      {isOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 sm:p-6 bg-slate-900/60 backdrop-blur-sm animate-in fade-in">
          <div className="w-full max-w-6xl h-full max-h-[85vh] flex flex-col bg-slate-100 dark:bg-[#060d18] rounded-2xl shadow-2xl overflow-hidden border border-slate-200 dark:border-slate-700/80 animate-in zoom-in-95 duration-200">
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center bg-white dark:bg-[#0f1523]">
              <h2 className="text-xl font-bold text-slate-900 dark:text-white flex items-center gap-2">
                <Code2 className="w-5 h-5 text-indigo-500" /> {title}
              </h2>
              <button
                onClick={() => setIsOpen(false)}
                className="p-2 bg-slate-100 dark:bg-slate-800 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded-full transition-colors">
                <X className="w-5 h-5" />
              </button>
            </div>

            {loading && response.resolved.length === 0 ? (
              <div className="flex-1 flex items-center justify-center text-slate-500">
                <RefreshCw className="w-5 h-5 animate-spin mr-2" /> Загрузка промптов...
              </div>
            ) : (
              <div className="flex-1 min-h-0 overflow-hidden p-0 sm:p-4">
                <EditorContent />
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
}
