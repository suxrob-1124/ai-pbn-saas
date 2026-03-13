import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "app", "(app)", "projects", "[id]", "queue", "page.tsx");
const content = readFileSync(filePath, "utf8");

if (!/const statusOptions = \['all', 'pending', 'queued'\];/.test(content)) {
  throw new Error("active queue status options should contain only all/pending/queued");
}
if (/const statusOptions = \['all', 'pending', 'queued', 'completed', 'failed'\];/.test(content)) {
  throw new Error("legacy active queue status options still present");
}

console.log("OK");
