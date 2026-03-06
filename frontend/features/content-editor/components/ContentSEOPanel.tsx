"use client";

import type { SEOData } from "../types/content-editor";
import { contentEditorRu } from "../services/i18n-content-ru";

type ContentSEOPanelProps = {
  seo: SEOData;
  onUpdate: <K extends keyof SEOData>(key: K, value: string) => void;
  currentPath: string | undefined;
  readOnly: boolean;
};

const t = contentEditorRu.seo;

export function ContentSEOPanel({
  seo,
  onUpdate,
  currentPath,
  readOnly,
}: ContentSEOPanelProps) {
  return (
    <div className="space-y-3">
      <div>
        <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
          {t.pageTitle}
        </label>
        <input
          type="text"
          value={seo.title}
          onChange={(e) => onUpdate("title", e.target.value)}
          placeholder={t.pageTitlePlaceholder}
          disabled={readOnly}
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
        />
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
          {t.pageDescription}
        </label>
        <textarea
          value={seo.description}
          onChange={(e) => onUpdate("description", e.target.value)}
          placeholder={t.pageDescriptionPlaceholder}
          rows={3}
          disabled={readOnly}
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
        />
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
          {t.slug}
        </label>
        <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-1.5 text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400">
          {currentPath || "—"}
        </div>
      </div>

      <div className="border-t border-slate-100 pt-3 dark:border-slate-800">
        <p className="mb-2 text-xs font-medium text-slate-400 dark:text-slate-500">
          Open Graph
        </p>
        <div className="space-y-2">
          <div>
            <label className="mb-0.5 block text-xs text-slate-400 dark:text-slate-500">
              {t.ogTitle}
            </label>
            <input
              type="text"
              value={seo.ogTitle}
              onChange={(e) => onUpdate("ogTitle", e.target.value)}
              placeholder={seo.title || t.pageTitlePlaceholder}
              disabled={readOnly}
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
          </div>
          <div>
            <label className="mb-0.5 block text-xs text-slate-400 dark:text-slate-500">
              {t.ogDescription}
            </label>
            <input
              type="text"
              value={seo.ogDescription}
              onChange={(e) => onUpdate("ogDescription", e.target.value)}
              placeholder={seo.description || t.pageDescriptionPlaceholder}
              disabled={readOnly}
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
