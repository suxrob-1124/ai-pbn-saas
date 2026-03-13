import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const adminPagePath = path.join(root, "frontend/app/(app)/monitoring/llm-usage/page.tsx");
const adminPage = fs.readFileSync(adminPagePath, "utf8");
const usageCostValuePath = path.join(root, "frontend/features/llm-usage/components/UsageCostValue.tsx");
const usageCostValue = fs.readFileSync(usageCostValuePath, "utf8");

assert.ok(adminPage.includes("UsageCostValue"), "Admin LLM usage page must render estimated badge");
assert.ok(
  usageCostValue.includes("нет активного тарифа модели на момент запроса"),
  "UsageCostValue must explain n/a cost in tooltip"
);
assert.ok(adminPage.includes("UsageTokenSourceBadge"), "Admin page must render token source badge");

const projectPagePath = path.join(root, "frontend/app/(app)/projects/[id]/usage/page.tsx");
const projectPage = fs.readFileSync(projectPagePath, "utf8");
assert.ok(
  projectPage.includes("UsageCostValue"),
  "Project usage page must include cost KPI"
);
assert.ok(projectPage.includes("estimated_cost_usd"), "Project usage table must display per-event cost");

console.log("OK");
