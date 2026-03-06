import { useState } from 'react';
import { PromptOverridesPanel } from '../../../components/PromptOverridesPanel';
import { Save, ChevronDown, ChevronRight, Settings2, Globe, Database, Activity } from 'lucide-react';
import { GENERATION_TYPES, getGenerationTypeLabel } from '../services/generationTypes';
import type { GenerationType } from '../services/generationTypes';

type DomainSettingsSectionProps = {
  domainId: string;
  loading: boolean;
  kw: string;
  linkAnchor: string;
  linkAcceptor: string;
  server: string;
  country: string;
  language: string;
  exclude: string;
  generationType?: string;
  canEditPrompts: boolean;
  indexCheckEnabled?: boolean;
  indexCheckLoading?: boolean;
  onKwChange: (value: string) => void;
  onLinkAnchorChange: (value: string) => void;
  onLinkAcceptorChange: (value: string) => void;
  onServerChange: (value: string) => void;
  onCountryChange: (value: string) => void;
  onLanguageChange: (value: string) => void;
  onExcludeChange: (value: string) => void;
  onGenerationTypeChange?: (value: string) => void;
  onSave: () => void;
  onToggleIndexCheck?: (enabled: boolean) => void;
};

export function DomainSettingsSection({
  domainId,
  loading,
  kw,
  linkAnchor,
  linkAcceptor,
  server,
  country,
  language,
  exclude,
  canEditPrompts,
  generationType,
  indexCheckEnabled,
  indexCheckLoading,
  onKwChange,
  onLinkAnchorChange,
  onLinkAcceptorChange,
  onServerChange,
  onCountryChange,
  onLanguageChange,
  onExcludeChange,
  onGenerationTypeChange,
  onSave,
  onToggleIndexCheck,
}: DomainSettingsSectionProps) {
  const [showPrompts, setShowPrompts] = useState(false);

  // Единые стили для инпутов сайдбара
  const inputClass =
    'w-full bg-slate-50 dark:bg-[#060d18] border border-slate-200 dark:border-slate-700/80 rounded-xl px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:text-white transition-all shadow-sm';
  const labelClass =
    'text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1.5 block flex items-center gap-1.5';

  return (
    <div className="space-y-6">
      {/* 0. ТИП ГЕНЕРАЦИИ */}
      {onGenerationTypeChange && (
        <div>
          <label className={labelClass}>Тип генерации</label>
          <select
            className={inputClass}
            value={generationType || 'single_page'}
            onChange={(e) => onGenerationTypeChange(e.target.value)}>
            {(Object.entries(GENERATION_TYPES) as [GenerationType, { label: string; available: boolean }][]).map(
              ([key, { label, available }]) => (
                <option key={key} value={key} disabled={!available}>
                  {label}{!available ? ' (Скоро)' : ''}
                </option>
              ),
            )}
          </select>
        </div>
      )}

      {/* 1. БЛОК SEO & ГЕО */}
      <div className="space-y-4">
        <div>
          <label className={labelClass}>
            <Search className="w-3.5 h-3.5" /> Главный ключ (Keyword)
          </label>
          <input
            className={inputClass}
            value={kw}
            onChange={(e) => onKwChange(e.target.value)}
            placeholder="Например: best casino"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>
              <Globe className="w-3.5 h-3.5" /> Страна (Geo)
            </label>
            <input
              className={inputClass}
              value={country}
              onChange={(e) => onCountryChange(e.target.value)}
              placeholder="US, SE..."
            />
          </div>
          <div>
            <label className={labelClass}>Язык</label>
            <input
              className={inputClass}
              value={language}
              onChange={(e) => onLanguageChange(e.target.value)}
              placeholder="en-US"
            />
          </div>
        </div>
      </div>

      <div className="w-full h-px bg-slate-100 dark:bg-slate-800/80"></div>

      {/* 2. БЛОК ССЫЛОК (Link Flow Default Data) */}
      <div className="space-y-4">
        <div>
          <label className={labelClass}>
            <LinkIcon className="w-3.5 h-3.5" /> Анкор ссылки
          </label>
          <input
            className={inputClass}
            value={linkAnchor}
            onChange={(e) => onLinkAnchorChange(e.target.value)}
            placeholder="Текст ссылки..."
          />
        </div>
        <div>
          <label className={labelClass}>URL Акцептора</label>
          <input
            className={inputClass}
            value={linkAcceptor}
            onChange={(e) => onLinkAcceptorChange(e.target.value)}
            placeholder="https://target.com"
          />
        </div>
      </div>

      <div className="w-full h-px bg-slate-100 dark:bg-slate-800/80"></div>

      {/* 3. ТЕХНИЧЕСКИЕ НАСТРОЙКИ (Свернуты по умолчанию для чистоты) */}
      <div className="space-y-4">
        <details className="group">
          <summary className="flex items-center justify-between cursor-pointer list-none text-sm font-semibold text-slate-700 dark:text-slate-300 select-none">
            <span className="flex items-center gap-2">
              <Database className="w-4 h-4 text-indigo-500" /> Технические параметры
            </span>
            <ChevronDown className="w-4 h-4 opacity-50 group-open:rotate-180 transition-transform" />
          </summary>
          <div className="pt-4 space-y-4">
            <div>
              <label className={labelClass}>Целевой сервер</label>
              <input
                className={inputClass}
                value={server}
                onChange={(e) => onServerChange(e.target.value)}
                placeholder="seotech-web-media1"
              />
            </div>
            <div>
              <label className={labelClass}>Исключить домены (Exclude)</label>
              <textarea
                className={`${inputClass} min-h-[80px] resize-y text-xs`}
                value={exclude}
                onChange={(e) => onExcludeChange(e.target.value)}
                placeholder="domain1.com, domain2.net"
              />
            </div>
          </div>
        </details>
      </div>

      {/* КНОПКА СОХРАНЕНИЯ (Фиксирована внизу блока) */}
      <div className="pt-4 mt-6">
        <button
          onClick={onSave}
          disabled={loading}
          className="w-full flex justify-center items-center gap-2 bg-indigo-600 text-white px-5 py-2.5 rounded-xl text-sm font-bold hover:bg-indigo-500 transition-all shadow-sm active:scale-95 disabled:opacity-50">
          <Save className="w-4 h-4" /> {loading ? 'Сохранение...' : 'Сохранить настройки'}
        </button>
      </div>

      {/* ПРОВЕРКА ИНДЕКСАЦИИ */}
      {onToggleIndexCheck && (
        <>
          <div className="w-full h-px bg-slate-100 dark:bg-slate-800/80"></div>
          <div className="flex items-center justify-between py-1">
            <div className="flex items-center gap-2.5">
              <Activity className={`w-4 h-4 ${indexCheckEnabled ? 'text-emerald-500' : 'text-slate-400'}`} />
              <div>
                <div className="text-sm font-semibold text-slate-700 dark:text-slate-300">
                  Проверка индексации
                </div>
                <div className="text-[11px] text-slate-400 dark:text-slate-500">
                  Автоматическая ежедневная проверка
                </div>
              </div>
            </div>
            <button
              onClick={() => onToggleIndexCheck(!indexCheckEnabled)}
              disabled={indexCheckLoading}
              className={`relative inline-flex h-6 w-10 items-center rounded-full transition-colors focus:outline-none disabled:opacity-50 ${
                indexCheckEnabled ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-600'
              }`}>
              <span className={`inline-block h-4 w-4 rounded-full bg-white shadow-sm transform transition-transform ${
                indexCheckEnabled ? 'translate-x-5' : 'translate-x-1'
              }`} />
            </button>
          </div>
        </>
      )}

      <div className="w-full h-px bg-slate-100 dark:bg-slate-800/80"></div>

      {/* 4. ПЕРЕОПРЕДЕЛЕНИЕ ПРОМПТОВ */}
      <div>
        <button
          onClick={() => setShowPrompts(!showPrompts)}
          className="w-full flex items-center justify-between text-sm font-semibold text-slate-700 dark:text-slate-300 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors">
          <span className="flex items-center gap-2">
            <Settings2 className="w-4 h-4" /> Промпты домена
          </span>
          {showPrompts ? (
            <ChevronDown className="w-4 h-4 opacity-50" />
          ) : (
            <ChevronRight className="w-4 h-4 opacity-50" />
          )}
        </button>

        {showPrompts && (
          <div className="mt-4 animate-in slide-in-from-top-2">
            <PromptOverridesPanel
              title="Локальные оверрайды"
              endpoint={`/api/domains/${domainId}/prompts`}
              canEdit={canEditPrompts}
              layout="single-stage"
              mode="modal"
            />
          </div>
        )}
      </div>
    </div>
  );
}

// Импорт недостающих иконок
import { Search, Link as LinkIcon } from 'lucide-react';
