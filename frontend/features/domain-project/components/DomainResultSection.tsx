import { useState } from "react";
import Link from "next/link";
import type { UrlObject } from "url";
import { FiCheck, FiCode, FiDownload, FiEdit3 } from "react-icons/fi";
import { Badge } from "../../../components/Badge";
import { apiBase, authFetch } from "../../../lib/http";
import { showToast } from "../../../lib/toastStore";

type DomainLike = {
  id: string;
  url: string;
  status: string;
};

type GenerationLike = {
  id: string;
};

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

type DomainFileListItem = {
  path: string;
};

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
  labels
}: DomainResultSectionProps) {
  const [showResultHTMLModal, setShowResultHTMLModal] = useState(false);
  const [resultHTMLTab, setResultHTMLTab] = useState<"preview" | "code">("preview");
  const [liveResultHTML, setLiveResultHTML] = useState("");
  const [liveResultCode, setLiveResultCode] = useState("");
  const [liveResultLoading, setLiveResultLoading] = useState(false);
  const [liveResultError, setLiveResultError] = useState<string | null>(null);
  const resultPreviewStyle = { height: "82vh", minHeight: "680px" } as const;

  if (!showResultBlock) {
    return null;
  }

  const downloadZipArchive = () => {
    if (!zipArchive) return;
    try {
      const binary = atob(zipArchive);
      const bytes = new Uint8Array(binary.length);
      for (let idx = 0; idx < binary.length; idx += 1) {
        bytes[idx] = binary.charCodeAt(idx);
      }
      const blob = new Blob([bytes], { type: "application/zip" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      const fileName = domain?.url ? `${domain.url}.zip` : "site.zip";
      a.download = fileName;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      showToast({
        type: "error",
        title: "Ошибка скачивания ZIP",
        message: "Не удалось декодировать zip_archive"
      });
    }
  };

  const openLiveResultPreview = async () => {
    setResultHTMLTab("preview");
    setShowResultHTMLModal(true);
    setLiveResultLoading(true);
    setLiveResultError(null);
    try {
      const fileListResp = await authFetch<DomainFileListItem[]>(`/api/domains/${domainId}/files`).catch(() => []);
      const existingPaths = new Set(
        (Array.isArray(fileListResp) ? fileListResp : [])
          .map((item) => (item?.path || "").trim().replace(/^\/+/, "").toLowerCase())
          .filter(Boolean)
      );
      const indexResp = await authFetch<{ content: string }>(`/api/domains/${domainId}/files/index.html`);
      const indexHtml = indexResp?.content || "";
      const styleResp = existingPaths.has("style.css")
        ? await authFetch<{ content: string }>(`/api/domains/${domainId}/files/style.css`).catch(() => null)
        : null;
      const scriptResp = existingPaths.has("script.js")
        ? await authFetch<{ content: string }>(`/api/domains/${domainId}/files/script.js`).catch(() => null)
        : null;
      const styleContent = styleResp?.content ? rewriteCssUrls(styleResp.content, domainId, existingPaths) : "";
      const scriptContent = scriptResp?.content || "";
      const htmlWithAssets = injectRuntimeAssets(indexHtml, styleContent, scriptContent);
      const livePreview = rewriteHtmlAssetRefs(htmlWithAssets, domainId, existingPaths);
      setLiveResultCode(indexHtml);
      setLiveResultHTML(livePreview);
    } catch {
      if (finalHTML) {
        setLiveResultCode(finalHTML);
        setLiveResultHTML(finalHTML);
        setLiveResultError("Показан артефактный HTML: живые файлы пока недоступны.");
      } else {
        setLiveResultCode("");
        setLiveResultHTML("");
        setLiveResultError("Не удалось загрузить live preview из файлов домена.");
      }
    } finally {
      setLiveResultLoading(false);
    }
  };

  return (
    <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h3 className="font-semibold">{labels.title}</h3>
            <div className="mt-1 flex items-center gap-2">
              <Badge
                label={resultSourceLabel}
                tone={legacyDecodedAt ? "sky" : "emerald"}
                icon={<FiCheck />}
                className="text-xs"
              />
              {legacyDecodedAt && (
                <span className="text-xs text-slate-500 dark:text-slate-400">
                  Декодировано: {new Date(legacyDecodedAt).toLocaleString()}
                </span>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => {
                void openLiveResultPreview();
              }}
              disabled={!domain?.id}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiCode /> {labels.htmlAction}
            </button>
            <button
              type="button"
              onClick={downloadZipArchive}
              disabled={!zipArchive}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiDownload /> {labels.zipAction}
            </button>
            {hasArtifacts && (
              <a
                href="#domain-artifacts"
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                  {labels.artifactsAction}
              </a>
            )}
            {latestAttempt && (
              <Link
                href={`/queue/${latestAttempt.id}`}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Последняя попытка
              </Link>
            )}
            {latestSuccess && latestSuccess.id !== latestAttempt?.id && (
              <Link
                href={`/queue/${latestSuccess.id}`}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Последний успех
              </Link>
            )}
            {domain?.url && (
              <a
                href={`https://${domain.url}`}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Открыть по домену
              </a>
            )}
            {canOpenEditor ? (
              <Link
                href={editorHref}
                className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-semibold text-white hover:bg-indigo-500"
              >
                <FiEdit3 /> {labels.editorAction}
              </Link>
            ) : (
              <span
                title={labels.editorDisabledHint}
                className="inline-flex items-center gap-2 rounded-lg bg-slate-300 px-3 py-2 text-xs font-semibold text-slate-600"
              >
                <FiEdit3 /> {labels.editorAction}
              </span>
            )}
          </div>
        </div>

        {!hasArtifacts && domain?.status === "published" && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
            Артефакты еще не заполнены. Запустите legacy import: <code>{labels.backfillHint} ...</code>
          </div>
        )}
      </div>

      {deployments[0] && (
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-2">
          <h3 className="font-semibold">Последний деплой</h3>
          <div className="text-sm text-slate-600 dark:text-slate-300">
            Статус: {renderStatusBadge(deployments[0].status)} · Режим: {deployments[0].mode}
          </div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            Путь: {deployments[0].target_path || "—"} · Owner: {deployments[0].owner_after || deployments[0].owner_before || "—"}
          </div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            Файлов: {deployments[0].file_count} · Размер: {formatBytes(deployments[0].total_size_bytes)}
          </div>
          {deployments[0].error_message && (
            <div className="text-xs text-red-500">{deployments[0].error_message}</div>
          )}
        </div>
      )}

      {showResultHTMLModal && (
        <div className="fixed inset-0 z-50 bg-black/60 px-3 py-6 md:px-8 overflow-auto">
          <div className="mx-auto w-[min(98vw,1700px)] rounded-xl border border-slate-200 bg-white p-4 shadow-2xl dark:border-slate-800 dark:bg-slate-950">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h4 className="text-sm font-semibold">Финальный HTML</h4>
              <button
                type="button"
                onClick={() => setShowResultHTMLModal(false)}
                className="rounded-lg border border-slate-200 px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-100 dark:hover:bg-slate-800"
              >
                Закрыть
              </button>
            </div>
            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                onClick={() => setResultHTMLTab("preview")}
                className={`rounded-lg border px-3 py-1 text-xs font-semibold ${
                  resultHTMLTab === "preview"
                    ? "bg-indigo-600 border-indigo-600 text-white"
                    : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"
                }`}
              >
                Preview
              </button>
              <button
                type="button"
                onClick={() => setResultHTMLTab("code")}
                className={`rounded-lg border px-3 py-1 text-xs font-semibold ${
                  resultHTMLTab === "code"
                    ? "bg-indigo-600 border-indigo-600 text-white"
                    : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"
                }`}
              >
                Code
              </button>
            </div>
            {liveResultLoading ? (
              <div
                style={resultPreviewStyle}
                className="mt-3 rounded-lg border border-slate-200 bg-slate-50 p-3 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-300"
              >
                Загружаем живой preview...
              </div>
            ) : (
              <>
                {liveResultError && (
                  <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
                    {liveResultError}
                  </div>
                )}
                {resultHTMLTab === "preview" ? (
                  <iframe
                    title="Final HTML Preview"
                    sandbox="allow-same-origin allow-scripts"
                    srcDoc={liveResultHTML}
                    style={resultPreviewStyle}
                    className="mt-3 w-full rounded-lg border border-slate-200 dark:border-slate-700 bg-white"
                  />
                ) : (
                  <pre
                    style={resultPreviewStyle}
                    className="mt-3 overflow-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs dark:border-slate-700 dark:bg-slate-900/60"
                  >
                    {liveResultCode || "(файл index.html отсутствует)"}
                  </pre>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </>
  );
}

function rewriteHtmlAssetRefs(html: string, domainId: string, existingPaths?: Set<string>): string {
  const base = apiBase();
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = rawValue.trim();
    if (!value || value.startsWith("#")) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(value)) return full;
    const normalized = value.replace(/^\.\//, "").replace(/^\//, "");
    if (!normalized) return full;
    const [pathPart, hashPart = ""] = normalized.split("#");
    const [purePath, queryPart = ""] = pathPart.split("?");
    if (!purePath) return full;
    const purePathLower = purePath.toLowerCase();
    if (existingPaths && !existingPaths.has(purePathLower)) return full;
    const encodedPath = purePath
      .split("/")
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join("/");
    if (!encodedPath) return full;
    const query = queryPart ? `&${queryPart}` : "";
    const hash = hashPart ? `#${hashPart}` : "";
    const url = `${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}`;
    return `${attr}="${url}"`;
  });
}

function rewriteCssUrls(css: string, domainId: string, existingPaths?: Set<string>): string {
  const base = apiBase();
  return css.replace(/url\(([^)]+)\)/gi, (_full, rawValue: string) => {
    const value = rawValue.trim().replace(/^['"]|['"]$/g, "");
    if (!value || value.startsWith("#")) return `url(${rawValue})`;
    if (/^(data:|https?:|blob:)/i.test(value)) return `url(${rawValue})`;
    const normalized = value.replace(/^\.\//, "").replace(/^\//, "");
    const [pathPart, hashPart = ""] = normalized.split("#");
    const [purePath, queryPart = ""] = pathPart.split("?");
    if (!purePath) return `url(${rawValue})`;
    const purePathLower = purePath.toLowerCase();
    if (existingPaths && !existingPaths.has(purePathLower)) return `url(${rawValue})`;
    const encodedPath = purePath
      .split("/")
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join("/");
    if (!encodedPath) return `url(${rawValue})`;
    const query = queryPart ? `&${queryPart}` : "";
    const hash = hashPart ? `#${hashPart}` : "";
    return `url("${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}")`;
  });
}

function injectRuntimeAssets(indexHtml: string, styleContent: string, scriptContent: string): string {
  let html = indexHtml || "";
  if (styleContent) {
    html = html.replace(/<link[^>]*href=["']style\.css["'][^>]*>/gi, "");
    if (/<\/head>/i.test(html)) {
      html = html.replace(/<\/head>/i, `<style data-live-preview="style.css">\n${styleContent}\n</style>\n</head>`);
    } else {
      html = `<style data-live-preview="style.css">\n${styleContent}\n</style>\n${html}`;
    }
  }
  if (scriptContent) {
    html = html.replace(/<script[^>]*src=["']script\.js["'][^>]*>\s*<\/script>/gi, "");
    if (/<\/body>/i.test(html)) {
      html = html.replace(/<\/body>/i, `<script data-live-preview="script.js">\n${scriptContent}\n</script>\n</body>`);
    } else {
      html = `${html}\n<script data-live-preview="script.js">\n${scriptContent}\n</script>`;
    }
  }
  return html;
}

function formatBytes(value?: number): string {
  if (!value || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let idx = 0;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}
