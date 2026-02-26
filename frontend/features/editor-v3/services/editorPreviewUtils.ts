import { apiBase } from "../../../lib/http";
import { editorV3Ru } from "./i18n-ru";
import type { AIAssetGenerationResultDTO, AIFlowStatus } from "../types/ai";

export const detectLanguage = (pathValue: string) => {
  const path = pathValue.toLowerCase();
  if (path.endsWith(".html") || path.endsWith(".htm")) return "html";
  if (path.endsWith(".css")) return "css";
  if (path.endsWith(".js") || path.endsWith(".mjs") || path.endsWith(".cjs")) return "javascript";
  if (path.endsWith(".ts")) return "typescript";
  if (path.endsWith(".tsx")) return "typescript";
  if (path.endsWith(".json")) return "json";
  if (path.endsWith(".xml") || path.endsWith(".svg")) return "xml";
  if (path.endsWith(".md") || path.endsWith(".markdown")) return "markdown";
  return "plaintext";
};

export const looksBinary = (value: string) => value.includes("\u0000");

const IMAGE_FILE_EXT_RE = /\.(png|jpe?g|gif|webp|svg|avif|bmp|ico)$/i;

export const isImageLikeFile = (pathValue: string, mimeType?: string | null) => {
  const mime = (mimeType || "").toLowerCase();
  if (mime.startsWith("image/")) return true;
  return IMAGE_FILE_EXT_RE.test(pathValue.toLowerCase());
};

export const normalizeRelativeSitePath = (baseFilePath: string, refValue: string) => {
  const trimmed = refValue.trim();
  if (!trimmed || trimmed.startsWith("#")) return "";
  if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(trimmed)) return "";
  const pathPart = trimmed.split("#")[0].split("?")[0].trim();
  if (!pathPart) return "";
  const rawSegments = (pathPart.startsWith("/")
    ? pathPart.replace(/^\/+/, "").split("/")
    : [...baseFilePath.split("/").filter(Boolean).slice(0, -1), ...pathPart.split("/")]).filter(Boolean);
  const normalized: string[] = [];
  for (const segment of rawSegments) {
    if (segment === ".") continue;
    if (segment === "..") {
      normalized.pop();
      continue;
    }
    normalized.push(segment);
  }
  return normalized.join("/");
};

export const extractLocalAssetRefsFromHTML = (html: string) => {
  const isLikelyAssetHref = (value: string) => {
    const clean = value.split("#")[0].split("?")[0].trim().toLowerCase();
    if (!clean || clean === "/" || clean.endsWith("/")) return false;
    return /\.(css|js|mjs|cjs|png|jpe?g|gif|webp|svg|ico|bmp|avif|webm|mp4|mp3|wav|ogg|woff2?|ttf|otf|eot|json|xml|txt|pdf|webmanifest)$/i.test(
      clean
    );
  };
  const refs = new Set<string>();
  const re = /\b(src|href)\s*=\s*["']([^"']+)["']/gi;
  let match: RegExpExecArray | null;
  while ((match = re.exec(html)) !== null) {
    const attr = (match[1] || "").trim().toLowerCase();
    const value = (match[2] || "").trim();
    if (!value) continue;
    if (/^(data:|mailto:|tel:|javascript:|https?:|#)/i.test(value)) continue;
    if (attr === "href" && !isLikelyAssetHref(value)) continue;
    refs.add(value);
  }
  return Array.from(refs);
};

export const normalizeGeneratedHtmlResourcePaths = (filePath: string, html: string) => {
  if (!html || !filePath.toLowerCase().endsWith(".html")) return html;
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = (rawValue || "").trim();
    if (!value) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:|#)/i.test(value)) return full;
    if (!value.startsWith("./") && !value.startsWith("../")) return full;
    const [pathAndQuery, hashPart = ""] = value.split("#");
    const [pathPart = "", queryPart = ""] = pathAndQuery.split("?");
    const normalized = normalizeRelativeSitePath(filePath, pathPart);
    if (!normalized) return full;
    const querySuffix = queryPart ? `?${queryPart}` : "";
    const hashSuffix = hashPart ? `#${hashPart}` : "";
    const nextValue = `/${normalized}${querySuffix}${hashSuffix}`;
    return `${attr}="${nextValue}"`;
  });
};

export const encodePath = (value: string) =>
  value
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");

export function rewriteHtmlAssetRefs(html: string, domainId: string) {
  const base = apiBase();
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = rawValue.trim();
    if (!value || value.startsWith("#")) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(value)) return full;
    const noDotPrefix = value.replace(/^\.\//, "");
    const [pathAndQuery, hashPart = ""] = noDotPrefix.split("#");
    const [pathPartRaw, queryPart = ""] = pathAndQuery.split("?");
    const pathPart = pathPartRaw.trim();
    const normalized = pathPart.replace(/^\/+/, "");
    const effectivePath = normalized || "index.html";
    const encodedPath = effectivePath
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

export function injectRuntimeAssets(indexHtml: string, styleContent: string, scriptContent: string) {
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

export const AI_FLOW_STATUS_LABELS: Record<AIFlowStatus, string> = editorV3Ru.flowStatusLabels;

export const getFlowToneClass = (status: AIFlowStatus) => {
  if (status === "error") return "border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300";
  if (status === "ready" || status === "done")
    return "border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300";
  if (status === "idle")
    return "border-slate-300 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800/40 dark:text-slate-300";
  return "border-indigo-300 bg-indigo-50 text-indigo-700 dark:border-indigo-900 dark:bg-indigo-950/30 dark:text-indigo-300";
};

export const assetStatusLabel = (status: AIAssetGenerationResultDTO["status"]) => {
  if (status === "ok") return editorV3Ru.imagePanel.statusOk;
  if (status === "broken" || status === "missing") return editorV3Ru.imagePanel.statusAttention;
  return editorV3Ru.imagePanel.statusError;
};

export const assetStatusClass = (status: AIAssetGenerationResultDTO["status"]) => {
  if (status === "ok") return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300";
  if (status === "broken" || status === "missing") return "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300";
  return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300";
};
