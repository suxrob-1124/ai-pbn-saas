import { readFileSync } from "node:fs";
import path from "node:path";

const pagePath = path.join(process.cwd(), "app", "projects", "[id]", "queue", "page.tsx");
const apiPath = path.join(process.cwd(), "lib", "queueApi.ts");

const pageContent = readFileSync(pagePath, "utf8");
const apiContent = readFileSync(apiPath, "utf8");

if (!pageContent.includes("История запусков")) {
  throw new Error("history block title is missing");
}
if (!pageContent.includes("loadHistory")) {
  throw new Error("history loader is missing");
}
if (!apiContent.includes("/queue/history")) {
  throw new Error("queue history API path is missing");
}

console.log("OK");
