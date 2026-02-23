import type { AIContextModeOption, EditorModelOption } from "../types/ai";

const defaultModel = process.env.NEXT_PUBLIC_GEMINI_DEFAULT_MODEL || "gemini-2.5-flash";

export const EDITOR_MODEL_OPTIONS: EditorModelOption[] = [
  { value: "", label: `По умолчанию (${defaultModel})` },
  { value: "gemini-3-pro-preview", label: "gemini-3-pro-preview" },
  { value: "gemini-2.5-pro", label: "gemini-2.5-pro" },
  { value: "gemini-2.5-flash", label: "gemini-2.5-flash" },
  { value: "gemini-2.5-flash-image", label: "gemini-2.5-flash-image" },
  { value: "gemini-1.5-pro", label: "gemini-1.5-pro" },
  { value: "gemini-1.5-flash", label: "gemini-1.5-flash" },
];

export const EDITOR_IMAGE_MODEL_OPTIONS: EditorModelOption[] = [
  { value: "", label: "По умолчанию (gemini-2.5-flash-image)" },
  { value: "gemini-2.5-flash-image", label: "gemini-2.5-flash-image" },
];

export const AI_CONTEXT_MODE_OPTIONS: AIContextModeOption[] = [
  { value: "auto", label: "Авто (рекомендуется)" },
  { value: "hybrid", label: "Гибрид: авто + выбранные файлы" },
  { value: "manual", label: "Только выбранные файлы" },
];
