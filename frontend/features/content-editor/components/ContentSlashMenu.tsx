"use client";

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useState,
  useCallback,
} from "react";
import type { SlashCommandItem } from "../extensions/SlashCommandExtension";
import { SLASH_CATEGORIES } from "../services/slashMenuCategories";

type ContentSlashMenuProps = {
  items: SlashCommandItem[];
  command: (item: SlashCommandItem) => void;
};

export type ContentSlashMenuRef = {
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
};

const CATEGORY_ORDER = SLASH_CATEGORIES.map((c) => c.id);
const CATEGORY_LABELS: Record<string, string> = Object.fromEntries(
  SLASH_CATEGORIES.map((c) => [c.id, c.label]),
);

export const ContentSlashMenu = forwardRef<ContentSlashMenuRef, ContentSlashMenuProps>(
  ({ items, command }, ref) => {
    const [selectedIndex, setSelectedIndex] = useState(0);

    // Group items by category, preserving category order
    const grouped = useMemo(() => {
      const map = new Map<string, SlashCommandItem[]>();
      for (const item of items) {
        const cat = item.category || "blocks";
        if (!map.has(cat)) map.set(cat, []);
        map.get(cat)!.push(item);
      }
      // Sort by category order
      const sorted = new Map<string, SlashCommandItem[]>();
      for (const catId of CATEGORY_ORDER) {
        if (map.has(catId)) sorted.set(catId, map.get(catId)!);
      }
      // Add any remaining
      for (const [catId, catItems] of map) {
        if (!sorted.has(catId)) sorted.set(catId, catItems);
      }
      return sorted;
    }, [items]);

    // Flat list for keyboard navigation
    const flatItems = useMemo(() => {
      const result: SlashCommandItem[] = [];
      for (const [, catItems] of grouped) {
        result.push(...catItems);
      }
      return result;
    }, [grouped]);

    useEffect(() => {
      setSelectedIndex(0);
    }, [items]);

    const selectItem = useCallback(
      (index: number) => {
        const item = flatItems[index];
        if (item) command(item);
      },
      [flatItems, command],
    );

    useImperativeHandle(ref, () => ({
      onKeyDown: ({ event }) => {
        if (event.key === "ArrowUp") {
          setSelectedIndex((prev) => (prev + flatItems.length - 1) % flatItems.length);
          return true;
        }
        if (event.key === "ArrowDown") {
          setSelectedIndex((prev) => (prev + 1) % flatItems.length);
          return true;
        }
        if (event.key === "Enter") {
          selectItem(selectedIndex);
          return true;
        }
        return false;
      },
    }));

    if (flatItems.length === 0) return null;

    let globalIndex = 0;

    return (
      <div className="z-50 min-w-[240px] max-h-[320px] overflow-y-auto rounded-xl border border-slate-200 bg-white py-1 shadow-lg dark:border-slate-700 dark:bg-slate-900">
        {Array.from(grouped.entries()).map(([categoryId, categoryItems]) => (
          <div key={categoryId}>
            {/* Category header */}
            <div className="px-3 pb-0.5 pt-2 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
              {CATEGORY_LABELS[categoryId] || categoryId}
            </div>
            {/* Items */}
            {categoryItems.map((item) => {
              const idx = globalIndex++;
              const isSelected = idx === selectedIndex;
              return (
                <button
                  key={item.title}
                  type="button"
                  onClick={() => selectItem(idx)}
                  className={`flex w-full items-center gap-2.5 px-3 py-1.5 text-left transition-colors ${
                    isSelected
                      ? "bg-indigo-50 text-indigo-700 dark:bg-indigo-500/10 dark:text-indigo-400"
                      : "text-slate-600 hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-slate-800"
                  }`}
                >
                  <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-slate-100 text-xs dark:bg-slate-800">
                    {item.icon}
                  </span>
                  <div className="flex min-w-0 flex-col">
                    <span className="text-sm leading-tight">{item.title}</span>
                    {item.description && (
                      <span className="truncate text-[11px] leading-tight text-slate-400 dark:text-slate-500">
                        {item.description}
                      </span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        ))}
      </div>
    );
  },
);

ContentSlashMenu.displayName = "ContentSlashMenu";
