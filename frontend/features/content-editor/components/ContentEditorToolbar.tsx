"use client";

import { useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import { FiMinus, FiImage, FiUpload } from "react-icons/fi";
import { LuTable, LuSparkles } from "react-icons/lu";
import { uploadFile } from "@/lib/fileApi";
import { apiBase } from "@/lib/http";
import { showToast } from "@/lib/toastStore";
import { contentEditorRu } from "../services/i18n-content-ru";
import { ImageLibraryModal } from "./ImageLibraryModal";

type ContentEditorToolbarProps = {
  editor: Editor | null;
  domainId: string;
};

const t = contentEditorRu.toolbar;

type ToolbarButton = {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  action: (editor: Editor) => void;
};

const INSERT_BUTTONS: ToolbarButton[] = [
  {
    icon: LuTable,
    title: t.table,
    action: (e) =>
      e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
  },
  {
    icon: FiMinus,
    title: t.horizontalRule,
    action: (e) => e.chain().focus().setHorizontalRule().run(),
  },
  {
    icon: LuSparkles,
    title: t.aiImage,
    action: (e) => {
      e.chain()
        .focus()
        .insertContent({ type: "aiImageBlock", attrs: {} })
        .run();
    },
  },
];

export function ContentEditorToolbar({ editor, domainId }: ContentEditorToolbarProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [libraryOpen, setLibraryOpen] = useState(false);

  if (!editor) return null;

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !domainId) return;
    try {
      const result = await uploadFile(domainId, file);
      const path = result.path.replace(/^\/+/, "");
      const encoded = path.split("/").filter(Boolean).map(encodeURIComponent).join("/");
      const src = `${apiBase()}/api/domains/${domainId}/files/${encoded}?raw=1`;
      editor.chain().focus().setImage({ src, alt: file.name }).run();
    } catch (err: any) {
      showToast({ type: "error", title: "Ошибка загрузки", message: err?.message || "Не удалось загрузить файл" });
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const btnClass =
    "rounded-md p-1.5 transition-colors text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200";

  return (
    <div className="sticky top-0 z-10 flex items-center gap-0.5 rounded-t-xl border-b border-slate-200 bg-white px-2 py-1.5 dark:border-slate-700 dark:bg-slate-900">
      {/* Insert buttons */}
      {INSERT_BUTTONS.map((btn) => (
        <button
          key={btn.title}
          type="button"
          title={btn.title}
          onClick={() => btn.action(editor)}
          className={btnClass}
        >
          <btn.icon className="h-4 w-4" />
        </button>
      ))}

      <div className="mx-1 h-5 w-px bg-slate-200 dark:bg-slate-700" />

      {/* Image library */}
      <button
        type="button"
        title={t.image}
        onClick={() => setLibraryOpen(true)}
        className={btnClass}
      >
        <FiImage className="h-4 w-4" />
      </button>

      {/* Upload image */}
      <button
        type="button"
        title={t.uploadImage}
        onClick={() => fileInputRef.current?.click()}
        className={btnClass}
      >
        <FiUpload className="h-4 w-4" />
      </button>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleUpload}
      />

      <ImageLibraryModal
        open={libraryOpen}
        onClose={() => setLibraryOpen(false)}
        domainId={domainId}
        onSelect={(src, alt) => {
          editor.chain().focus().setImage({ src, alt }).run();
          setLibraryOpen(false);
        }}
      />
    </div>
  );
}
