import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "LinkTaskForm.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain('type="url"');
mustContain('type="datetime-local"');
mustContain("Сохранить и добавить ещё");
mustContain("Сохранить");

let missingCaught = false;
try {
  mustContain("NON_EXISTING_LINK_TASK_FORM_TOKEN");
} catch (err) {
  missingCaught = true;
  assert.match(String(err), /missing/);
}
assert.ok(missingCaught, "expected missing check to fail");

console.log("OK");
