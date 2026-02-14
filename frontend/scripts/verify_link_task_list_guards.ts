import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "components", "LinkTaskList.tsx");
const content = readFileSync(filePath, "utf8");

const requiredTokens = [
  "canRetryLinkTask",
  "canEditLinkTask",
  "canDeleteLinkTask",
  "disabled={loading || !canRetry}",
  "disabled={loading || !canEdit}",
  "disabled={loading || !canDelete}"
];

for (const token of requiredTokens) {
  if (!content.includes(token)) {
    throw new Error(`missing '${token}'`);
  }
}

console.log("OK");
