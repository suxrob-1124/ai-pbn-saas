import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "(app)", "domains", "[id]", "page.tsx");
const pageContent = readFileSync(pagePath, "utf8");

const mustContain = (content: string, needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

const mustNotContain = (content: string, needle: string) => {
  if (content.includes(needle)) {
    throw new Error(`unexpected '${needle}'`);
  }
};

mustNotContain(pageContent, "LinkTaskForm");
mustNotContain(pageContent, "LinkTaskList");
mustNotContain(pageContent, "CSVImport");
mustContain(pageContent, "DomainLinkStatusSection");
mustContain(pageContent, "linkTasks");
mustContain(pageContent, "linkNotice");

const csvComponentPath = path.join(process.cwd(), "components", "CSVImport.tsx");
const csvContent = readFileSync(csvComponentPath, "utf8");
mustContain(csvContent, "onImport");
mustContain(csvContent, "drag");

let unexpectedCaught = false;
try {
  mustNotContain(pageContent, "UNEXPECTED_LINK_UI_TOKEN");
} catch (err) {
  unexpectedCaught = true;
  assert.match(String(err), /unexpected/);
}
assert.ok(!unexpectedCaught, "unexpected check should not fail");

console.log("OK");
