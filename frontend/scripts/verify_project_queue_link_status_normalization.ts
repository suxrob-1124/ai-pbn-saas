import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "projects", "[id]", "queue", "page.tsx");
const content = readFileSync(pagePath, "utf8");

if (!content.includes("normalizeLinkTaskStatus(task.status)")) {
  throw new Error("project queue should normalize link task statuses");
}
if (!content.includes("canRetryLinkTask(task.status)")) {
  throw new Error("project queue should use retry guard by canonical status");
}

console.log("OK");
