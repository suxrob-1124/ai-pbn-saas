import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "LinkTaskList.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain("Массовый повтор");
mustContain("Массовое удаление");
mustContain("Изменить");
mustContain("pending");
mustContain("searching");
mustContain("inserted");
mustContain("generated");
mustContain("failed");
mustContain("statusFilter");

let missingCaught = false;
try {
  mustContain("NON_EXISTING_LINK_TASK_LIST_TOKEN");
} catch (err) {
  missingCaught = true;
  assert.match(String(err), /missing/);
}
assert.ok(missingCaught, "expected missing check to fail");

console.log("OK");
