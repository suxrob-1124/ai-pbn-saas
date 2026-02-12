import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";

import { apiBase } from "../lib/http";
import { listAdmin } from "../lib/indexChecksApi";

const root = path.resolve(__dirname, "..", "..");

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" }
  });

async function main() {
  process.env.NEXT_PUBLIC_API_URL = "http://example.test";

  const calls: Array<{ url: string; method: string }> = [];
  const base = apiBase();

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const method = init?.method ? init.method.toUpperCase() : "GET";
    calls.push({ url, method });

    const parsed = new URL(url);
    if (parsed.pathname === "/api/admin/index-checks" && method === "GET") {
      return json(200, {
        items: [
          {
            id: "check-1",
            domain_id: "domain-1",
            check_date: new Date().toISOString(),
            status: "success",
            attempts: 1,
            created_at: new Date().toISOString()
          }
        ],
        total: 42
      });
    }

    return json(500, { error: "unexpected request" });
  };

  const list = await listAdmin({
    status: "success,checking",
    isIndexed: true,
    from: new Date("2026-02-01T00:00:00Z"),
    to: new Date("2026-02-12T00:00:00Z"),
    domainId: "domain-1",
    search: "example.com",
    sort: "check_date:desc",
    limit: 20,
    page: 2
  });

  assert.equal(list.items.length, 1);
  assert.equal(list.total, 42);

  const call = calls.find((item) => item.url.startsWith(`${base}/api/admin/index-checks`));
  assert.ok(call, "expected listAdmin call");
  if (call) {
    const params = new URL(call.url).searchParams;
    assert.equal(params.get("status"), "success,checking");
    assert.equal(params.get("is_indexed"), "true");
    assert.equal(params.get("from"), "2026-02-01T00:00:00.000Z");
    assert.equal(params.get("to"), "2026-02-12T00:00:00.000Z");
    assert.equal(params.get("domain_id"), "domain-1");
    assert.equal(params.get("search"), "example.com");
    assert.equal(params.get("sort"), "check_date:desc");
    assert.equal(params.get("limit"), "20");
    assert.equal(params.get("page"), "2");
  }

  const pagePath = path.join(root, "frontend/app/monitoring/indexing/page.tsx");
  const page = fs.readFileSync(pagePath, "utf8");
  assert.ok(page.includes("page * limit < totalChecks"), "page should use total for next page");
  assert.ok(page.includes("setPage(totalPages)"), "page should clamp page when total shrinks");

  const tablePath = path.join(root, "frontend/components/indexing/IndexTable.tsx");
  const table = fs.readFileSync(tablePath, "utf8");
  assert.ok(table.includes("SortableTh"), "IndexTable must include SortableTh for sorting");

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
