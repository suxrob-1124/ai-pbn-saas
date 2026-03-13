import { useState } from 'react';
import Link from 'next/link';
import type { UrlObject } from 'url';
import {
  CheckCircle2,
  Code,
  Download,
  ExternalLink,
  Edit3,
  ArrowRight,
  Server,
  FileBox,
  Archive,
  Check,
  RefreshCw,
} from 'lucide-react';
import { Badge } from '../../../components/Badge';
import { apiBase, authFetch } from '../../../lib/http';
import { showToast } from '../../../lib/toastStore';

type DomainLike = { id: string; url: string; status: string };
type GenerationLike = { id: string };
type DeploymentAttempt = {
  mode: string;
  target_path: string;
  owner_before?: string;
  owner_after?: string;
  status: string;
  error_message?: string;
  file_count: number;
  total_size_bytes: number;
};
type DomainFileListItem = { path: string };

type DomainResultSectionProps = {
  showResultBlock: boolean;
  domainId: string;
  domain: DomainLike | null;
  latestAttempt: GenerationLike | null;
  latestSuccess: GenerationLike | null;
  hasArtifacts: boolean;
  resultSourceLabel: string;
  legacyDecodedAt?: string;
  zipArchive: string;
  finalHTML: string;
  canOpenEditor: boolean;
  editorHref: UrlObject;
  deployments: DeploymentAttempt[];
  renderStatusBadge: (status: string) => React.ReactNode;
  labels: {
    title: string;
    htmlAction: string;
    zipAction: string;
    artifactsAction: string;
    editorAction: string;
    editorDisabledHint: string;
    backfillHint: string;
  };
};

export function DomainResultSection({
  showResultBlock,
  domainId,
  domain,
  latestAttempt,
  latestSuccess,
  hasArtifacts,
  resultSourceLabel,
  legacyDecodedAt,
  zipArchive,
  finalHTML,
  canOpenEditor,
  editorHref,
  deployments,
  renderStatusBadge,
  labels,
}: DomainResultSectionProps) {
  const [showResultHTMLModal, setShowResultHTMLModal] = useState(false);
  const [resultHTMLTab, setResultHTMLTab] = useState<'preview' | 'code'>('preview');
  const [liveResultHTML, setLiveResultHTML] = useState('');
  const [liveResultCode, setLiveResultCode] = useState('');
  const [liveResultLoading, setLiveResultLoading] = useState(false);
  const [liveResultError, setLiveResultError] = useState<string | null>(null);

  const resultPreviewStyle = { height: '82vh', minHeight: '680px' } as const;

  if (!showResultBlock) return null;

  const downloadZipArchive = () => {
    if (!zipArchive) return;
    try {
      const binary = atob(zipArchive);
      const bytes = new Uint8Array(binary.length);
      for (let idx = 0; idx < binary.length; idx += 1) bytes[idx] = binary.charCodeAt(idx);
      const blob = new Blob([bytes], { type: 'application/zip' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = domain?.url ? `${domain.url}.zip` : 'site.zip';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast({ type: 'error', title: 'Ошибка', message: 'Не удалось декодировать zip_archive' });
    }
  };

  const openLiveResultPreview = async () => {
    setResultHTMLTab('preview');
    setShowResultHTMLModal(true);
    setLiveResultLoading(true);
    setLiveResultError(null);
    try {
      const fileListResp = await authFetch<DomainFileListItem[]>(
        `/api/domains/${domainId}/files`,
      ).catch(() => []);
      const existingPaths = new Set(
        (Array.isArray(fileListResp) ? fileListResp : [])
          .map((i) => (i?.path || '').trim().replace(/^\/+/, '').toLowerCase())
          .filter(Boolean),
      );
      const indexResp = await authFetch<{ content: string }>(
        `/api/domains/${domainId}/files/index.html`,
      );
      const indexHtml = indexResp?.content || '';
      const styleResp = existingPaths.has('style.css')
        ? await authFetch<{ content: string }>(`/api/domains/${domainId}/files/style.css`).catch(
            () => null,
          )
        : null;
      const scriptResp = existingPaths.has('script.js')
        ? await authFetch<{ content: string }>(`/api/domains/${domainId}/files/script.js`).catch(
            () => null,
          )
        : null;
      const styleContent = styleResp?.content
        ? rewriteCssUrls(styleResp.content, domainId, existingPaths)
        : '';
      const scriptContent = scriptResp?.content || '';
      const htmlWithAssets = injectRuntimeAssets(indexHtml, styleContent, scriptContent);
      const livePreview = rewriteHtmlAssetRefs(htmlWithAssets, domainId, existingPaths);
      setLiveResultCode(indexHtml);
      setLiveResultHTML(livePreview);
    } catch {
      if (finalHTML) {
        setLiveResultCode(finalHTML);
        setLiveResultHTML(finalHTML);
        setLiveResultError('Живые файлы недоступны, показан архивный HTML.');
      } else {
        setLiveResultCode('');
        setLiveResultHTML('');
        setLiveResultError('Не удалось загрузить live preview.');
      }
    } finally {
      setLiveResultLoading(false);
    }
  };

  // Базовые стили для вторичных кнопок
  const btnSecondary =
    'inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700/60 dark:bg-[#060d18] dark:text-slate-200 dark:hover:bg-[#0a1020] transition-all shadow-sm active:scale-95 disabled:opacity-50';

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in flex flex-col h-full">
      <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
        <h3 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
          <Archive className="w-4 h-4 text-indigo-500" /> {labels.title}
        </h3>
        <Badge
          label={resultSourceLabel}
          tone={legacyDecodedAt ? 'sky' : 'emerald'}
          icon={<CheckCircle2 className="w-3 h-3" />}
          className="text-[10px]"
        />
      </div>

      <div className="p-6 space-y-6 flex-1 flex flex-col justify-center">
        {/* ГЛАВНЫЕ ДЕЙСТВИЯ (HTML / ZIP) */}
        <div className="grid sm:grid-cols-2 gap-4">
          <button onClick={openLiveResultPreview} disabled={!domain?.id} className={btnSecondary}>
            <Code className="w-4 h-4 text-indigo-500" /> {labels.htmlAction}
          </button>
          <button onClick={downloadZipArchive} disabled={!zipArchive} className={btnSecondary}>
            <Download className="w-4 h-4 text-indigo-500" /> {labels.zipAction}
          </button>
        </div>

        {/* ДОПОЛНИТЕЛЬНЫЕ ССЫЛКИ */}
        <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-3 pt-2">
          {latestAttempt && (
            <Link
              href={`/queue/${latestAttempt.id}`}
              className="text-xs font-medium text-slate-500 hover:text-indigo-600 dark:text-slate-400 dark:hover:text-indigo-400 flex items-center gap-1 transition-colors">
              Последняя попытка <ArrowRight className="w-3 h-3" />
            </Link>
          )}
          {latestSuccess && latestSuccess.id !== latestAttempt?.id && (
            <Link
              href={`/queue/${latestSuccess.id}`}
              className="text-xs font-medium text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 flex items-center gap-1 transition-colors">
              Последний успех <ArrowRight className="w-3 h-3" />
            </Link>
          )}
          {domain?.url && (
            <a
              href={`https://${domain.url}`}
              target="_blank"
              rel="noreferrer"
              className="text-xs font-bold text-slate-900 hover:text-indigo-600 dark:text-white dark:hover:text-indigo-400 flex items-center gap-1 transition-colors">
              <ExternalLink className="w-3.5 h-3.5" /> Открыть сайт
            </a>
          )}
        </div>

        {!hasArtifacts && domain?.status === 'published' && (
          <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-700 dark:border-amber-900/50 dark:bg-amber-500/10 dark:text-amber-400 mt-2">
            <span className="font-bold">Артефакты не заполнены.</span> Запустите legacy import:{' '}
            <code className="bg-amber-100 dark:bg-amber-900 px-1 py-0.5 rounded font-mono ml-1">
              {labels.backfillHint}
            </code>
          </div>
        )}

        {/* БЛОК ДЕПЛОЯ */}
        {deployments[0] && (
          <div className="mt-auto pt-6 border-t border-slate-100 dark:border-slate-800/60">
            <div className="flex items-center gap-2 mb-3">
              <Server className="w-4 h-4 text-slate-400" />
              <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
                Сводка деплоя
              </h4>
            </div>

            <div className="grid grid-cols-2 gap-4 bg-slate-50 dark:bg-[#0a1020] rounded-xl p-4 border border-slate-100 dark:border-slate-800/60">
              <div>
                <div className="text-[10px] text-slate-500 uppercase tracking-widest mb-1">
                  Режим
                </div>
                <div className="font-mono text-xs font-medium dark:text-slate-300">
                  {deployments[0].mode}
                </div>
              </div>
              <div>
                <div className="text-[10px] text-slate-500 uppercase tracking-widest mb-1">
                  Статус
                </div>
                <div>{renderStatusBadge(deployments[0].status)}</div>
              </div>
              <div className="col-span-2">
                <div className="text-[10px] text-slate-500 uppercase tracking-widest mb-1">
                  Целевой путь
                </div>
                <div
                  className="font-mono text-xs text-slate-700 dark:text-slate-300 truncate"
                  title={deployments[0].target_path || '—'}>
                  {deployments[0].target_path || '—'}
                </div>
              </div>
              <div className="col-span-2 flex items-center justify-between pt-2 border-t border-slate-200 dark:border-slate-700/50">
                <div className="flex items-center gap-1.5 text-xs text-slate-500">
                  <FileBox className="w-3.5 h-3.5" /> {deployments[0].file_count} файлов
                </div>
                <div className="text-xs font-medium text-slate-600 dark:text-slate-400">
                  {formatBytes(deployments[0].total_size_bytes)}
                </div>
              </div>
            </div>
            {deployments[0].error_message && (
              <div className="mt-3 text-xs text-red-500 dark:text-red-400 bg-red-50 dark:bg-red-500/10 p-3 rounded-lg border border-red-100 dark:border-red-900/30">
                {deployments[0].error_message}
              </div>
            )}
          </div>
        )}
      </div>

      {/* МОДАЛКА PREVIEW */}
      {showResultHTMLModal && (
        <div className="fixed inset-0 z-[100] bg-slate-900/80 backdrop-blur-sm p-4 md:p-8 flex flex-col animate-in fade-in">
          <div className="mx-auto w-full max-w-7xl flex-1 flex flex-col bg-white dark:bg-[#0f1117] rounded-2xl shadow-2xl overflow-hidden border border-slate-200 dark:border-slate-800 animate-in zoom-in-95 duration-200">
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-[#0a1020]">
              <div className="flex items-center gap-4">
                <h4 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
                  <Code className="w-5 h-5 text-indigo-500" /> HTML Preview
                </h4>

                {/* Переключатель вкладок */}
                <div className="flex bg-slate-200/50 dark:bg-slate-800/80 p-1 rounded-lg">
                  <button
                    onClick={() => setResultHTMLTab('preview')}
                    className={`px-4 py-1.5 text-xs font-bold rounded-md transition-all ${resultHTMLTab === 'preview' ? 'bg-white dark:bg-[#1a2235] text-indigo-600 dark:text-indigo-400 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-white'}`}>
                    Visual Preview
                  </button>
                  <button
                    onClick={() => setResultHTMLTab('code')}
                    className={`px-4 py-1.5 text-xs font-bold rounded-md transition-all ${resultHTMLTab === 'code' ? 'bg-white dark:bg-[#1a2235] text-indigo-600 dark:text-indigo-400 shadow-sm' : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-white'}`}>
                    Source Code
                  </button>
                </div>
              </div>

              <button
                onClick={() => setShowResultHTMLModal(false)}
                className="p-2 bg-slate-200 dark:bg-slate-800 text-slate-500 hover:text-slate-900 dark:hover:text-white rounded-full transition-colors">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex-1 overflow-hidden relative bg-slate-100 dark:bg-[#060d18] p-4">
              {liveResultLoading ? (
                <div className="absolute inset-0 flex flex-col items-center justify-center text-slate-500">
                  <RefreshCw className="w-8 h-8 animate-spin mb-3 text-indigo-500" />
                  <p className="font-medium">Сборка живого preview...</p>
                </div>
              ) : (
                <>
                  {liveResultError && (
                    <div className="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm font-medium text-amber-700 dark:border-amber-900/50 dark:bg-amber-500/10 dark:text-amber-400 flex items-center gap-2">
                      <AlertCircle className="w-5 h-5" /> {liveResultError}
                    </div>
                  )}
                  {resultHTMLTab === 'preview' ? (
                    <iframe
                      title="Final HTML Preview"
                      sandbox="allow-same-origin"
                      srcDoc={liveResultHTML}
                      className="w-full h-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white shadow-inner"
                    />
                  ) : (
                    <pre className="w-full h-full overflow-auto rounded-xl border border-slate-200 bg-slate-50 p-6 text-[13px] font-mono text-slate-700 dark:border-slate-700 dark:bg-[#0a1020] dark:text-slate-300 shadow-inner">
                      {liveResultCode || '(файл index.html отсутствует)'}
                    </pre>
                  )}
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// --- УТИЛИТЫ ---
function rewriteHtmlAssetRefs(html: string, domainId: string, existingPaths?: Set<string>): string {
  const base = apiBase();
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = rawValue.trim();
    if (!value || value.startsWith('#')) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(value)) return full;
    const normalized = value.replace(/^\.\//, '').replace(/^\//, '');
    if (!normalized) return full;
    const [pathPart, hashPart = ''] = normalized.split('#');
    const [purePath, queryPart = ''] = pathPart.split('?');
    if (!purePath) return full;
    const purePathLower = purePath.toLowerCase();
    if (existingPaths && !existingPaths.has(purePathLower)) return full;
    const encodedPath = purePath
      .split('/')
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join('/');
    if (!encodedPath) return full;
    const query = queryPart ? `&${queryPart}` : '';
    const hash = hashPart ? `#${hashPart}` : '';
    const url = `${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}`;
    return `${attr}="${url}"`;
  });
}

function rewriteCssUrls(css: string, domainId: string, existingPaths?: Set<string>): string {
  const base = apiBase();
  return css.replace(/url\(([^)]+)\)/gi, (_full, rawValue: string) => {
    const value = rawValue.trim().replace(/^['"]|['"]$/g, '');
    if (!value || value.startsWith('#')) return `url(${rawValue})`;
    if (/^(data:|https?:|blob:)/i.test(value)) return `url(${rawValue})`;
    const normalized = value.replace(/^\.\//, '').replace(/^\//, '');
    const [pathPart, hashPart = ''] = normalized.split('#');
    const [purePath, queryPart = ''] = pathPart.split('?');
    if (!purePath) return `url(${rawValue})`;
    const purePathLower = purePath.toLowerCase();
    if (existingPaths && !existingPaths.has(purePathLower)) return `url(${rawValue})`;
    const encodedPath = purePath
      .split('/')
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join('/');
    if (!encodedPath) return `url(${rawValue})`;
    const query = queryPart ? `&${queryPart}` : '';
    const hash = hashPart ? `#${hashPart}` : '';
    return `url("${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}")`;
  });
}

function injectRuntimeAssets(
  indexHtml: string,
  styleContent: string,
  scriptContent: string,
): string {
  let html = indexHtml || '';
  if (styleContent) {
    html = html.replace(/<link[^>]*href=["']style\.css["'][^>]*>/gi, '');
    if (/<\/head>/i.test(html)) {
      html = html.replace(
        /<\/head>/i,
        `<style data-live-preview="style.css">\n${styleContent}\n</style>\n</head>`,
      );
    } else {
      html = `<style data-live-preview="style.css">\n${styleContent}\n</style>\n${html}`;
    }
  }
  if (scriptContent) {
    html = html.replace(/<script[^>]*src=["']script\.js["'][^>]*>\s*<\/script>/gi, '');
    if (/<\/body>/i.test(html)) {
      html = html.replace(
        /<\/body>/i,
        `<script data-live-preview="script.js">\n${scriptContent}\n</script>\n</body>`,
      );
    } else {
      html = `${html}\n<script data-live-preview="script.js">\n${scriptContent}\n</script>`;
    }
  }
  return html;
}

function formatBytes(value?: number): string {
  if (!value || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = value;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

import { AlertCircle, X } from 'lucide-react';
