import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "CSVImport.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

mustContain("Импортировать все");
mustContain("Предпросмотр");
mustContain("parseCsv");
mustContain("anchor_text,target_url,scheduled_for");

let missingCaught = false;
try {
  mustContain("NON_EXISTING_CSV_IMPORT_TOKEN");
} catch (err) {
  missingCaught = true;
  assert.match(String(err), /missing/);
}
assert.ok(missingCaught, "expected missing check to fail");

console.log("OK");
