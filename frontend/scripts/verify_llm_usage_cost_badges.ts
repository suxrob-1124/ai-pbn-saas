import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const adminPagePath = path.join(root, "frontend/app/monitoring/llm-usage/page.tsx");
const adminPage = fs.readFileSync(adminPagePath, "utf8");

assert.ok(adminPage.includes("estimated"), "Admin LLM usage page must render estimated badge");
assert.ok(
  adminPage.includes("нет активного тарифа модели на момент запроса"),
  "Admin LLM usage page must explain n/a cost in tooltip"
);
assert.ok(adminPage.includes("item.token_source !== \"provider\""), "Estimated badge should depend on token source");

const projectPagePath = path.join(root, "frontend/app/projects/[id]/usage/page.tsx");
const projectPage = fs.readFileSync(projectPagePath, "utf8");
assert.ok(
  projectPage.includes("Estimated cost (USD)") || projectPage.includes("Оценочная стоимость (USD)"),
  "Project usage page must include cost KPI"
);
assert.ok(projectPage.includes("item.estimated_cost_usd"), "Project usage table must display per-event cost");

console.log("OK");
