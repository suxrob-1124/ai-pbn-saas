"use client";

import { FiDownload, FiExternalLink } from "react-icons/fi";
import { downloadBase64File } from "../lib/utils";

type Props = {
  artifacts?: Record<string, any>;
};

export function GenerationResultActions({ artifacts }: Props) {
  if (!artifacts) return null;

  const zipData = artifacts["zip_archive"] as string | undefined;
  const finalHtml = artifacts["final_html"] as string | undefined;
  const generatedFiles = artifacts["generated_files"] as any[] | undefined;

  if (!zipData && !finalHtml) return null;

  const handleDownload = () => {
    if (!zipData) return;
    downloadBase64File(zipData, "website.zip", "application/zip");
  };

  const handlePreview = () => {
    if (!finalHtml) return;
    const bundled = inlineAssets(finalHtml, generatedFiles || []);
    const preview = window.open("about:blank", "_blank");
    if (preview) {
      preview.document.open();
      preview.document.write(bundled);
      preview.document.close();
    }
  };

  return (
    <div className="rounded-lg border border-emerald-200 dark:border-emerald-800 bg-emerald-50/70 dark:bg-emerald-900/20 p-4 space-y-3">
      <h3 className="text-sm font-semibold text-emerald-900 dark:text-emerald-100">Готовый сайт</h3>
      <div className="flex flex-wrap gap-3">
        <button
          onClick={handleDownload}
          disabled={!zipData}
          className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 disabled:opacity-60"
        >
          <FiDownload /> Скачать сайт (.zip)
        </button>
        <button
          onClick={handlePreview}
          disabled={!finalHtml}
          className="inline-flex items-center gap-2 rounded-lg border border-emerald-300 bg-white px-4 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-900 dark:text-emerald-200 dark:hover:bg-slate-800 disabled:opacity-60"
        >
          <FiExternalLink /> Открыть предпросмотр
        </button>
      </div>
      {!zipData && <p className="text-xs text-emerald-900/80 dark:text-emerald-200/80">Артефакт zip_archive отсутствует.</p>}
    </div>
  );
}

function inlineAssets(html: string, files: any[]): string {
  if (!files?.length) return html;
  let result = html;

  const decodeText = (f: any) => {
    if (f.content) return f.content as string;
    if (f.content_base64) {
      try {
        return atob(f.content_base64 as string);
      } catch {
        return "";
      }
    }
    return "";
  };

  const escapeReg = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

  files.forEach((f: any) => {
    const path = (f.path || "").trim();
    if (!path) return;
    const lower = path.toLowerCase();

    // CSS: инлайним
    if (lower.endsWith(".css")) {
      const css = decodeText(f);
      if (!css) return;
      const re = new RegExp(`<link[^>]+href=["'][^"']*${escapeReg(path)}[^"']*["'][^>]*>`, "i");
      result = result.replace(re, `<style>${css}</style>`);
      return;
    }

    // JS: инлайним
    if (lower.endsWith(".js")) {
      const js = decodeText(f);
      if (!js) return;
      const re = new RegExp(`<script[^>]+src=["'][^"']*${escapeReg(path)}[^"']*["'][^>]*><\/script>`, "i");
      result = result.replace(re, `<script>${js}</script>`);
      return;
    }

    // Images: заменяем src
    const isImg = [".webp", ".png", ".jpg", ".jpeg", ".svg"].some((ext) => lower.endsWith(ext));
    if (isImg) {
      let dataUrl = "";
      if (f.content_base64) {
        const mime = lower.endsWith(".webp")
          ? "image/webp"
          : lower.endsWith(".png")
          ? "image/png"
          : lower.endsWith(".jpg") || lower.endsWith(".jpeg")
          ? "image/jpeg"
          : lower.endsWith(".svg")
          ? "image/svg+xml"
          : "application/octet-stream";
        dataUrl = `data:${mime};base64,${f.content_base64}`;
      } else if (f.content && lower.endsWith(".svg")) {
        try {
          dataUrl = `data:image/svg+xml;base64,${btoa(unescape(encodeURIComponent(f.content)))}`;
        } catch {
          dataUrl = "";
        }
      }
      if (!dataUrl) return;
      const re = new RegExp(`src=["']([^"']*${escapeReg(path)})["']`, "gi");
      result = result.replace(re, `src="${dataUrl}"`);
      return;
    }
  });

  return result;
}
