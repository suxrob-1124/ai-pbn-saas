import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const projectUsagePath = path.join(root, "frontend/app/projects/[id]/usage/page.tsx");
assert.ok(fs.existsSync(projectUsagePath), "Project LLM usage page must exist");
const page = fs.readFileSync(projectUsagePath, "utf8");

assert.ok(page.includes("LLM Usage проекта"), "Project page must render title");
assert.ok(page.includes("listProjectLLMUsageEvents"), "Project page must call project events API");
assert.ok(page.includes("listProjectLLMUsageStats"), "Project page must call project stats API");
assert.ok(page.includes("Estimated cost (USD)"), "Project page must show cost KPI");

const projectPagePath = path.join(root, "frontend/app/projects/[id]/page.tsx");
const projectPage = fs.readFileSync(projectPagePath, "utf8");
const projectHeaderActionsPath = path.join(
  root,
  "frontend/features/domain-project/components/ProjectHeaderActionsSection.tsx"
);
const projectHeaderActions = fs.readFileSync(projectHeaderActionsPath, "utf8");
const usageLinkSource = `${projectPage}\n${projectHeaderActions}`;
assert.ok(usageLinkSource.includes("/usage"), "Project scope must contain usage route link");
assert.ok(usageLinkSource.includes("LLM Usage"), "Project scope must contain LLM Usage CTA label");

console.log("OK");
