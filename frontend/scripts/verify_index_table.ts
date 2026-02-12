import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");
const tablePath = path.join(root, "frontend/components/IndexTable.tsx");
assert.ok(fs.existsSync(tablePath), "IndexTable component must exist");

const content = fs.readFileSync(tablePath, "utf8");
const requiredLabels = [
  "Домен",
  "Дата",
  "Статус",
  "Attempts",
  "Indexed",
  "Last attempt",
  "Next retry",
  "Run now",
  "Open domain",
  "Статусы",
  "С даты",
  "По дату",
  "В индексе"
];

requiredLabels.forEach((label) => {
  assert.ok(content.includes(label), `IndexTable must include label: ${label}`);
});

assert.ok(content.includes("SortableTh"), "IndexTable must support sorting");
assert.ok(content.includes("pageSize"), "IndexTable must support pagination");

console.log("OK");
