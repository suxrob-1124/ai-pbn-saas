"use client";

import { useMemo } from "react";
import type { CSSVariable, CSSVariableCategory } from "../services/cssVariableExtraction";
import { contentEditorRu } from "../services/i18n-content-ru";

type ContentCssVarsPanelProps = {
  variables: CSSVariable[];
  readOnly: boolean;
  onUpdateVariable: (index: number, value: string) => void;
};

const t = contentEditorRu.cssVars;

const CATEGORY_ORDER: CSSVariableCategory[] = ["color", "size", "font", "other"];
const CATEGORY_LABELS: Record<CSSVariableCategory, string> = {
  color: t.colors,
  size: t.sizes,
  font: t.fonts,
  other: t.other,
};

function prettyName(name: string): string {
  return name
    .replace(/^--/, "")
    .replace(/^[\w]+-/, "") // strip prefix like "x9a3b-"
    .replace(/-/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * Конвертирует любой CSS-цвет (hex, rgb, hsl, oklch, named) в #rrggbb
 * для <input type="color">. Использует canvas API браузера.
 * Возвращает null если конвертация невозможна.
 */
function cssColorToHex(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;

  // Быстрый путь для hex
  if (/^#[0-9a-f]{6}$/i.test(trimmed)) return trimmed.toLowerCase();
  if (/^#[0-9a-f]{3}$/i.test(trimmed)) {
    const h = trimmed.slice(1);
    return `#${h[0]}${h[0]}${h[1]}${h[1]}${h[2]}${h[2]}`.toLowerCase();
  }

  // Canvas API конвертирует hsl, rgb, oklch, lch, lab, hwb, named colors → hex
  try {
    const ctx = document.createElement("canvas").getContext("2d");
    if (!ctx) return null;
    ctx.fillStyle = "#010203"; // sentinel
    ctx.fillStyle = trimmed;
    const result = ctx.fillStyle;
    if (result === "#010203") return null; // цвет не распознан
    if (result.startsWith("#")) return result;

    // rgba(r, g, b, a) — canvas возвращает для цветов с альфой
    const rgba = result.match(/rgba?\(\s*(\d+),\s*(\d+),\s*(\d+)/);
    if (rgba) {
      const r = Number(rgba[1]).toString(16).padStart(2, "0");
      const g = Number(rgba[2]).toString(16).padStart(2, "0");
      const b = Number(rgba[3]).toString(16).padStart(2, "0");
      return `#${r}${g}${b}`;
    }
    return null;
  } catch {
    return null;
  }
}

export function ContentCssVarsPanel({
  variables,
  readOnly,
  onUpdateVariable,
}: ContentCssVarsPanelProps) {
  const grouped = useMemo(() => {
    const map = new Map<CSSVariableCategory, { variable: CSSVariable; index: number }[]>();
    for (const cat of CATEGORY_ORDER) map.set(cat, []);
    variables.forEach((v, i) => {
      const list = map.get(v.category) || map.get("other")!;
      list.push({ variable: v, index: i });
    });
    return map;
  }, [variables]);

  if (variables.length === 0) {
    return (
      <div className="flex min-h-[120px] items-center justify-center">
        <p className="text-xs text-slate-400 dark:text-slate-500">{t.noVars}</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {CATEGORY_ORDER.map((cat) => {
        const items = grouped.get(cat)!;
        if (items.length === 0) return null;
        return (
          <div key={cat}>
            <p className="mb-2 text-[10px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">
              {CATEGORY_LABELS[cat]}
            </p>
            <div className="space-y-2">
              {items.map(({ variable, index }) => {
                const hex = cat === "color" ? cssColorToHex(variable.value) : null;
                return (
                  <div key={variable.name}>
                    <label className="mb-0.5 block text-xs text-slate-500 dark:text-slate-400">
                      {prettyName(variable.name)}
                    </label>
                    <div className="flex items-center gap-1.5">
                      {hex && (
                        <input
                          type="color"
                          value={hex}
                          onChange={(e) => onUpdateVariable(index, e.target.value)}
                          disabled={readOnly}
                          className="h-7 w-7 shrink-0 cursor-pointer rounded border border-slate-200 bg-transparent p-0.5 dark:border-slate-700 disabled:cursor-not-allowed"
                        />
                      )}
                      <input
                        type="text"
                        value={variable.value}
                        onChange={(e) => onUpdateVariable(index, e.target.value)}
                        disabled={readOnly}
                        className="flex-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
