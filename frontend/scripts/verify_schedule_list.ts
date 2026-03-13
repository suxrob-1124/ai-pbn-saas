import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "ScheduleList.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain("Редактировать");
mustContain("Удалить расписание?");
mustContain("След. запуск:");
mustContain("Это действие нельзя отменить");
mustContain("ScheduleTrigger");

let missingCaught = false;
try {
  mustContain("NON_EXISTING_LABEL");
} catch (err) {
  missingCaught = true;
  assert.match(String(err), /missing/);
}
assert.ok(missingCaught, "expected missing label check to fail");

console.log("OK");
