import type { SEOData } from "../types/content-editor";

/**
 * Парсит SEO-метатеги из полного HTML-документа.
 */
export function parseSeoFromHtml(html: string): SEOData {
  const empty: SEOData = { title: "", description: "", ogTitle: "", ogDescription: "" };
  if (!html.trim()) return empty;

  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");

  const title = doc.querySelector("title")?.textContent?.trim() || "";
  const description =
    doc.querySelector('meta[name="description"]')?.getAttribute("content")?.trim() || "";
  const ogTitle =
    doc.querySelector('meta[property="og:title"]')?.getAttribute("content")?.trim() || "";
  const ogDescription =
    doc.querySelector('meta[property="og:description"]')?.getAttribute("content")?.trim() || "";

  return { title, description, ogTitle, ogDescription };
}
