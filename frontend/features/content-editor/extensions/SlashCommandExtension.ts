"use client";

import { Extension } from "@tiptap/core";
import { PluginKey } from "@tiptap/pm/state";
import { ReactRenderer } from "@tiptap/react";
import Suggestion, { type SuggestionOptions } from "@tiptap/suggestion";
import tippy, { type Instance } from "tippy.js";
import { ContentSlashMenu, type ContentSlashMenuRef } from "../components/ContentSlashMenu";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.slashMenu;

export type SlashCommandItem = {
  title: string;
  description?: string;
  icon: string;
  category: string;
  keywords?: string[];
  command: (props: { editor: any; range: any }) => void;
};

/** Validates that a ProseMirror range is still within the current document. */
function isRangeValid(editor: any, range: { from: number; to: number }): boolean {
  try {
    const docSize = editor.state.doc.content.size;
    return range.from >= 0 && range.to >= range.from && range.to <= docSize;
  } catch {
    return false;
  }
}

/** Safely executes a slash command, catching stale-range errors. */
function safeCommand(editor: any, range: any, fn: () => void) {
  if (!isRangeValid(editor, range)) return;
  try {
    fn();
  } catch {
    // Stale range — ProseMirror may throw RangeError; ignore silently
  }
}

const slashCommandItems: SlashCommandItem[] = [
  // ── Text ──
  {
    title: t.heading1,
    description: t.heading1Desc,
    icon: "H1",
    category: "text",
    keywords: ["заголовок", "heading", "h1"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 1 }).run(),
      );
    },
  },
  {
    title: t.heading2,
    description: t.heading2Desc,
    icon: "H2",
    category: "text",
    keywords: ["заголовок", "heading", "h2"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 2 }).run(),
      );
    },
  },
  {
    title: t.heading3,
    description: t.heading3Desc,
    icon: "H3",
    category: "text",
    keywords: ["заголовок", "heading", "h3"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 3 }).run(),
      );
    },
  },
  {
    title: t.heading4,
    description: t.heading4Desc,
    icon: "H4",
    category: "text",
    keywords: ["заголовок", "heading", "h4"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setNode("heading", { level: 4 }).run(),
      );
    },
  },
  {
    title: t.paragraph,
    description: t.paragraphDesc,
    icon: "P",
    category: "text",
    keywords: ["текст", "paragraph", "абзац"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setNode("paragraph").run(),
      );
    },
  },

  // ── Lists ──
  {
    title: t.bulletList,
    description: t.bulletListDesc,
    icon: "•",
    category: "lists",
    keywords: ["список", "bullet", "ul"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).toggleBulletList().run(),
      );
    },
  },
  {
    title: t.orderedList,
    description: t.orderedListDesc,
    icon: "1.",
    category: "lists",
    keywords: ["нумерованный", "ordered", "ol"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).toggleOrderedList().run(),
      );
    },
  },

  // ── Media ──
  {
    title: t.imageLibrary,
    description: t.imageLibraryDesc,
    icon: "\uD83D\uDDBC",
    category: "media",
    keywords: ["изображение", "картинка", "image", "фото"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () => {
        editor.chain().focus().deleteRange(range).run();
        window.dispatchEvent(new CustomEvent("content-editor:open-image-library"));
      });
    },
  },
  {
    title: t.image,
    description: t.imageDesc,
    icon: "\uD83C\uDFA8",
    category: "media",
    keywords: ["ai", "генерация", "generate"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).insertContent({ type: "aiImageBlock", attrs: {} }).run(),
      );
    },
  },

  // ── Blocks ──
  {
    title: t.quote,
    description: t.quoteDesc,
    icon: "\u201C",
    category: "blocks",
    keywords: ["цитата", "blockquote", "quote"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).toggleBlockquote().run(),
      );
    },
  },
  {
    title: t.codeBlock,
    description: t.codeBlockDesc,
    icon: "</>",
    category: "blocks",
    keywords: ["код", "code", "программа"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).toggleCodeBlock().run(),
      );
    },
  },
  {
    title: t.table,
    description: t.tableDesc,
    icon: "\u229E",
    category: "blocks",
    keywords: ["таблица", "table"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).insertTable({ rows: 2, cols: 2, withHeaderRow: true }).run(),
      );
    },
  },
  {
    title: t.divider,
    description: t.dividerDesc,
    icon: "\u2014",
    category: "blocks",
    keywords: ["разделитель", "линия", "divider", "hr"],
    command: ({ editor, range }) => {
      safeCommand(editor, range, () =>
        editor.chain().focus().deleteRange(range).setHorizontalRule().run(),
      );
    },
  },
];

export const SlashCommandPluginKey = new PluginKey("slashCommand");

export const SlashCommandExtension = Extension.create({
  name: "slashCommand",

  addOptions() {
    return {
      suggestion: {
        char: "/",
        pluginKey: SlashCommandPluginKey,
        items: ({ query }: { query: string }) => {
          if (!query) return slashCommandItems;
          const q = query.toLowerCase();
          return slashCommandItems.filter(
            (item) =>
              item.title.toLowerCase().includes(q) ||
              item.keywords?.some((k) => k.toLowerCase().includes(q)),
          );
        },
        command: ({
          editor,
          range,
          props,
        }: {
          editor: any;
          range: any;
          props: SlashCommandItem;
        }) => {
          props.command({ editor, range });
        },
        render: () => {
          let component: ReactRenderer<ContentSlashMenuRef> | null = null;
          let popup: Instance[] | null = null;

          return {
            onStart: (props: any) => {
              component = new ReactRenderer(ContentSlashMenu, {
                props,
                editor: props.editor,
              });

              if (!props.clientRect) return;

              popup = tippy("body", {
                getReferenceClientRect: props.clientRect,
                appendTo: () => document.body,
                content: component.element,
                showOnCreate: true,
                interactive: true,
                trigger: "manual",
                placement: "bottom-start",
              });
            },

            onUpdate: (props: any) => {
              component?.updateProps(props);

              if (!props.clientRect || !popup?.[0]) return;

              popup[0].setProps({
                getReferenceClientRect: props.clientRect,
              });
            },

            onKeyDown: (props: any) => {
              if (props.event.key === "Escape") {
                popup?.[0]?.hide();
                return true;
              }
              return component?.ref?.onKeyDown(props) ?? false;
            },

            onExit: () => {
              popup?.[0]?.destroy();
              component?.destroy();
            },
          };
        },
      } satisfies Partial<SuggestionOptions>,
    };
  },

  addProseMirrorPlugins() {
    return [
      Suggestion({
        editor: this.editor,
        ...this.options.suggestion,
      }),
    ];
  },
});
