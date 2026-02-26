import { useState } from "react";
import { PromptOverridesPanel } from "../../../components/PromptOverridesPanel";

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
  canEditPrompts: boolean;
  onKwChange: (value: string) => void;
  onLinkAnchorChange: (value: string) => void;
  onLinkAcceptorChange: (value: string) => void;
  onServerChange: (value: string) => void;
  onCountryChange: (value: string) => void;
  onLanguageChange: (value: string) => void;
  onExcludeChange: (value: string) => void;
  onSave: () => void;
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
  onKwChange,
  onLinkAnchorChange,
  onLinkAcceptorChange,
  onServerChange,
  onCountryChange,
  onLanguageChange,
  onExcludeChange,
  onSave
}: DomainSettingsSectionProps) {
  const [showDomainPromptOverrides, setShowDomainPromptOverrides] = useState(false);

  return (
    <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <h3 className="font-semibold">Настройки домена</h3>
        <div className="grid gap-3 md:grid-cols-2">
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Ключевое слово</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={kw}
              onChange={(e) => onKwChange(e.target.value)}
              placeholder="casino ..."
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Анкор</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkAnchor}
              onChange={(e) => onLinkAnchorChange(e.target.value)}
              placeholder="Текст ссылки"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Акцептор</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkAcceptor}
              onChange={(e) => onLinkAcceptorChange(e.target.value)}
              placeholder="https://example.com"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Сервер</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={server}
              onChange={(e) => onServerChange(e.target.value)}
              placeholder="seotech-web-media1"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Страна</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={country}
              onChange={(e) => onCountryChange(e.target.value)}
              placeholder="se"
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Язык</span>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={language}
              onChange={(e) => onLanguageChange(e.target.value)}
              placeholder="sv-SE"
            />
          </label>
          <label className="text-sm space-y-1 md:col-span-2">
            <span className="text-slate-600 dark:text-slate-300">Исключить домены (через запятую)</span>
            <textarea
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              rows={2}
              value={exclude}
              onChange={(e) => onExcludeChange(e.target.value)}
              placeholder="https://example.com, https://www.foo.bar"
            />
          </label>
        </div>
        <button
          onClick={onSave}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          Сохранить
        </button>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h3 className="font-semibold">Промпты домена</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Переопределения промптов и моделей для этапов генерации.
            </p>
          </div>
          <button
            type="button"
            onClick={() => setShowDomainPromptOverrides((prev) => !prev)}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            {showDomainPromptOverrides ? "Скрыть блок" : "Показать блок"}
          </button>
        </div>
        {showDomainPromptOverrides ? (
          <PromptOverridesPanel
            title="Переопределения промптов (домен)"
            endpoint={`/api/domains/${domainId}/prompts`}
            canEdit={canEditPrompts}
            layout="single-stage"
          />
        ) : (
          <div className="rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2 text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-300">
            Блок скрыт. Нажмите «Показать блок», чтобы открыть настройки промптов домена.
          </div>
        )}
      </div>
    </>
  );
}
