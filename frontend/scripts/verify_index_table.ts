import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");
const tablePath = path.join(root, "frontend/components/indexing/IndexTable.tsx");
assert.ok(fs.existsSync(tablePath), "IndexTable component must exist");

const content = fs.readFileSync(tablePath, "utf8");
const requiredLabels = [
  "Домен",
  "Дата",
  "Статус",
  "Попытки",
  "В индексе",
  "Последняя попытка",
  "Следующий ретрай",
  "Запустить",
  "История",
  "Действия"
];

requiredLabels.forEach((label) => {
  assert.ok(content.includes(label), `IndexTable must include label: ${label}`);
});

assert.ok(content.includes("SortableTh"), "IndexTable must support sorting");
assert.ok(content.includes("onRunNow"), "IndexTable must support run-now action");
assert.ok(content.includes("IndexCheckHistoryCard"), "IndexTable must render history card");

console.log("OK");
