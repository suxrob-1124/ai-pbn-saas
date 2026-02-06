import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const readFile = (filePath: string) => readFileSync(filePath, "utf8");

const assertContains = (content: string, needle: string, label: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing ${label}: ${needle}`);
  }
};

const root = process.cwd();
const projectPage = path.join(root, "app", "projects", "[id]", "page.tsx");
const domainPage = path.join(root, "app", "domains", "[id]", "page.tsx");

const projectContent = readFile(projectPage);
const domainContent = readFile(domainPage);

assertContains(projectContent, "Расписания", "project tabs");
assertContains(projectContent, "Очередь", "project tabs");
assert.throws(() => assertContains(domainContent, "Ссылки", "domain tabs"), /Ссылки/);

assert.throws(() => assertContains("", "Расписания", "project tabs"), /Расписания/);

console.log("OK");
