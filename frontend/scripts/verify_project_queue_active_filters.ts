import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "app", "projects", "[id]", "queue", "page.tsx");
const content = readFileSync(filePath, "utf8");

if (!content.includes('const statusOptions = ["all", "pending", "queued"];')) {
  throw new Error("active queue status options should contain only all/pending/queued");
}
if (content.includes('const statusOptions = ["all", "pending", "queued", "completed", "failed"];')) {
  throw new Error("legacy active queue status options still present");
}

console.log("OK");
