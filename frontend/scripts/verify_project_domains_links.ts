import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "(app)", "projects", "[id]", "page.tsx");
const pageContent = readFileSync(pagePath, "utf8");

const mustContain = (content: string, needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain(pageContent, "Анкор");
mustContain(pageContent, "Акцептор");
mustContain(pageContent, "Добавить ссылку");
mustContain(pageContent, "Обновить ссылку");
mustContain(pageContent, "/link/run");

console.log("OK");
