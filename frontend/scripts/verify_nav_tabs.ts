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
const projectPage = path.join(root, "app", "(app)", "projects", "[id]", "page.tsx");
const domainPage = path.join(root, "app", "(app)", "domains", "[id]", "page.tsx");
const projectHeader = path.join(
  root,
  "features",
  "domain-project",
  "components",
  "ProjectHeaderActionsSection.tsx"
);

const projectContent = readFile(projectPage);
const domainContent = readFile(domainPage);
const projectHeaderContent = readFile(projectHeader);

assertContains(projectContent, "ProjectHeaderActionsSection", "project header actions");
assertContains(projectContent, "ProjectSchedulesSection", "project schedules section");
assertContains(projectHeaderContent, "Очередь", "project queue CTA");
assertContains(projectHeaderContent, "/monitoring/indexing", "project monitoring CTA");
assertContains(projectHeaderContent, "LLM Usage", "project usage CTA");
assertContains(domainContent, "DomainLinkStatusSection", "domain link section");
assertContains(domainContent, "DomainResultSection", "domain result section");
assert.throws(() => assertContains(domainContent, "LinkTaskForm", "legacy domain link form"), /LinkTaskForm/);

assert.throws(() => assertContains("", "ProjectHeaderActionsSection", "project header actions"), /ProjectHeaderActionsSection/);

console.log("OK");
