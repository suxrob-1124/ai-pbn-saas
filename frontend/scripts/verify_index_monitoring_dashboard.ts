import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const read = (p: string) => fs.readFileSync(path.join(root, p), "utf8");

const pagePath = "frontend/app/monitoring/indexing/page.tsx";
const page = read(pagePath);

const requiredComponents = [
  "IndexFiltersBar",
  "IndexCalendar",
  "IndexTable",
  "IndexStats",
  "FailedChecksAlert"
];

requiredComponents.forEach((name) => {
  assert.ok(page.includes(name), `page must reference ${name}`);
});

const queryKeys = ["status", "from", "to", "domainId", "isIndexed", "search", "sort"];
queryKeys.forEach((key) => {
  assert.ok(
    page.includes(`\"${key}\"`) || page.includes(`'${key}'`),
    `page must sync query param ${key}`
  );
});

const componentFiles = [
  "frontend/components/IndexFiltersBar.tsx",
  "frontend/components/IndexCalendar.tsx",
  "frontend/components/FailedChecksAlert.tsx",
  "frontend/components/indexing/IndexFiltersBar.tsx",
  "frontend/components/indexing/IndexCalendar.tsx",
  "frontend/components/indexing/IndexTable.tsx",
  "frontend/components/indexing/IndexStats.tsx",
  "frontend/components/indexing/FailedChecksAlert.tsx"
];

componentFiles.forEach((file) => {
  const full = path.join(root, file);
  assert.ok(fs.existsSync(full), `${file} must exist`);
});

console.log("OK");
