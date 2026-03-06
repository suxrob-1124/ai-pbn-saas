"use client";

import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { TableKit } from "@tiptap/extension-table";
import TextAlign from "@tiptap/extension-text-align";
import Highlight from "@tiptap/extension-highlight";
import { useEffect, useRef, useState } from "react";
import { EditorBubbleMenu } from "./EditorBubbleMenu";
import { TableContextMenu } from "./TableContextMenu";
import { TableAddButtons } from "./TableAddButtons";
import { ImageLibraryModal } from "./ImageLibraryModal";
import { contentEditorRu } from "../services/i18n-content-ru";
import { SlashCommandExtension } from "../extensions/SlashCommandExtension";
import { AIImageNode } from "../extensions/AIImageExtension";
import { CustomImage } from "../extensions/ImageExtension";

type ContentEditorProps = {
  content: string;
  onChange: (html: string) => void;
  readOnly: boolean;
  domainId: string;
};

const t = contentEditorRu.editor;

export function ContentEditor({ content, onChange, readOnly, domainId }: ContentEditorProps) {
  const isExternalUpdate = useRef(false);
  const [slashLibraryOpen, setSlashLibraryOpen] = useState(false);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3, 4] },
        link: {
          openOnClick: false,
          autolink: true,
        },
      }),
      Placeholder.configure({
        placeholder: t.placeholder,
      }),
      CustomImage.configure({
        inline: false,
        allowBase64: false,
      }),
      TableKit.configure({
        table: {
          resizable: true,
          cellMinWidth: 80,
        },
      }),
      TextAlign.configure({
        types: ["heading", "paragraph"],
      }),
      Highlight,
      SlashCommandExtension,
      AIImageNode.configure({ domainId }),
    ],
    immediatelyRender: false,
    content,
    editable: !readOnly,
    onUpdate: ({ editor: e }) => {
      if (isExternalUpdate.current) return;
      onChange(e.getHTML());
    },
    editorProps: {
      attributes: {
        class:
          "prose prose-slate dark:prose-invert prose-lg mx-auto max-w-3xl min-h-[500px] px-8 py-6 outline-none focus:outline-none",
      },
    },
  });

  useEffect(() => {
    if (!editor) return;
    const currentHtml = editor.getHTML();
    if (currentHtml === content) return;
    isExternalUpdate.current = true;
    try {
      editor.commands.setContent(content || "<p></p>");
    } catch {
      // ProseMirror may throw RangeError on malformed content — ignore silently
    }
    isExternalUpdate.current = false;
  }, [content, editor]);

  useEffect(() => {
    if (!editor) return;
    editor.setEditable(!readOnly);
  }, [readOnly, editor]);

  // Listen for slash menu "open image library" event
  useEffect(() => {
    const handler = () => setSlashLibraryOpen(true);
    window.addEventListener("content-editor:open-image-library", handler);
    return () => window.removeEventListener("content-editor:open-image-library", handler);
  }, []);

  return (
    <div className="relative rounded-xl border border-slate-200 bg-white shadow dark:border-slate-800 dark:bg-slate-950">
      <EditorContent editor={editor} />
      {!readOnly && editor && <EditorBubbleMenu editor={editor} />}
      {!readOnly && editor && <TableContextMenu editor={editor} />}
      {!readOnly && editor && <TableAddButtons editor={editor} />}
      {!readOnly && (
        <div className="border-t border-slate-100 px-4 py-1.5 text-xs text-slate-400 dark:border-slate-800 dark:text-slate-500">
          {t.slashHint}
        </div>
      )}

      {/* Image library triggered from slash menu */}
      <ImageLibraryModal
        open={slashLibraryOpen}
        onClose={() => setSlashLibraryOpen(false)}
        domainId={domainId}
        onSelect={(src, alt) => {
          editor?.chain().focus().setImage({ src, alt }).run();
          setSlashLibraryOpen(false);
        }}
      />
    </div>
  );
}
