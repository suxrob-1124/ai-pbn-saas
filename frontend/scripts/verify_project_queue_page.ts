import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "app", "projects", "[id]", "queue", "page.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain("Удалить из очереди");
mustContain("Фильтр по статусу");
mustContain("Фильтр по дате");
mustContain("Приоритет");

let missingCaught = false;
try {
  mustContain("NON_EXISTING_QUEUE_TOKEN");
} catch (err) {
  missingCaught = true;
  assert.match(String(err), /missing/);
}
assert.ok(missingCaught, "expected missing token check to fail");

console.log("OK");
