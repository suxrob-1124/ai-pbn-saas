"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import {
  FiBold,
  FiItalic,
  FiUnderline,
  FiList,
  FiAlignLeft,
  FiAlignCenter,
  FiAlignRight,
  FiLink,
  FiCode,
  FiX,
  FiCheck,
  FiChevronDown,
} from "react-icons/fi";
import {
  LuStrikethrough,
  LuListOrdered,
  LuQuote,
} from "react-icons/lu";
import { contentEditorRu } from "../services/i18n-content-ru";

type EditorBubbleMenuProps = {
  editor: Editor;
};

const t = contentEditorRu.bubbleMenu;

// ── Block type definitions for the dropdown ──
type BlockTypeOption = {
  id: string;
  label: string;
  icon: string;
  command: (editor: Editor) => void;
};

const BLOCK_TYPES: BlockTypeOption[] = [
  {
    id: "paragraph",
    label: t.normalText,
    icon: "Aa",
    command: (e) => e.chain().focus().setParagraph().run(),
  },
  {
    id: "h1",
    label: t.heading1,
    icon: "H1",
    command: (e) => e.chain().focus().toggleHeading({ level: 1 }).run(),
  },
  {
    id: "h2",
    label: t.heading2,
    icon: "H2",
    command: (e) => e.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    id: "h3",
    label: t.heading3,
    icon: "H3",
    command: (e) => e.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    id: "h4",
    label: t.heading4,
    icon: "H4",
    command: (e) => e.chain().focus().toggleHeading({ level: 4 }).run(),
  },
  {
    id: "code",
    label: t.code,
    icon: "</>",
    command: (e) => e.chain().focus().toggleCodeBlock().run(),
  },
];

/** Detect active block type directly from ProseMirror state (more reliable than isActive) */
function getActiveBlockType(editor: Editor): BlockTypeOption {
  try {
    const { $from } = editor.state.selection;
    const node = $from.parent;

    if (node.type.name === "heading") {
      const level = node.attrs.level as number;
      const found = BLOCK_TYPES.find((bt) => bt.id === `h${level}`);
      if (found) return found;
    }
    if (node.type.name === "codeBlock") {
      const found = BLOCK_TYPES.find((bt) => bt.id === "code");
      if (found) return found;
    }
  } catch {
    // fallback
  }
  return BLOCK_TYPES[0]; // paragraph
}

function BubbleBtn({
  icon: Icon,
  title,
  active,
  onClick,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={title}
      onMouseDown={(e) => {
        e.preventDefault();
        onClick();
      }}
      className={`rounded-md p-1.5 transition-colors ${
        active
          ? "bg-indigo-500 text-white"
          : "text-slate-400 hover:bg-slate-700 hover:text-slate-200"
      }`}
    >
      <Icon className="h-3.5 w-3.5" />
    </button>
  );
}

function Divider() {
  return <div className="mx-0.5 h-5 w-px bg-slate-700" />;
}

/** Walks up from el to find the positioned floating container and sets z-index on it. */
function applyZIndex(el: HTMLElement | null) {
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
}

// ── Block type dropdown ──
function BlockTypeDropdown({ editor }: { editor: Editor }) {
  const [open, setOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const activeType = getActiveBlockType(editor);

  // Close dropdown on click outside
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <div ref={dropdownRef} className="relative">
      <button
        type="button"
        onMouseDown={(e) => {
          e.preventDefault();
          setOpen((v) => !v);
        }}
        className="flex items-center gap-0.5 rounded-md px-1.5 py-1 text-xs font-medium text-slate-300 transition-colors hover:bg-slate-700 hover:text-white"
      >
        <span className="font-semibold">{activeType.icon}</span>
        <FiChevronDown className="h-3 w-3" />
      </button>

      {open && (
        <div className="absolute left-0 top-full mt-1 min-w-[180px] rounded-lg border border-slate-700 bg-slate-900 py-1 shadow-xl shadow-black/40">
          {BLOCK_TYPES.map((bt) => {
            const isActive = bt.id === activeType.id;
            return (
              <button
                key={bt.id}
                type="button"
                onMouseDown={(e) => {
                  e.preventDefault();
                  bt.command(editor);
                  setOpen(false);
                }}
                className={`flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors ${
                  isActive
                    ? "bg-indigo-500/20 text-indigo-300"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                }`}
              >
                <span className="w-6 text-center text-xs font-semibold text-slate-500">
                  {bt.icon}
                </span>
                <span>{bt.label}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Stable shouldShow — must NOT be recreated every render
const textBubbleShouldShow = ({ editor: e, state }: { editor: Editor; state: any }) => {
  if (state.selection.empty) return false;
  if (e.isActive("image")) return false;
  if (e.isActive("aiImageBlock")) return false;
  if (e.isActive("codeBlock")) return false;
  // Hide for CellSelection in tables — the table context menu handles that
  if ("$anchorCell" in state.selection) return false;
  return true;
};

// Stable options object
const textBubbleOptions = { placement: "top" as const, offset: { mainAxis: 8 } };

export function EditorBubbleMenu({ editor }: EditorBubbleMenuProps) {
  const [linkMode, setLinkMode] = useState(false);
  const [linkUrl, setLinkUrl] = useState("");
  const linkInputRef = useRef<HTMLInputElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  // Fix z-index: set high z-index on the floating container created by TipTap/floating-ui
  useEffect(() => {
    applyZIndex(contentRef.current);
  });

  const openLinkEdit = useCallback(() => {
    const currentHref = editor.getAttributes("link").href || "";
    setLinkUrl(currentHref);
    setLinkMode(true);
  }, [editor]);

  const applyLink = useCallback(() => {
    const url = linkUrl.trim();
    if (url) {
      editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
    } else {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
    }
    setLinkMode(false);
  }, [editor, linkUrl]);

  const removeLink = useCallback(() => {
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
    setLinkMode(false);
  }, [editor]);

  useEffect(() => {
    if (linkMode) {
      requestAnimationFrame(() => linkInputRef.current?.focus());
    }
  }, [linkMode]);

  useEffect(() => {
    const handler = () => {
      if (editor.state.selection.empty) {
        setLinkMode(false);
      }
    };
    editor.on("selectionUpdate", handler);
    return () => {
      editor.off("selectionUpdate", handler);
    };
  }, [editor]);

  return (
    <BubbleMenu
      editor={editor}
      pluginKey="textBubbleMenu"
      shouldShow={textBubbleShouldShow}
      options={textBubbleOptions}
      updateDelay={0}
    >
      <div
        ref={contentRef}
        className="flex items-center gap-0.5 rounded-xl border border-slate-700 bg-slate-900 px-1 py-0.5 shadow-xl shadow-black/30"
        style={{ zIndex: 9999 }}
      >
        {linkMode ? (
          /* ── Link editing mode ── */
          <div className="flex items-center gap-1 px-1">
            <FiLink className="h-3.5 w-3.5 shrink-0 text-slate-500" />
            <input
              ref={linkInputRef}
              type="url"
              value={linkUrl}
              onChange={(e) => setLinkUrl(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  applyLink();
                }
                if (e.key === "Escape") {
                  e.preventDefault();
                  setLinkMode(false);
                }
              }}
              placeholder={t.linkPlaceholder}
              className="w-48 bg-transparent px-1 py-1 text-xs text-slate-200 outline-none placeholder:text-slate-600"
            />
            <button
              type="button"
              title={t.linkApply}
              onMouseDown={(e) => {
                e.preventDefault();
                applyLink();
              }}
              className="rounded p-1 text-green-400 hover:bg-slate-700"
            >
              <FiCheck className="h-3.5 w-3.5" />
            </button>
            {editor.isActive("link") && (
              <button
                type="button"
                title={t.linkRemove}
                onMouseDown={(e) => {
                  e.preventDefault();
                  removeLink();
                }}
                className="rounded p-1 text-red-400 hover:bg-slate-700"
              >
                <FiX className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        ) : (
          /* ── Normal mode ── */
          <>
            {/* Block type dropdown */}
            <BlockTypeDropdown editor={editor} />

            <Divider />

            {/* Inline formatting */}
            <BubbleBtn
              icon={FiBold}
              title={t.bold}
              active={editor.isActive("bold")}
              onClick={() => editor.chain().focus().toggleBold().run()}
            />
            <BubbleBtn
              icon={FiItalic}
              title={t.italic}
              active={editor.isActive("italic")}
              onClick={() => editor.chain().focus().toggleItalic().run()}
            />
            <BubbleBtn
              icon={FiUnderline}
              title={t.underline}
              active={editor.isActive("underline")}
              onClick={() => editor.chain().focus().toggleUnderline().run()}
            />
            <BubbleBtn
              icon={LuStrikethrough}
              title={t.strike}
              active={editor.isActive("strike")}
              onClick={() => editor.chain().focus().toggleStrike().run()}
            />
            <BubbleBtn
              icon={FiCode}
              title={t.code}
              active={editor.isActive("code")}
              onClick={() => editor.chain().focus().toggleCode().run()}
            />

            <Divider />

            {/* Quote */}
            <BubbleBtn
              icon={LuQuote}
              title={t.blockquote}
              active={editor.isActive("blockquote")}
              onClick={() => editor.chain().focus().toggleBlockquote().run()}
            />

            {/* Lists */}
            <BubbleBtn
              icon={FiList}
              title={t.bulletList}
              active={editor.isActive("bulletList")}
              onClick={() => editor.chain().focus().toggleBulletList().run()}
            />
            <BubbleBtn
              icon={LuListOrdered}
              title={t.orderedList}
              active={editor.isActive("orderedList")}
              onClick={() => editor.chain().focus().toggleOrderedList().run()}
            />

            <Divider />

            {/* Alignment */}
            <BubbleBtn
              icon={FiAlignLeft}
              title={t.alignLeft}
              active={editor.isActive({ textAlign: "left" })}
              onClick={() => editor.chain().focus().setTextAlign("left").run()}
            />
            <BubbleBtn
              icon={FiAlignCenter}
              title={t.alignCenter}
              active={editor.isActive({ textAlign: "center" })}
              onClick={() => editor.chain().focus().setTextAlign("center").run()}
            />
            <BubbleBtn
              icon={FiAlignRight}
              title={t.alignRight}
              active={editor.isActive({ textAlign: "right" })}
              onClick={() => editor.chain().focus().setTextAlign("right").run()}
            />

            <Divider />

            {/* Link */}
            <BubbleBtn
              icon={FiLink}
              title={t.link}
              active={editor.isActive("link")}
              onClick={openLinkEdit}
            />
          </>
        )}
      </div>
    </BubbleMenu>
  );
}
