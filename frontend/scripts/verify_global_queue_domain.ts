import { readFileSync } from "node:fs";
import path from "node:path";

const filePath = path.join(process.cwd(), "app", "(app)", "queue", "page.tsx");
const content = readFileSync(filePath, "utf8");

const mustContain = (needle: string) => {
  if (!content.includes(needle)) {
    throw new Error(`missing '${needle}'`);
  }
};

const mustNotContain = (needle: string) => {
  if (content.includes(needle)) {
    throw new Error(`unexpected '${needle}'`);
  }
};

mustContain("domain_url");
mustNotContain("domain_id.slice");

console.log("OK");
