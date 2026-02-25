import assert from "node:assert/strict";

import { apiBase } from "../lib/http";
import { listAdminLLMUsageEvents } from "../lib/llmUsageApi";

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" }
  });

async function main() {
  process.env.NEXT_PUBLIC_API_URL = "http://example.test";
  const base = apiBase();
  const calls: Array<{ url: string; method: string }> = [];

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const method = init?.method ? init.method.toUpperCase() : "GET";
    calls.push({ url, method });
    const parsed = new URL(url);
    if (parsed.pathname === "/api/admin/llm-usage/events" && method === "GET") {
      return json(200, {
        items: [
          {
            id: "evt-1",
            created_at: new Date().toISOString(),
            provider: "gemini",
            operation: "editor_ai_suggest",
            model: "gemini-2.5-pro",
            status: "success",
            requester_email: "owner@example.com",
            token_source: "estimated"
          }
        ],
        total: 77
      });
    }
    return json(500, { error: "unexpected request" });
  };

  const res = await listAdminLLMUsageEvents({
    from: new Date("2026-02-01T00:00:00Z"),
    to: new Date("2026-02-19T23:59:59Z"),
    userEmail: "owner@example.com",
    projectId: "project-1",
    domainId: "domain-1",
    model: "gemini-2.5-pro",
    operation: "editor_ai_suggest",
    status: "success",
    page: 2,
    limit: 20
  });

  assert.equal(res.items.length, 1);
  assert.equal(res.total, 77);

  const call = calls.find((item) => item.url.startsWith(`${base}/api/admin/llm-usage/events`));
  assert.ok(call, "Expected admin llm usage events request");
  if (call) {
    const params = new URL(call.url).searchParams;
    assert.equal(params.get("from"), "2026-02-01T00:00:00.000Z");
    assert.equal(params.get("to"), "2026-02-19T23:59:59.000Z");
    assert.equal(params.get("user_email"), "owner@example.com");
    assert.equal(params.get("project_id"), "project-1");
    assert.equal(params.get("domain_id"), "domain-1");
    assert.equal(params.get("model"), "gemini-2.5-pro");
    assert.equal(params.get("operation"), "editor_ai_suggest");
    assert.equal(params.get("status"), "success");
    assert.equal(params.get("page"), "2");
    assert.equal(params.get("limit"), "20");
  }

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
