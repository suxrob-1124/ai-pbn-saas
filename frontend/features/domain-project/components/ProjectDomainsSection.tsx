import Link from 'next/link';
import type { UrlObject } from 'url';
import type { ReactNode } from 'react';
import {
  Link as LinkIcon,
  Trash2,
  List,
  Edit3,
  Activity,
  RefreshCw,
  Check,
  Search,
  ExternalLink,
  Globe,
} from 'lucide-react';
import { Badge } from '../../../components/Badge';
import { DOMAIN_PROJECT_CTA, getLinkActionLabel } from '../services/statusCta';
import { hasInsertedLink, isLinkTaskInProgress } from '../../../lib/linkTaskStatus';
import { getGenerationTypeLabel } from '../services/generationTypes';

type DomainCard = {
  id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  generation_type?: string;
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
  domainSearch: string;
  domainsCount: number;
  filteredDomains: DomainCard[];
  linkLoadingId: string | null;
  openRuns: Record<string, boolean>;
  gens: Record<string, any[]>;
  keywordEdits: Record<string, string>;
  linkEdits: Record<string, LinkEditValue>;
  onDomainSearchChange: (value: string) => void;
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
  url?: string;
  keyword?: string;
  country?: string;
  language?: string;
  exclude?: string;
  importText?: string;
  onUrlChange?: (v: string) => void;
  onKeywordChange?: (v: string) => void;
  onCountryChange?: (v: string) => void;
  onLanguageChange?: (v: string) => void;
  onExcludeChange?: (v: string) => void;
  onImportTextChange?: (v: string) => void;
  onAddDomain?: () => void;
  onImportDomains?: () => void;
};

export function ProjectDomainsSection({
  loading,
  domainSearch,
  domainsCount,
  filteredDomains,
  linkLoadingId,
  openRuns,
  gens,
  keywordEdits,
  linkEdits,
  onDomainSearchChange,
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
  formatRelativeTime,
}: ProjectDomainsSectionProps) {
  // --- Унифицированные стили кнопок, чтобы они не ломались в темной теме ---
  const btnPrimaryClass =
    'inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-50 text-indigo-700 hover:bg-indigo-100 disabled:opacity-50 dark:bg-indigo-500/10 dark:text-indigo-400 dark:hover:bg-indigo-500/20 text-xs font-semibold transition-colors';
  const btnDangerClass =
    'inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-red-50 text-red-700 hover:bg-red-100 disabled:opacity-50 dark:bg-red-500/10 dark:text-red-400 dark:hover:bg-red-500/20 text-xs font-semibold transition-colors';
  const btnSecondaryClass =
    'inline-flex items-center gap-1.5 px-3 py-2 rounded-lg border border-slate-200 bg-white text-slate-700 hover:bg-slate-100 hover:text-slate-900 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 dark:hover:text-white text-xs font-medium transition-colors';

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm animate-in fade-in duration-300 overflow-hidden">
      {/* HEADER ТАБЛИЦЫ */}
      <div className="p-6 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#0a1020] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <Globe className="w-5 h-5 text-indigo-500" /> SEO-настройки и Ссылки
          </h3>
        </div>
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 dark:text-slate-400" />
          <input
            type="text"
            value={domainSearch}
            onChange={(e) => onDomainSearchChange(e.target.value)}
            placeholder={`Поиск из ${domainsCount} доменов...`}
            className="w-full pl-10 pr-4 py-2.5 bg-white border border-slate-300 rounded-xl text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:bg-[#060d18] dark:border-slate-600 dark:text-white dark:placeholder:text-slate-500 transition-all shadow-sm"
          />
        </div>
      </div>

      {filteredDomains.length === 0 && (
        <div className="p-12 text-center text-slate-500 dark:text-slate-400 font-medium">
          Сайты не найдены.
        </div>
      )}

      {/* ПЛОТНЫЙ СПИСОК */}
      <div className="divide-y divide-slate-100 dark:divide-slate-700/60">
        {filteredDomains.map((domain) => {
          const linkStatus = getEffectiveLinkStatus(domain);
          const hasActiveLink = hasInsertedLink(linkStatus);
          const linkInProgress = isLinkTaskInProgress(linkStatus);
          const canRemoveLink = hasActiveLink && !linkInProgress;
          const linkReadyAtDate = domain.link_ready_at ? new Date(domain.link_ready_at) : null;
          const linkReadyValid = linkReadyAtDate && !Number.isNaN(linkReadyAtDate.getTime());
          const linkReadyFuture = Boolean(
            linkReadyValid && linkReadyAtDate!.getTime() > Date.now(),
          );

          const keywordValue = keywordEdits[domain.id] ?? '';
          const keywordDirty = keywordValue.trim() !== (domain.main_keyword || '').trim();
          const link = linkEdits[domain.id] || {
            anchor: domain.link_anchor_text || '',
            acceptor: domain.link_acceptor_url || '',
          };
          const anchorValue = link.anchor ?? '';
          const acceptorValue = link.acceptor ?? '';
          const linkDirty =
            anchorValue.trim() !== (domain.link_anchor_text || '').trim() ||
            acceptorValue.trim() !== (domain.link_acceptor_url || '').trim();

          const inputBaseClass =
            'w-full bg-white dark:bg-[#060d18] border px-3 py-2 text-sm rounded-lg outline-none transition-all dark:text-slate-100 placeholder:text-slate-400 dark:placeholder:text-slate-500';

          return (
            <div
              key={domain.id}
              className="p-6 hover:bg-slate-50/50 dark:hover:bg-white/[0.03] transition-colors">
              {/* ВЕРХ: ИНФО И КНОПКИ */}
              <div className="flex flex-col xl:flex-row xl:items-start justify-between gap-5 mb-5">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-3 mb-2">
                    <Link
                      href={`/domains/${domain.id}`}
                      className="text-base font-bold text-slate-900 dark:text-white hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors flex items-center gap-1.5">
                      {domain.url} <ExternalLink className="w-4 h-4 opacity-50" />
                    </Link>
                    {renderStatusBadge(domain.status)}
                    {domain.generation_type && domain.generation_type !== 'single_page' && (
                      <Badge
                        label={getGenerationTypeLabel(domain.generation_type)}
                        tone="blue"
                        className="text-[11px] hidden sm:inline-flex px-2 py-0.5"
                      />
                    )}
                    {renderLinkStatusBadge(domain)}
                    {domain.last_success_generation_id && (
                      <Badge
                        label="Сгенерирован"
                        tone="green"
                        className="text-[11px] hidden sm:inline-flex px-2 py-0.5"
                      />
                    )}
                  </div>
                  <div className="text-sm text-slate-500 dark:text-slate-400 flex flex-wrap items-center gap-3 font-medium">
                    <span>
                      {domain.target_country || 'Мир'} / {domain.target_language || 'Авто'}
                    </span>
                    <span className="opacity-50">•</span>
                    <span>Обновлен: {formatDateTime(domain.updated_at)}</span>
                  </div>
                </div>

                {/* КНОПКИ ДЕЙСТВИЙ */}
                <div className="flex flex-wrap items-center gap-2 flex-shrink-0 mt-2 xl:mt-0">
                  <button
                    onClick={() => onRunLinkTask(domain.id)}
                    disabled={loading || linkLoadingId === domain.id || linkInProgress}
                    className={btnPrimaryClass}>
                    {linkInProgress ? (
                      <RefreshCw className="w-3.5 h-3.5 flex-shrink-0 animate-spin" />
                    ) : (
                      <LinkIcon className="w-3.5 h-3.5 flex-shrink-0" />
                    )}
                    {getLinkActionLabel(hasActiveLink, linkInProgress, true)}
                  </button>
                  {canRemoveLink && (
                    <button
                      onClick={() => onRemoveLinkTask(domain.id)}
                      disabled={loading || linkLoadingId === domain.id}
                      className={btnDangerClass}>
                      <Trash2 className="w-3.5 h-3.5 flex-shrink-0" /> Удалить ссылку
                    </button>
                  )}
                  <button onClick={() => onLoadRuns(domain.id)} className={btnSecondaryClass}>
                    <List className="w-3.5 h-3.5 flex-shrink-0" /> Логи{' '}
                    {openRuns[domain.id] && gens[domain.id] && `(${gens[domain.id].length})`}
                  </button>
                  <Link
                    href={{ pathname: `/domains/${domain.id}/editor` } as UrlObject}
                    className={btnSecondaryClass}>
                    <Edit3 className="w-3.5 h-3.5 flex-shrink-0" /> Редактор
                  </Link>
                  <Link
                    href={
                      {
                        pathname: '/monitoring/indexing',
                        query: { domainId: domain.id },
                      } as UrlObject
                    }
                    className={btnSecondaryClass}>
                    <Activity className="w-3.5 h-3.5 flex-shrink-0" /> Индекс
                  </Link>
                  <button
                    onClick={() => onDeleteDomain(domain.id)}
                    disabled={loading}
                    className="p-2 rounded-lg text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
                    title="Удалить домен">
                    <Trash2 className="w-4 h-4 flex-shrink-0" />
                  </button>
                </div>
              </div>

              {/* ИНПУТЫ */}
              <div className="grid lg:grid-cols-[1fr_1.5fr] gap-6 bg-slate-50 dark:bg-[#0a1020] p-4 rounded-xl border border-slate-200 dark:border-slate-700">
                {/* KeyWord */}
                <div className="flex flex-col gap-2">
                  <label className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-300">
                    Main Keyword
                  </label>
                  <div className="relative flex items-center">
                    <input
                      className={`${inputBaseClass} ${keywordDirty ? 'border-amber-400 pr-28 ring-1 ring-amber-400/30' : 'border-slate-200 dark:border-slate-600 focus:border-indigo-500 dark:focus:border-indigo-400'}`}
                      value={keywordValue}
                      onChange={(e) => onKeywordEditChange(domain.id, e.target.value)}
                      placeholder="Например: auto insurance"
                    />
                    {keywordDirty && (
                      <button
                        onClick={() => onUpdateKeyword(domain.id)}
                        disabled={loading}
                        className="absolute right-1.5 top-1.5 bottom-1.5 px-3 bg-amber-500 text-white text-xs font-bold rounded-md hover:bg-amber-600 transition-colors flex items-center gap-1.5 shadow-sm">
                        <Check className="w-3.5 h-3.5 flex-shrink-0" /> Сохранить
                      </button>
                    )}
                  </div>
                </div>

                {/* Link Task */}
                <div className="flex flex-col gap-2">
                  <label className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-300 flex items-center justify-between">
                    <span>Link Task (Anchor & URL)</span>
                    {linkReadyValid && (
                      <span
                        className={`normal-case text-[11px] font-semibold ${linkReadyFuture ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'}`}>
                        {linkReadyFuture
                          ? `Ожидает ${formatRelativeTime(linkReadyAtDate!)}`
                          : 'Готово к вставке'}
                      </span>
                    )}
                  </label>
                  <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 relative">
                    <input
                      className={`sm:w-1/3 ${inputBaseClass} ${linkDirty ? 'border-amber-400 ring-1 ring-amber-400/30' : 'border-slate-200 dark:border-slate-600 focus:border-indigo-500 dark:focus:border-indigo-400'}`}
                      value={anchorValue}
                      onChange={(e) =>
                        onLinkEditChange(domain.id, {
                          anchor: e.target.value,
                          acceptor: acceptorValue,
                        })
                      }
                      placeholder="Текст анкора"
                    />
                    <div className="relative flex items-center sm:w-2/3">
                      <input
                        className={`w-full ${inputBaseClass} ${linkDirty ? 'border-amber-400 pr-28 ring-1 ring-amber-400/30' : 'border-slate-200 dark:border-slate-600 focus:border-indigo-500 dark:focus:border-indigo-400'}`}
                        value={acceptorValue}
                        onChange={(e) =>
                          onLinkEditChange(domain.id, {
                            anchor: anchorValue,
                            acceptor: e.target.value,
                          })
                        }
                        placeholder="https://target-site.com"
                      />
                      {linkDirty && (
                        <button
                          onClick={() => onUpdateLinkSettings(domain.id)}
                          disabled={loading}
                          className="absolute right-1.5 top-1.5 bottom-1.5 px-3 bg-amber-500 text-white text-xs font-bold rounded-md hover:bg-amber-600 transition-colors flex items-center gap-1 shadow-sm">
                          <Check className="w-3 h-3" /> Сохранить
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              </div>

              {openRuns[domain.id] && gens[domain.id] && (
                <div className="mt-5 p-5 rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-[#0a1020] shadow-inner">
                  <h4 className="text-xs font-bold uppercase text-slate-500 mb-3 tracking-wider">
                    История запусков
                  </h4>
                  {renderRunsList(gens[domain.id])}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
