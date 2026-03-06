import { contentEditorRu } from "./i18n-content-ru";

const t = contentEditorRu.slashMenu;

export const SLASH_CATEGORIES = [
  { id: "text", label: t.categoryText },
  { id: "lists", label: t.categoryLists },
  { id: "media", label: t.categoryMedia },
  { id: "blocks", label: t.categoryBlocks },
] as const;
