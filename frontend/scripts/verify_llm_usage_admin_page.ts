import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(__dirname, "..", "..");

const adminPagePath = path.join(root, "frontend/app/(app)/monitoring/llm-usage/page.tsx");
assert.ok(fs.existsSync(adminPagePath), "Admin LLM usage page must exist");
const page = fs.readFileSync(adminPagePath, "utf8");

assert.ok(page.includes("Расход токенов (LLM Usage)"), "Admin page must render title");
assert.ok(page.includes("listAdminLLMUsageEvents"), "Admin page must load events API");
assert.ok(page.includes("listAdminLLMUsageStats"), "Admin page must load stats API");
assert.ok(page.includes("listAdminLLMPricing"), "Admin page must load pricing API");
assert.ok(page.includes("UsageCostValue"), "Admin page must show cost KPI component");

const layoutPath = path.join(root, "frontend/app/(app)/layout.tsx");
const layout = fs.readFileSync(layoutPath, "utf8");
assert.ok(layout.includes("/monitoring/llm-usage"), "App layout monitoring nav must include LLM Usage link");

console.log("OK");
