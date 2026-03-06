const PAGE_NAME_MAP: Record<string, string> = {
  "index.html": "Главная",
  "about.html": "О нас",
  "about-us.html": "О нас",
  "contact.html": "Контакты",
  "contacts.html": "Контакты",
  "services.html": "Услуги",
  "blog.html": "Блог",
  "privacy.html": "Политика конфиденциальности",
  "privacy-policy.html": "Политика конфиденциальности",
  "terms.html": "Условия использования",
  "faq.html": "Вопросы и ответы",
  "404.html": "Страница 404",
  "portfolio.html": "Портфолио",
  "gallery.html": "Галерея",
  "team.html": "Команда",
  "pricing.html": "Цены",
  "testimonials.html": "Отзывы",
  "careers.html": "Карьера",
  "news.html": "Новости",
};

export function getPageDisplayName(path: string): string {
  const fileName = path.split("/").pop() || path;
  const parts = path.split("/");

  // Directory-based pages like "about/index.html" — use parent dir name
  if (fileName.toLowerCase() === "index.html" && parts.length >= 2) {
    const dirName = parts[parts.length - 2];
    const dirMapped = PAGE_NAME_MAP[`${dirName.toLowerCase()}.html`];
    if (dirMapped) return dirMapped;
    return dirName
      .replace(/[-_]/g, " ")
      .replace(/\b\w/g, (c) => c.toUpperCase());
  }

  // Flat files like "about.html", "index.html" (root)
  const mapped = PAGE_NAME_MAP[fileName.toLowerCase()];
  if (mapped) return mapped;

  // Full path legacy mapping
  const mapped2 = PAGE_NAME_MAP[path.toLowerCase()];
  if (mapped2) return mapped2;

  const withoutExt = fileName.replace(/\.html?$/i, "");
  return withoutExt
    .replace(/[-_]/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function isHtmlFile(path: string): boolean {
  return /\.html?$/i.test(path);
}

/**
 * Converts user slug input to a directory-based path: slug/index.html
 * Examples:
 *   "about"       → "about/index.html"
 *   "blog/post1"  → "blog/post1/index.html"
 *   "Контакты"    → "kontakty/index.html"
 */
export function slugToFilePath(slug: string): string {
  const segments = slug
    .split("/")
    .map((s) =>
      s
        .toLowerCase()
        .replace(/\.html?$/i, "")
        .replace(/[^a-z0-9а-яё-]/gi, "-")
        .replace(/-+/g, "-")
        .replace(/^-|-$/g, ""),
    )
    .filter(Boolean);

  const dir = segments.length > 0 ? segments.join("/") : "new-page";
  return `${dir}/index.html`;
}

/** @deprecated Use slugToFilePath instead */
export function slugToFileName(slug: string): string {
  return slugToFilePath(slug);
}
