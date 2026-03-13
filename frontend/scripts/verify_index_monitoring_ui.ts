import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const read = (p: string) => fs.readFileSync(path.join(root, p), "utf8");

const appLayout = read("frontend/app/(app)/layout.tsx");
assert.ok(appLayout.includes("/monitoring/indexing"), "App layout must link to /monitoring/indexing");
assert.ok(
  appLayout.includes("Мониторинг"),
  "App layout must include Monitoring label"
);

const projectPage = read("frontend/app/(app)/projects/[id]/page.tsx");
const projectHeaderSection = read("frontend/features/domain-project/components/ProjectHeaderActionsSection.tsx");
const projectDomainsSection = read("frontend/features/domain-project/components/ProjectDomainsSection.tsx");
assert.ok(
  projectPage.includes("Индексация") ||
    projectHeaderSection.includes("Индексация") ||
    projectDomainsSection.includes("Проверки индексации"),
  "Project domains must include Индексация link label"
);
assert.ok(
  projectPage.includes("/monitoring/indexing") ||
    projectHeaderSection.includes("/monitoring/indexing") ||
    projectDomainsSection.includes("/monitoring/indexing"),
  "Project domains must link to monitoring indexing"
);

const indexingPagePath = path.join(root, "frontend/app/(app)/monitoring/indexing/page.tsx");
assert.ok(fs.existsSync(indexingPagePath), "Indexing page must exist");
const indexingPage = fs.readFileSync(indexingPagePath, "utf8");
assert.ok(indexingPage.includes("listByDomain"), "Indexing page must use listByDomain");
assert.ok(indexingPage.includes("listAdmin"), "Indexing page must use listAdmin");
assert.ok(indexingPage.includes("useIndexCheckHistory"), "Indexing page must use history hook");
const indexTablePath = path.join(root, "frontend/components/indexing/IndexTable.tsx");
assert.ok(fs.existsSync(indexTablePath), "IndexTable component must exist");
const indexTable = fs.readFileSync(indexTablePath, "utf8");
assert.ok(
  indexTable.includes("IndexCheckHistoryCard"),
  "IndexTable must reuse IndexCheckHistoryCard component"
);
const historyCardPath = path.join(root, "frontend/components/IndexCheckHistoryCard.tsx");
assert.ok(fs.existsSync(historyCardPath), "IndexCheckHistoryCard component must exist");

console.log("OK");
