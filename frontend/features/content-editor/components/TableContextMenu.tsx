"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import type { Editor } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import { FiMoreHorizontal } from "react-icons/fi";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.tableMenu;

/** Check if the current selection is a ProseMirror CellSelection (multi-cell select) */
function isCellSelection(state: any): boolean {
  return "$anchorCell" in state.selection;
}

// Stable refs — avoid recreating on every render (breaks BubbleMenu plugin debounce)
// Show when: cursor in table with empty selection OR multi-cell selection in table
const tableBubbleShouldShow = ({ editor: e, state }: { editor: Editor; state: any }) => {
  if (!e.isActive("table")) return false;
  return state.selection.empty || isCellSelection(state);
};
const tableBubbleOptions = { placement: "top" as const, offset: { mainAxis: 8 } };

type TableContextMenuProps = {
  editor: Editor;
};

function MenuItem({
  label,
  onClick,
  danger,
  disabled,
}: {
  label: string;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onMouseDown={(e) => {
        e.preventDefault();
        if (!disabled) onClick();
      }}
      className={`flex w-full items-center px-3 py-1.5 text-left text-sm transition-colors ${
        disabled
          ? "cursor-not-allowed text-slate-300 dark:text-slate-600"
          : danger
            ? "text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-500/10"
            : "text-slate-700 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
      }`}
    >
      {label}
    </button>
  );
}

/** Delete button with confirmation — first click arms, second click executes */
function DangerMenuItem({
  label,
  confirmLabel,
  onClick,
}: {
  label: string;
  confirmLabel: string;
  onClick: () => void;
}) {
  const [armed, setArmed] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  // Auto-disarm after 3 seconds
  useEffect(() => {
    if (!armed) return;
    timerRef.current = setTimeout(() => setArmed(false), 3000);
    return () => clearTimeout(timerRef.current);
  }, [armed]);

  return (
    <button
      type="button"
      onMouseDown={(e) => {
        e.preventDefault();
        if (armed) {
          onClick();
        } else {
          setArmed(true);
        }
      }}
      className={`flex w-full items-center px-3 py-1.5 text-left text-sm transition-colors ${
        armed
          ? "bg-red-500 text-white dark:bg-red-600"
          : "text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-500/10"
      }`}
    >
      {armed ? confirmLabel : label}
    </button>
  );
}

function MenuSection({ label }: { label: string }) {
  return (
    <div className="px-3 pb-0.5 pt-2 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
      {label}
    </div>
  );
}

/** Detect if the table has a header row (first row cells are tableHeader type) */
function hasHeaderRow(editor: Editor): boolean {
  try {
    const { $from } = editor.state.selection;
    let depth = $from.depth;
    while (depth > 0 && $from.node(depth).type.name !== "table") depth--;
    if (depth === 0) return false;
    const table = $from.node(depth);
    const firstRow = table.child(0);
    return firstRow.childCount > 0 && firstRow.child(0).type.name === "tableHeader";
  } catch {
    return false;
  }
}

/** Detect if the table has a header column (first cell of non-header rows is tableHeader) */
function hasHeaderColumn(editor: Editor): boolean {
  try {
    const { $from } = editor.state.selection;
    let depth = $from.depth;
    while (depth > 0 && $from.node(depth).type.name !== "table") depth--;
    if (depth === 0) return false;
    const table = $from.node(depth);
    if (table.childCount < 2) return false;
    // Check second row — if its first cell is tableHeader, header column is on
    const secondRow = table.child(1);
    return secondRow.childCount > 0 && secondRow.child(0).type.name === "tableHeader";
  } catch {
    return false;
  }
}

export function TableContextMenu({ editor }: TableContextMenuProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  // Fix z-index on the floating container
  useEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    let parent = el.parentElement;
    while (parent) {
      const pos = window.getComputedStyle(parent).position;
      if (pos === "absolute" || pos === "fixed") {
        parent.style.zIndex = "9999";
        break;
      }
      parent = parent.parentElement;
    }
  });

  // Close menu on click outside (but not on the "..." trigger itself)
  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as Node;
      // Ignore clicks on the trigger button — it handles its own toggle
      if (triggerRef.current?.contains(target)) return;
      if (menuRef.current && !menuRef.current.contains(target)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [menuOpen]);

  const run = useCallback(
    (fn: () => void) => {
      fn();
      setMenuOpen(false);
    },
    [],
  );

  const canMerge = editor.can().mergeCells();
  const canSplit = editor.can().splitCell();
  const headerRowActive = hasHeaderRow(editor);
  const headerColActive = hasHeaderColumn(editor);

  return (
    <BubbleMenu
      editor={editor}
      pluginKey="tableBubbleMenu"
      shouldShow={tableBubbleShouldShow}
      options={tableBubbleOptions}
      updateDelay={0}
    >
      <div ref={contentRef} className="relative" style={{ zIndex: 9999 }}>
        <div className="flex items-center rounded-lg border border-slate-200 bg-white shadow-lg dark:border-slate-700 dark:bg-slate-900">
          <button
            ref={triggerRef}
            type="button"
            title={t.moreOptions}
            onMouseDown={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setMenuOpen((v) => !v);
            }}
            className="rounded-lg p-2 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
          >
            <FiMoreHorizontal className="h-4 w-4" />
          </button>
        </div>

        {menuOpen && (
          <div
            ref={menuRef}
            className="absolute left-0 top-full mt-1 min-w-[220px] rounded-lg border border-slate-200 bg-white py-1 shadow-xl dark:border-slate-700 dark:bg-slate-900"
          >
            <MenuSection label={t.sectionRows} />
            <MenuItem
              label={t.addRowBefore}
              onClick={() => run(() => editor.chain().focus().addRowBefore().run())}
            />
            <MenuItem
              label={t.addRowAfter}
              onClick={() => run(() => editor.chain().focus().addRowAfter().run())}
            />

            <MenuSection label={t.sectionColumns} />
            <MenuItem
              label={t.addColumnBefore}
              onClick={() => run(() => editor.chain().focus().addColumnBefore().run())}
            />
            <MenuItem
              label={t.addColumnAfter}
              onClick={() => run(() => editor.chain().focus().addColumnAfter().run())}
            />

            <MenuSection label={t.sectionCells} />
            <MenuItem
              label={t.mergeCells}
              onClick={() => run(() => editor.chain().focus().mergeCells().run())}
              disabled={!canMerge}
            />
            <MenuItem
              label={t.splitCell}
              onClick={() => run(() => editor.chain().focus().splitCell().run())}
              disabled={!canSplit}
            />

            <MenuSection label={t.sectionHeaders} />
            <MenuItem
              label={headerRowActive ? t.removeHeaderRow : t.toggleHeaderRow}
              onClick={() => run(() => editor.chain().focus().toggleHeaderRow().run())}
            />
            <MenuItem
              label={headerColActive ? t.removeHeaderColumn : t.toggleHeaderColumn}
              onClick={() => run(() => editor.chain().focus().toggleHeaderColumn().run())}
            />

            <MenuSection label={t.sectionDelete} />
            <DangerMenuItem
              label={t.deleteColumn}
              confirmLabel={t.confirmDeleteColumn}
              onClick={() => run(() => editor.chain().focus().deleteColumn().run())}
            />
            <DangerMenuItem
              label={t.deleteRow}
              confirmLabel={t.confirmDeleteRow}
              onClick={() => run(() => editor.chain().focus().deleteRow().run())}
            />
            <DangerMenuItem
              label={t.deleteTable}
              confirmLabel={t.confirmDeleteTable}
              onClick={() => run(() => editor.chain().focus().deleteTable().run())}
            />
          </div>
        )}
      </div>
    </BubbleMenu>
  );
}
