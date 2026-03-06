"use client";

import type { SEOData } from "../types/content-editor";
import type { PageMeta, NavLink, NavSection } from "../services/htmlMetaExtraction";
import type { CSSVariable } from "../services/cssVariableExtraction";
import { ContentSEOPanel } from "./ContentSEOPanel";
import { ContentMetaPanel } from "./ContentMetaPanel";
import { ContentCssVarsPanel } from "./ContentCssVarsPanel";
import { contentEditorRu } from "../services/i18n-content-ru";

export type SettingsTab = "seo" | "meta" | "css";

type ContentSettingsPanelProps = {
  activeTab: SettingsTab;
  onTabChange: (tab: SettingsTab) => void;
  // SEO
  seo: SEOData;
  onUpdateSeo: <K extends keyof SEOData>(key: K, value: string) => void;
  currentPath: string | undefined;
  readOnly: boolean;
  // Meta
  meta: PageMeta;
  domainId: string;
  onUpdateMetaField: <K extends keyof PageMeta>(key: K, value: PageMeta[K]) => void;
  onUpdateNavLink: (section: NavSection, index: number, field: keyof NavLink, value: string) => void;
  onAddNavLink: (section: NavSection) => void;
  onRemoveNavLink: (section: NavSection, index: number) => void;
  // CSS Variables
  cssVariables: CSSVariable[];
  onUpdateCssVariable: (index: number, value: string) => void;
};

const ts = contentEditorRu.settingsPanel;

const TABS: { key: SettingsTab; label: string }[] = [
  { key: "seo", label: ts.tabSeo },
  { key: "meta", label: ts.tabMeta },
  { key: "css", label: ts.tabCss },
];

export function ContentSettingsPanel({
  activeTab,
  onTabChange,
  seo,
  onUpdateSeo,
  currentPath,
  readOnly,
  meta,
  domainId,
  onUpdateMetaField,
  onUpdateNavLink,
  onAddNavLink,
  onRemoveNavLink,
  cssVariables,
  onUpdateCssVariable,
}: ContentSettingsPanelProps) {
  return (
    <aside className="w-[320px] shrink-0 self-start rounded-xl border border-slate-200 bg-white/80 shadow dark:border-slate-800 dark:bg-slate-900/60">
      {/* Tab bar */}
      <div className="flex border-b border-slate-200 dark:border-slate-700">
        {TABS.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => onTabChange(tab.key)}
            className={`flex-1 px-3 py-2.5 text-xs font-semibold transition-colors ${
              activeTab === tab.key
                ? "border-b-2 border-indigo-500 text-indigo-700 dark:text-indigo-400"
                : "text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content — scrollable */}
      <div className="overflow-y-auto p-4" style={{ maxHeight: "calc(100vh - 180px)" }}>
        {activeTab === "seo" && (
          <ContentSEOPanel
            seo={seo}
            onUpdate={onUpdateSeo}
            currentPath={currentPath}
            readOnly={readOnly}
          />
        )}

        {activeTab === "meta" && (
          <ContentMetaPanel
            meta={meta}
            domainId={domainId}
            readOnly={readOnly}
            onUpdateField={onUpdateMetaField}
            onUpdateNavLink={onUpdateNavLink}
            onAddNavLink={onAddNavLink}
            onRemoveNavLink={onRemoveNavLink}
          />
        )}

        {activeTab === "css" && (
          <ContentCssVarsPanel
            variables={cssVariables}
            readOnly={readOnly}
            onUpdateVariable={onUpdateCssVariable}
          />
        )}
      </div>
    </aside>
  );
}
