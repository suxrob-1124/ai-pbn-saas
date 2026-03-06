"use client";

import { useEffect, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import { FiPlus } from "react-icons/fi";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.tableMenu;

type Rect = { top: number; left: number; width: number; height: number };

/** Find table depth from current selection */
function findTableDepth(editor: Editor): number | null {
  const { $from } = editor.state.selection;
  let depth = $from.depth;
  while (depth > 0) {
    if ($from.node(depth).type.name === "table") return depth;
    depth--;
  }
  return null;
}

/** Get the table DOM rect relative to a container element */
function getTableRect(editor: Editor, containerEl: HTMLElement): Rect | null {
  const depth = findTableDepth(editor);
  if (depth === null) return null;

  try {
    const tablePos = editor.state.selection.$from.before(depth);
    const dom = editor.view.nodeDOM(tablePos);
    if (!dom || !(dom instanceof HTMLElement)) return null;

    const table = dom.querySelector("table") ?? dom;
    const tableRect = table.getBoundingClientRect();
    const containerRect = containerEl.getBoundingClientRect();

    return {
      top: tableRect.top - containerRect.top,
      left: tableRect.left - containerRect.left,
      width: tableRect.width,
      height: tableRect.height,
    };
  } catch {
    return null;
  }
}

/** Move cursor to the last row's first cell and add a row after it */
function addRowAtEnd(editor: Editor) {
  const depth = findTableDepth(editor);
  if (depth === null) return;

  const { $from } = editor.state.selection;
  const table = $from.node(depth);
  const tableContentStart = $from.start(depth);

  // Sum nodeSize of all rows except the last to find last row offset
  let offset = 0;
  for (let i = 0; i < table.childCount - 1; i++) {
    offset += table.child(i).nodeSize;
  }

  // Position inside last row's first cell content
  // tableContentStart + offset = before last row
  // +1 = inside last row (before first cell)
  // +1 = inside first cell content
  const pos = tableContentStart + offset + 1 + 1;

  editor.chain().focus().setTextSelection(pos).addRowAfter().run();
}

/** Move cursor to the last column's cell and add a column after it */
function addColumnAtEnd(editor: Editor) {
  const depth = findTableDepth(editor);
  if (depth === null) return;

  const { $from } = editor.state.selection;
  const table = $from.node(depth);
  const tableContentStart = $from.start(depth);

  // Use first row to find last column
  const firstRow = table.child(0);
  let cellOffset = 0;
  for (let i = 0; i < firstRow.childCount - 1; i++) {
    cellOffset += firstRow.child(i).nodeSize;
  }

  // tableContentStart = inside table, before first row
  // +1 = inside first row, before first cell
  // +cellOffset = before last cell
  // +1 = inside last cell content
  const pos = tableContentStart + 1 + cellOffset + 1;

  editor.chain().focus().setTextSelection(pos).addColumnAfter().run();
}

type TableAddButtonsProps = {
  editor: Editor;
};

export function TableAddButtons({ editor }: TableAddButtonsProps) {
  const [rect, setRect] = useState<Rect | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const update = () => {
      if (!editor.isActive("table")) {
        setRect(null);
        return;
      }
      const editorEl = containerRef.current?.closest(
        ".rounded-xl",
      ) as HTMLElement | null;
      if (!editorEl) {
        setRect(null);
        return;
      }
      setRect(getTableRect(editor, editorEl));
    };

    editor.on("selectionUpdate", update);
    editor.on("transaction", update);

    return () => {
      editor.off("selectionUpdate", update);
      editor.off("transaction", update);
    };
  }, [editor]);

  if (!rect) return <div ref={containerRef} className="hidden" />;

  return (
    <div ref={containerRef}>
      {/* Add row — full width below table */}
      <button
        type="button"
        title={t.addRow}
        onMouseDown={(e) => {
          e.preventDefault();
          e.stopPropagation();
          addRowAtEnd(editor);
        }}
        className="absolute flex items-center justify-center rounded-b-lg border border-t-0 border-slate-200 bg-slate-50 text-slate-400 transition-colors hover:bg-indigo-50 hover:text-indigo-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-500 dark:hover:bg-indigo-900/30 dark:hover:text-indigo-400"
        style={{
          left: rect.left,
          top: rect.top + rect.height,
          width: rect.width,
          height: 24,
          zIndex: 10,
        }}
      >
        <FiPlus className="h-3.5 w-3.5" />
      </button>

      {/* Add column — full height to the right of table */}
      <button
        type="button"
        title={t.addColumn}
        onMouseDown={(e) => {
          e.preventDefault();
          e.stopPropagation();
          addColumnAtEnd(editor);
        }}
        className="absolute flex items-center justify-center rounded-r-lg border border-l-0 border-slate-200 bg-slate-50 text-slate-400 transition-colors hover:bg-indigo-50 hover:text-indigo-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-500 dark:hover:bg-indigo-900/30 dark:hover:text-indigo-400"
        style={{
          left: rect.left + rect.width,
          top: rect.top,
          width: 24,
          height: rect.height,
          zIndex: 10,
        }}
      >
        <FiPlus className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}
