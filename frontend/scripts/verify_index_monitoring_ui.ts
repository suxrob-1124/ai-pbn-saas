import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const read = (p: string) => fs.readFileSync(path.join(root, p), "utf8");

const navbar = read("frontend/components/Navbar.tsx");
assert.ok(navbar.includes("/monitoring/indexing"), "Navbar must link to /monitoring/indexing");
assert.ok(
  navbar.toLowerCase().includes("monitoring"),
  "Navbar must include Monitoring label"
);

const projectPage = read("frontend/app/projects/[id]/page.tsx");
assert.ok(
  projectPage.includes("Index checks"),
  "Project domains must include Index checks link label"
);
assert.ok(
  projectPage.includes("/monitoring/indexing"),
  "Project domains must link to monitoring indexing"
);

const indexingPagePath = path.join(root, "frontend/app/monitoring/indexing/page.tsx");
assert.ok(fs.existsSync(indexingPagePath), "Indexing page must exist");
const indexingPage = fs.readFileSync(indexingPagePath, "utf8");
assert.ok(indexingPage.includes("listByDomain"), "Indexing page must use listByDomain");
assert.ok(indexingPage.includes("listAdmin"), "Indexing page must use listAdmin");
assert.ok(indexingPage.includes("listDomainHistory"), "Indexing page must use history API");
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
