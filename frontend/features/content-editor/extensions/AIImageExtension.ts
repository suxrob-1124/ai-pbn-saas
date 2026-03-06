import { Node, mergeAttributes } from "@tiptap/core";
import { ReactNodeViewRenderer } from "@tiptap/react";
import { AIImageNodeView } from "../components/AIImageNodeView";

/**
 * Кастомный TipTap Node для inline AI-генерации картинок.
 * Рендерится как React-компонент (NodeView) с полем промпта и кнопкой генерации.
 *
 * Использование: AIImageNode.configure({ domainId: "..." })
 */
export const AIImageNode = Node.create({
  name: "aiImageBlock",
  group: "block",
  atom: true,

  addOptions() {
    return {
      domainId: "",
    };
  },

  addAttributes() {
    return {
      prompt: { default: "" },
      alt: { default: "" },
      src: { default: "" },
      generating: { default: false },
    };
  },

  parseHTML() {
    return [{ tag: 'div[data-type="ai-image-block"]' }];
  },

  renderHTML({ HTMLAttributes }) {
    return ["div", mergeAttributes(HTMLAttributes, { "data-type": "ai-image-block" })];
  },

  addNodeView() {
    return ReactNodeViewRenderer(AIImageNodeView);
  },
});
