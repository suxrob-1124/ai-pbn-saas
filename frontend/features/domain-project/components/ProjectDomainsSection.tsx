import Link from "next/link";
import type { UrlObject } from "url";
import type { ReactNode } from "react";
import { FiActivity, FiCheck, FiEdit2, FiInfo, FiLink, FiList, FiRefreshCw, FiTrash2 } from "react-icons/fi";
import { Badge } from "../../../components/Badge";
import { DOMAIN_PROJECT_CTA, getLinkActionLabel } from "../services/statusCta";
import { hasInsertedLink, isLinkTaskInProgress } from "../../../lib/linkTaskStatus";

type DomainCard = {
  id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  link_status?: string;
  link_status_effective?: string;
  updated_at?: string;
  last_success_generation_id?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_ready_at?: string;
};

type LinkEditValue = { anchor: string; acceptor: string };

type ProjectDomainsSectionProps = {
  loading: boolean;
  url: string;
  keyword: string;
  country: string;
  language: string;
  exclude: string;
  importText: string;
  domainSearch: string;
  domainsCount: number;
  filteredDomains: DomainCard[];
  linkLoadingId: string | null;
  openRuns: Record<string, boolean>;
  gens: Record<string, any[]>;
  keywordEdits: Record<string, string>;
  linkEdits: Record<string, LinkEditValue>;
  onUrlChange: (value: string) => void;
  onKeywordChange: (value: string) => void;
  onCountryChange: (value: string) => void;
  onLanguageChange: (value: string) => void;
  onExcludeChange: (value: string) => void;
  onImportTextChange: (value: string) => void;
  onDomainSearchChange: (value: string) => void;
  onAddDomain: () => void;
  onImportDomains: () => void;
  onRunLinkTask: (domainId: string) => void;
  onRemoveLinkTask: (domainId: string) => void;
  onLoadRuns: (domainId: string) => void;
  onDeleteDomain: (domainId: string) => void;
  onKeywordEditChange: (domainId: string, value: string) => void;
  onUpdateKeyword: (domainId: string) => void;
  onLinkEditChange: (domainId: string, value: LinkEditValue) => void;
  onUpdateLinkSettings: (domainId: string) => void;
  getEffectiveLinkStatus: (domain: DomainCard) => string;
  renderStatusBadge: (status: string) => ReactNode;
  renderLinkStatusBadge: (domain: DomainCard) => ReactNode;
  renderRunsList: (runs: any[]) => ReactNode;
  formatDateTime: (value?: string) => string;
  formatRelativeTime: (target: Date) => string;
};

export function ProjectDomainsSection({
  loading,
  url,
  keyword,
  country,
  language,
  exclude,
  importText,
  domainSearch,
  domainsCount,
  filteredDomains,
  linkLoadingId,
  openRuns,
  gens,
  keywordEdits,
  linkEdits,
  onUrlChange,
  onKeywordChange,
  onCountryChange,
  onLanguageChange,
  onExcludeChange,
  onImportTextChange,
  onDomainSearchChange,
  onAddDomain,
  onImportDomains,
  onRunLinkTask,
  onRemoveLinkTask,
  onLoadRuns,
  onDeleteDomain,
  onKeywordEditChange,
  onUpdateKeyword,
  onLinkEditChange,
  onUpdateLinkSettings,
  getEffectiveLinkStatus,
  renderStatusBadge,
  renderLinkStatusBadge,
  renderRunsList,
  formatDateTime,
  formatRelativeTime
}: ProjectDomainsSectionProps) {
  return (
    <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3 mb-3">
        <h3 className="font-semibold">Добавить домен</h3>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="example.com"
            value={url}
            onChange={(e) => onUrlChange(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Ключевое слово"
            value={keyword}
            onChange={(e) => onKeywordChange(e.target.value)}
          />
          <div className="flex gap-2">
            <button
              onClick={onAddDomain}
              disabled={loading || !url.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              Добавить
            </button>
          </div>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Страна (по умолчанию из проекта)"
            value={country}
            onChange={(e) => onCountryChange(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Язык (по умолчанию из проекта)"
            value={language}
            onChange={(e) => onLanguageChange(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Исключить домены (через запятую)"
            value={exclude}
            onChange={(e) => onExcludeChange(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Импорт по строкам в формате{" "}
            <code>url[,keyword[,country[,language[,server_id[,anchor[,acceptor]]]]]]</code>. Пример:{" "}
            <code>example.com,casino,se,sv,seotech-web-media1,"Лучший бонус","https://acceptor.example/page"</code>. Проверка
            существования домена на сервере `media1` будет добавлена на этапе серверной интеграции.
          </p>
          <textarea
            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            rows={4}
            placeholder={"example.com,casino,se,sv,seotech-web-media1,\"Лучший бонус\",\"https://acceptor.example/page\"\nexample.org"}
            value={importText}
            onChange={(e) => onImportTextChange(e.target.value)}
          />
          <button
            onClick={onImportDomains}
            disabled={loading || !importText.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
          >
            Импортировать
          </button>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <h3 className="font-semibold">Домены</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">
            Показано: {filteredDomains.length} из {domainsCount}
          </span>
        </div>
        <input
          type="search"
          value={domainSearch}
          onChange={(e) => onDomainSearchChange(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        />
        {filteredDomains.length === 0 && <div className="text-sm text-slate-500 dark:text-slate-400">Домены не найдены.</div>}
        <div className="space-y-3">
          {filteredDomains.map((domain) => {
            const linkStatus = getEffectiveLinkStatus(domain);
            const hasActiveLink = hasInsertedLink(linkStatus);
            const linkInProgress = isLinkTaskInProgress(linkStatus);
            const canRemoveLink = hasActiveLink && !linkInProgress;
            const isPublished = (domain.status || "").toLowerCase() === "published";
            const linkReadyAtDate = domain.link_ready_at ? new Date(domain.link_ready_at) : null;
            const linkReadyValid = linkReadyAtDate && !Number.isNaN(linkReadyAtDate.getTime());
            const linkReadyFuture = Boolean(linkReadyValid && linkReadyAtDate!.getTime() > Date.now());
            const linkReadyBadge = !linkReadyValid
              ? { label: "не задано", tone: "slate" as const }
              : linkReadyFuture
              ? { label: "ожидает", tone: "amber" as const }
              : { label: "готово", tone: "green" as const };
            const keywordValue = keywordEdits[domain.id] ?? "";
            const keywordDirty = keywordValue.trim() !== (domain.main_keyword || "").trim();
            const link = linkEdits[domain.id] || { anchor: domain.link_anchor_text || "", acceptor: domain.link_acceptor_url || "" };
            const anchorValue = link.anchor ?? "";
            const acceptorValue = link.acceptor ?? "";
            const linkDirty =
              anchorValue.trim() !== (domain.link_anchor_text || "").trim() ||
              acceptorValue.trim() !== (domain.link_acceptor_url || "").trim();

            return (
              <div
                key={domain.id}
                className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 p-4 shadow-sm space-y-3"
              >
                <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Link href={`/domains/${domain.id}`} className="text-indigo-600 hover:underline font-semibold">
                        {domain.url}
                      </Link>
                      {renderStatusBadge(domain.status)}
                      <Badge
                        label={`Попытка: ${domain.status}`}
                        tone={["processing", "pending", "pause_requested", "cancelling"].includes((domain.status || "").toLowerCase()) ? "amber" : "slate"}
                        className="text-[11px]"
                      />
                      <Badge
                        label={domain.last_success_generation_id ? "Успех: есть" : "Успех: нет"}
                        tone={domain.last_success_generation_id ? "green" : "slate"}
                        className="text-[11px]"
                      />
                      {renderLinkStatusBadge(domain)}
                    </div>
                    <div className="text-xs text-slate-500 dark:text-slate-400">Обновлено: {formatDateTime(domain.updated_at)}</div>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                      <span>Готовность ссылок: {linkReadyValid ? formatDateTime(domain.link_ready_at) : "не задано"}</span>
                      <Badge label={linkReadyBadge.label} tone={linkReadyBadge.tone} className="text-[11px]" />
                      {linkReadyFuture && linkReadyAtDate && (
                        <span className="text-amber-600 dark:text-amber-300">через {formatRelativeTime(linkReadyAtDate)}</span>
                      )}
                    </div>
                    <div className="text-xs text-slate-400">
                      Страна: {domain.target_country || "—"} · Язык: {domain.target_language || "—"}
                    </div>
                    {domain.exclude_domains && <div className="text-xs text-slate-400">Исключить: {domain.exclude_domains}</div>}
                  </div>
                  <div className="flex flex-wrap items-center gap-2 md:justify-end">
                    <button
                      onClick={() => onRunLinkTask(domain.id)}
                      disabled={loading || linkLoadingId === domain.id || linkInProgress}
                      className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      <FiLink />
                      {getLinkActionLabel(hasActiveLink, linkInProgress, true)}
                    </button>
                    <button
                      onClick={() => onRunLinkTask(domain.id)}
                      disabled={loading || linkLoadingId === domain.id || linkInProgress}
                      className="hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      title={linkInProgress ? DOMAIN_PROJECT_CTA.linkTaskInProgressShort : getLinkActionLabel(hasActiveLink, linkInProgress, true)}
                      aria-label={linkInProgress ? DOMAIN_PROJECT_CTA.linkTaskInProgressShort : getLinkActionLabel(hasActiveLink, linkInProgress, true)}
                    >
                      <FiLink />
                    </button>
                    {canRemoveLink ? (
                      <>
                        <button
                          onClick={() => onRemoveLinkTask(domain.id)}
                          disabled={loading || linkLoadingId === domain.id}
                          className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                        >
                          <FiTrash2 />
                          {DOMAIN_PROJECT_CTA.linkRemove}
                        </button>
                        <button
                          onClick={() => onRemoveLinkTask(domain.id)}
                          disabled={loading || linkLoadingId === domain.id}
                          className="hidden items-center justify-center rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                          title={DOMAIN_PROJECT_CTA.linkRemove}
                          aria-label={DOMAIN_PROJECT_CTA.linkRemove}
                        >
                          <FiTrash2 />
                        </button>
                      </>
                    ) : (
                      <>
                        {linkInProgress ? (
                          <span className="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-[11px] font-semibold text-amber-600 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                            <FiRefreshCw className="h-3 w-3" /> Выполняется
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 rounded-full border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                            <FiInfo className="h-3 w-3" /> Нет ссылки
                          </span>
                        )}
                      </>
                    )}
                    <button
                      onClick={() => onLoadRuns(domain.id)}
                      className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      <FiList /> {DOMAIN_PROJECT_CTA.runs} {openRuns[domain.id] && gens[domain.id] && `(${gens[domain.id].length})`}
                    </button>
                    <button
                      onClick={() => onLoadRuns(domain.id)}
                      className="hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      title={`${DOMAIN_PROJECT_CTA.runs} ${openRuns[domain.id] && gens[domain.id] ? `(${gens[domain.id].length})` : ""}`}
                      aria-label={DOMAIN_PROJECT_CTA.runs}
                    >
                      <FiList />
                    </button>
                    {isPublished ? (
                      <>
                        <Link
                          href={{ pathname: `/domains/${domain.id}/editor` } as UrlObject}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          <FiEdit2 /> Редактор
                        </Link>
                        <Link
                          href={{ pathname: `/domains/${domain.id}/editor` } as UrlObject}
                          className="hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                          title="Редактор"
                          aria-label="Открыть редактор"
                        >
                          <FiEdit2 />
                        </Link>
                      </>
                    ) : (
                      <>
                        <span
                          title="Редактор доступен после публикации сайта"
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
                        >
                          <FiEdit2 /> Редактор
                        </span>
                        <span
                          title="Редактор доступен после публикации сайта"
                          aria-label="Редактор доступен после публикации"
                          className="hidden items-center justify-center rounded-lg border border-slate-200 bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
                        >
                          <FiEdit2 />
                        </span>
                      </>
                    )}
                    {isPublished ? (
                      <>
                        <Link
                          href={{ pathname: "/monitoring/indexing", query: { domainId: domain.id } } as UrlObject}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          <FiActivity /> Проверки индексации
                        </Link>
                        <Link
                          href={{ pathname: "/monitoring/indexing", query: { domainId: domain.id } } as UrlObject}
                          className="hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                          title="Проверки индексации"
                          aria-label="Проверки индексации"
                        >
                          <FiActivity />
                        </Link>
                      </>
                    ) : (
                      <>
                        <span
                          title="Доступно после публикации сайта"
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-100 px-3 py-1 text-xs font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
                        >
                          <FiActivity /> Проверки индексации
                        </span>
                        <span
                          title="Доступно после публикации сайта"
                          aria-label="Проверки индексации доступны после публикации"
                          className="hidden items-center justify-center rounded-lg border border-slate-200 bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
                        >
                          <FiActivity />
                        </span>
                        <span
                          title="Опубликуйте сайт, чтобы включить проверки индексации"
                          className="inline-flex items-center gap-1 rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-400"
                        >
                          <FiInfo className="h-3 w-3" /> После публикации
                        </span>
                      </>
                    )}
                    <button
                      onClick={() => onDeleteDomain(domain.id)}
                      disabled={loading}
                      className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    >
                      Удалить
                    </button>
                    <button
                      onClick={() => onDeleteDomain(domain.id)}
                      disabled={loading}
                      className="hidden items-center justify-center rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                      title="Удалить"
                      aria-label="Удалить"
                    >
                      <FiTrash2 />
                    </button>
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-1">
                    <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                      Ключевое слово
                      {keywordDirty && (
                        <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">несохранено</span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <input
                        className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${
                          keywordDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""
                        }`}
                        value={keywordValue}
                        onChange={(e) => onKeywordEditChange(domain.id, e.target.value)}
                        placeholder="Ключевое слово"
                      />
                      <button
                        onClick={() => onUpdateKeyword(domain.id)}
                        disabled={loading || !keywordDirty}
                        className="inline-flex items-center gap-1 rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                      >
                        Сохранить
                      </button>
                      <button
                        onClick={() => onUpdateKeyword(domain.id)}
                        disabled={loading || !keywordDirty}
                        className="hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                        title="Сохранить ключевое слово"
                        aria-label="Сохранить ключевое слово"
                      >
                        <FiCheck />
                      </button>
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                      Ссылка
                      {linkDirty && (
                        <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">несохранено</span>
                      )}
                    </div>
                    <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                      <input
                        className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${
                          linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""
                        }`}
                        value={anchorValue}
                        onChange={(e) => onLinkEditChange(domain.id, { anchor: e.target.value, acceptor: acceptorValue })}
                        placeholder="Анкор"
                      />
                      <input
                        className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${
                          linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""
                        }`}
                        value={acceptorValue}
                        onChange={(e) => onLinkEditChange(domain.id, { anchor: anchorValue, acceptor: e.target.value })}
                        placeholder="https://acceptor.example"
                      />
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => onUpdateLinkSettings(domain.id)}
                          disabled={loading || !linkDirty}
                          className="inline-flex items-center gap-1 rounded-lg bg-slate-200 px-3 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                        >
                          Сохранить
                        </button>
                        <button
                          onClick={() => onUpdateLinkSettings(domain.id)}
                          disabled={loading || !linkDirty}
                          className="hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                          title="Сохранить ссылку"
                          aria-label="Сохранить ссылку"
                        >
                          <FiCheck />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                {openRuns[domain.id] && gens[domain.id] && renderRunsList(gens[domain.id])}
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
