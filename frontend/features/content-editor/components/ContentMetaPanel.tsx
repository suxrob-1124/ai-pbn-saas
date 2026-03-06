"use client";

import { useRef } from "react";
import { FiUpload, FiPlus, FiTrash2 } from "react-icons/fi";
import { uploadFile } from "@/lib/fileApi";
import { apiBase } from "@/lib/http";
import { showToast } from "@/lib/toastStore";
import type { PageMeta, NavLink, NavSection } from "../services/htmlMetaExtraction";
import { contentEditorRu } from "../services/i18n-content-ru";

type ContentMetaPanelProps = {
  meta: PageMeta;
  domainId: string;
  readOnly: boolean;
  onUpdateField: <K extends keyof PageMeta>(key: K, value: PageMeta[K]) => void;
  onUpdateNavLink: (section: NavSection, index: number, field: keyof NavLink, value: string) => void;
  onAddNavLink: (section: NavSection) => void;
  onRemoveNavLink: (section: NavSection, index: number) => void;
};

const t = contentEditorRu.meta;

type NavSectionConfig = {
  key: NavSection;
  label: string;
  dataKey: "headerNav" | "footerNav" | "asideNav";
};

const NAV_SECTIONS: NavSectionConfig[] = [
  { key: "header", label: t.navHeader, dataKey: "headerNav" },
  { key: "footer", label: t.navFooter, dataKey: "footerNav" },
  { key: "aside", label: t.navAside, dataKey: "asideNav" },
];

export function ContentMetaPanel({
  meta,
  domainId,
  readOnly,
  onUpdateField,
  onUpdateNavLink,
  onAddNavLink,
  onRemoveNavLink,
}: ContentMetaPanelProps) {
  const faviconInputRef = useRef<HTMLInputElement | null>(null);
  const logoInputRef = useRef<HTMLInputElement | null>(null);

  const handleFileUpload = async (
    file: File,
    onSuccess: (url: string) => void,
  ) => {
    try {
      const result = await uploadFile(domainId, file);
      const path = result.path.replace(/^\/+/, "");
      const encoded = path
        .split("/")
        .filter(Boolean)
        .map(encodeURIComponent)
        .join("/");
      onSuccess(`${apiBase()}/api/domains/${domainId}/files/${encoded}?raw=1`);
    } catch (err: any) {
      showToast({
        type: "error",
        title: t.uploadError,
        message: err?.message || "Не удалось загрузить файл",
      });
    }
  };

  return (
    <div className="space-y-3">
        {/* Favicon */}
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
            {t.favicon}
          </label>
          <div className="flex gap-1.5">
            <input
              type="text"
              value={meta.favicon}
              onChange={(e) => onUpdateField("favicon", e.target.value)}
              placeholder={t.faviconPlaceholder}
              disabled={readOnly}
              className="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
            {!readOnly && (
              <button
                type="button"
                title={t.upload}
                onClick={() => faviconInputRef.current?.click()}
                className="rounded-lg border border-slate-200 bg-white p-1.5 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
              >
                <FiUpload className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
          <input
            ref={faviconInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) {
                handleFileUpload(file, (url) => onUpdateField("favicon", url));
              }
              if (faviconInputRef.current) faviconInputRef.current.value = "";
            }}
          />
        </div>

        {/* Logo */}
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-500 dark:text-slate-400">
            {t.logo}
          </label>
          <div className="flex gap-1.5">
            <input
              type="text"
              value={meta.logo}
              onChange={(e) => onUpdateField("logo", e.target.value)}
              placeholder={t.logoPlaceholder}
              disabled={readOnly}
              className="flex-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
            {!readOnly && (
              <button
                type="button"
                title={t.upload}
                onClick={() => logoInputRef.current?.click()}
                className="rounded-lg border border-slate-200 bg-white p-1.5 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
              >
                <FiUpload className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
          {meta.logo && (
            <div className="mt-1.5 flex h-8 w-8 items-center justify-center rounded border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800">
              <img
                src={meta.logo}
                alt="Logo"
                className="max-h-full max-w-full object-contain"
              />
            </div>
          )}
          <input
            ref={logoInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) {
                handleFileUpload(file, (url) => onUpdateField("logo", url));
              }
              if (logoInputRef.current) logoInputRef.current.value = "";
            }}
          />
        </div>

        {/* Navigation sections */}
        {NAV_SECTIONS.map(({ key, label, dataKey }) => {
          const links = meta[dataKey];
          // Не показываем секцию если в HTML нет такой навигации и она пустая
          if (links.length === 0 && key !== "header") return null;

          return (
            <div key={key} className="border-t border-slate-100 pt-3 dark:border-slate-800">
              <div className="mb-2 flex items-center justify-between">
                <p className="text-xs font-medium text-slate-500 dark:text-slate-400">
                  {label}
                </p>
                {!readOnly && (
                  <button
                    type="button"
                    onClick={() => onAddNavLink(key)}
                    className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-indigo-600 hover:bg-indigo-50 dark:text-indigo-400 dark:hover:bg-indigo-950/30"
                  >
                    <FiPlus className="h-3 w-3" />
                    {t.addLink}
                  </button>
                )}
              </div>

              {links.length === 0 ? (
                <p className="text-xs text-slate-400 dark:text-slate-500">
                  {t.noLinks}
                </p>
              ) : (
                <div className="space-y-2">
                  {links.map((link, i) => (
                    <div key={i} className="flex items-start gap-1.5">
                      <div className="flex-1 space-y-1">
                        <input
                          type="text"
                          value={link.label}
                          onChange={(e) =>
                            onUpdateNavLink(key, i, "label", e.target.value)
                          }
                          placeholder={t.linkLabel}
                          disabled={readOnly}
                          className="w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
                        />
                        <input
                          type="text"
                          value={link.href}
                          onChange={(e) =>
                            onUpdateNavLink(key, i, "href", e.target.value)
                          }
                          placeholder={t.linkHref}
                          disabled={readOnly}
                          className="w-full rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs text-slate-500 outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-400"
                        />
                      </div>
                      {!readOnly && (
                        <button
                          type="button"
                          onClick={() => onRemoveNavLink(key, i)}
                          className="mt-1 rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                        >
                          <FiTrash2 className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
    </div>
  );
}
