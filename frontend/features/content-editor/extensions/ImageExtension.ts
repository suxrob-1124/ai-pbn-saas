import Image from "@tiptap/extension-image";
import { mergeAttributes } from "@tiptap/core";
import { ReactNodeViewRenderer } from "@tiptap/react";
import { ImageNodeView } from "../components/ImageNodeView";

/**
 * Custom Image extension with alignment, link wrapping, and React NodeView.
 * Extends the standard @tiptap/extension-image.
 */
export const CustomImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      alignment: {
        default: "center",
        parseHTML: (element) => {
          // Check data attribute
          const dataAlign = element.getAttribute("data-alignment");
          if (dataAlign) return dataAlign;
          // Infer from style
          const style = element.getAttribute("style") || "";
          if (style.includes("margin-right: auto") && !style.includes("margin-left: auto"))
            return "left";
          if (style.includes("margin-left: auto") && !style.includes("margin-right: auto"))
            return "right";
          return "center";
        },
        renderHTML: (attributes) => {
          if (!attributes.alignment || attributes.alignment === "center") return {};
          return { "data-alignment": attributes.alignment };
        },
      },
      linkHref: {
        default: null,
        parseHTML: (element) => {
          // If img is wrapped in <a>, get href from parent
          const parent = element.parentElement;
          if (parent?.tagName === "A") {
            return parent.getAttribute("href");
          }
          return element.getAttribute("data-link-href");
        },
        renderHTML: (attributes) => {
          if (!attributes.linkHref) return {};
          return { "data-link-href": attributes.linkHref };
        },
      },
    };
  },

  parseHTML() {
    return [
      {
        tag: "img[src]",
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    // alignment/linkHref are rendered as data-* attrs by addAttributes.renderHTML,
    // so read raw values from node.attrs directly.
    const align = node.attrs.alignment || "center";
    const linkHref = node.attrs.linkHref;

    // Remove rendered data-* attrs so they don't duplicate in the output
    const { "data-alignment": _da, "data-link-href": _dl, ...imgAttrs } = HTMLAttributes;

    const mergedAttrs = mergeAttributes(this.options.HTMLAttributes, imgAttrs);

    // Apply alignment via style
    if (align === "left") {
      mergedAttrs.style = "display: block; margin-right: auto;";
    } else if (align === "right") {
      mergedAttrs.style = "display: block; margin-left: auto;";
    } else {
      mergedAttrs.style = "display: block; margin-left: auto; margin-right: auto;";
    }

    if (linkHref) {
      return ["a", { href: linkHref, target: "_blank", rel: "noopener noreferrer" }, ["img", mergedAttrs]];
    }

    return ["img", mergedAttrs];
  },

  addNodeView() {
    return ReactNodeViewRenderer(ImageNodeView);
  },
});
