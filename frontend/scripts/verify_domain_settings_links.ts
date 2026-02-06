import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "domains", "[id]", "page.tsx");
const content = readFileSync(pagePath, "utf8");

const mustContain = (needle: string, label: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing ${label}: ${needle}`);
  }
};

const mustNotContain = (needle: string, label: string) => {
  if (content.includes(needle)) {
    throw new Error(`unexpected ${label}: ${needle}`);
  }
};

mustContain("Анкор", "anchor label");
mustContain("Акцептор", "acceptor label");
mustNotContain("Ссылки", "links tab");
mustNotContain("LinkTaskForm", "link form component");
mustNotContain("LinkTaskList", "link list component");
mustNotContain("CSVImport", "csv import component");

console.log("OK");
