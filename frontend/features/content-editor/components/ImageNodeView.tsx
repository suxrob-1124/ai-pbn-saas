"use client";

import { NodeViewWrapper } from "@tiptap/react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  FiAlignLeft,
  FiAlignCenter,
  FiAlignRight,
  FiType,
  FiLink,
  FiTrash2,
  FiCheck,
  FiX,
} from "react-icons/fi";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.imageControls;

type EditMode = null | "alt" | "link";

export function ImageNodeView(props: any) {
  const { node, updateAttributes, deleteNode, selected } = props;
  const { src, alt, alignment, linkHref } = node.attrs;

  const [showToolbar, setShowToolbar] = useState(false);
  const [editMode, setEditMode] = useState<EditMode>(null);
  const [editValue, setEditValue] = useState("");
  const wrapperRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Show toolbar when node is selected (but don't auto-hide on deselect)
  useEffect(() => {
    if (selected) {
      setShowToolbar(true);
    }
  }, [selected]);

  // Close toolbar on click outside wrapper
  useEffect(() => {
    if (!showToolbar) return;
    const handler = (e: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setShowToolbar(false);
        setEditMode(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [showToolbar]);

  // Focus input when entering edit mode
  useEffect(() => {
    if (editMode) {
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [editMode]);

  const setAlignment = useCallback(
    (align: string) => {
      updateAttributes({ alignment: align });
    },
    [updateAttributes],
  );

  const openAltEdit = useCallback(() => {
    setEditValue(alt || "");
    setEditMode("alt");
  }, [alt]);

  const openLinkEdit = useCallback(() => {
    setEditValue(linkHref || "");
    setEditMode("link");
  }, [linkHref]);

  const applyEdit = useCallback(() => {
    if (editMode === "alt") {
      updateAttributes({ alt: editValue });
    } else if (editMode === "link") {
      updateAttributes({ linkHref: editValue.trim() || null });
    }
    setEditMode(null);
  }, [editMode, editValue, updateAttributes]);

  const removeLink = useCallback(() => {
    updateAttributes({ linkHref: null });
    setEditMode(null);
  }, [updateAttributes]);

  const alignClass =
    alignment === "left"
      ? "mr-auto"
      : alignment === "right"
        ? "ml-auto"
        : "mx-auto";

  const btnClass = (active?: boolean) =>
    `rounded-md p-1.5 transition-colors ${
      active
        ? "bg-indigo-500/20 text-indigo-400"
        : "text-slate-400 hover:bg-slate-700 hover:text-slate-200"
    }`;

  /** Prevent event from reaching ProseMirror so the image stays selected */
  const stopAndRun = (e: React.MouseEvent, fn: () => void) => {
    e.preventDefault();
    e.stopPropagation();
    fn();
  };

  return (
    <NodeViewWrapper
      ref={wrapperRef}
      className={`relative my-4 max-w-full ${alignClass}`}
      style={{ width: "fit-content" }}
      data-drag-handle
    >
      {/* Image */}
      <img
        src={src}
        alt={alt || ""}
        className={`max-w-full rounded-lg transition-shadow ${
          showToolbar
            ? "ring-2 ring-indigo-500/50 shadow-lg"
            : "hover:ring-1 hover:ring-slate-400/30"
        }`}
        onClick={() => setShowToolbar(true)}
        draggable={false}
      />

      {/* Floating toolbar */}
      {showToolbar && (
        <div className="absolute -top-11 left-1/2 z-50 flex -translate-x-1/2 items-center gap-0.5 rounded-xl border border-slate-700 bg-slate-900 px-1 py-0.5 shadow-xl shadow-black/30">
          {editMode ? (
            /* ── Inline edit mode ── */
            <div className="flex items-center gap-1 px-1">
              {editMode === "link" ? (
                <FiLink className="h-3.5 w-3.5 shrink-0 text-slate-500" />
              ) : (
                <FiType className="h-3.5 w-3.5 shrink-0 text-slate-500" />
              )}
              <input
                ref={inputRef}
                type="text"
                value={editValue}
                onChange={(e) => setEditValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    applyEdit();
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    setEditMode(null);
                  }
                }}
                onMouseDown={(e) => e.stopPropagation()}
                placeholder={editMode === "alt" ? t.altPlaceholder : t.linkPlaceholder}
                className="w-48 bg-transparent px-1 py-1 text-xs text-slate-200 outline-none placeholder:text-slate-600"
              />
              <button
                type="button"
                title={editMode === "alt" ? t.altText : t.addLink}
                onMouseDown={(e) => stopAndRun(e, applyEdit)}
                className="rounded p-1 text-green-400 hover:bg-slate-700"
              >
                <FiCheck className="h-3.5 w-3.5" />
              </button>
              {editMode === "link" && linkHref && (
                <button
                  type="button"
                  title={t.removeLink}
                  onMouseDown={(e) => stopAndRun(e, removeLink)}
                  className="rounded p-1 text-red-400 hover:bg-slate-700"
                >
                  <FiX className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
          ) : (
            /* ── Normal mode ── */
            <>
              {/* Alignment */}
              <button
                type="button"
                title={t.alignLeft}
                onMouseDown={(e) => stopAndRun(e, () => setAlignment("left"))}
                className={btnClass(alignment === "left")}
              >
                <FiAlignLeft className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                title={t.alignCenter}
                onMouseDown={(e) => stopAndRun(e, () => setAlignment("center"))}
                className={btnClass(!alignment || alignment === "center")}
              >
                <FiAlignCenter className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                title={t.alignRight}
                onMouseDown={(e) => stopAndRun(e, () => setAlignment("right"))}
                className={btnClass(alignment === "right")}
              >
                <FiAlignRight className="h-3.5 w-3.5" />
              </button>

              <div className="mx-0.5 h-5 w-px bg-slate-700" />

              {/* Alt text */}
              <button
                type="button"
                title={t.altText}
                onMouseDown={(e) => stopAndRun(e, openAltEdit)}
                className={btnClass(!!alt)}
              >
                <FiType className="h-3.5 w-3.5" />
              </button>

              {/* Link */}
              <button
                type="button"
                title={t.addLink}
                onMouseDown={(e) => stopAndRun(e, openLinkEdit)}
                className={btnClass(!!linkHref)}
              >
                <FiLink className="h-3.5 w-3.5" />
              </button>

              <div className="mx-0.5 h-5 w-px bg-slate-700" />

              {/* Delete */}
              <button
                type="button"
                title={t.delete}
                onMouseDown={(e) => stopAndRun(e, deleteNode)}
                className="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-red-500/20 hover:text-red-400"
              >
                <FiTrash2 className="h-3.5 w-3.5" />
              </button>
            </>
          )}
        </div>
      )}
    </NodeViewWrapper>
  );
}
