import assert from "node:assert/strict";

import { apiBase } from "../lib/http";
import {
  listAdmin,
  listAdminHistory,
  listByDomain,
  listByProject,
  listDomainHistory,
  listFailed,
  listProjectHistory,
  runManual,
  runManualProject
} from "../lib/indexChecksApi";

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" }
  });

const findCall = (calls: Array<{ url: string }>, path: string) =>
  calls.find((call) => call.url.includes(path));

async function main() {
  process.env.NEXT_PUBLIC_API_URL = "http://example.test";

  const base = apiBase();
  const calls: Array<{ url: string; method: string; body?: string }> = [];

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const method = init?.method ? init.method.toUpperCase() : "GET";
    const body = typeof init?.body === "string" ? init.body : undefined;
    calls.push({ url, method, body });

    const parsed = new URL(url);

    if (url === `${base}/api/refresh`) {
      return json(200, { status: "ok" });
    }

    if (parsed.pathname === "/api/domains/domain-1/index-checks" && method === "GET") {
      return json(200, [
        {
          id: "check-1",
          domain_id: "domain-1",
          domain_url: "example.com",
          check_date: new Date().toISOString(),
          status: "success",
          is_indexed: true,
          attempts: 1,
          created_at: new Date().toISOString()
        }
      ]);
    }

    if (parsed.pathname === "/api/domains/domain-1/index-checks" && method === "POST") {
      return json(200, {
        id: "check-2",
        domain_id: "domain-1",
        check_date: new Date().toISOString(),
        status: "pending",
        attempts: 0,
        created_at: new Date().toISOString()
      });
    }

    if (parsed.pathname === "/api/projects/project-1/index-checks" && method === "GET") {
      return json(200, []);
    }

    if (parsed.pathname === "/api/projects/project-1/index-checks" && method === "POST") {
      return json(200, { created: 1, updated: 1, skipped: 0 });
    }

    if (parsed.pathname === "/api/admin/index-checks" && method === "GET") {
      return json(200, []);
    }

    if (parsed.pathname === "/api/admin/index-checks/failed" && method === "GET") {
      return json(200, []);
    }

    if (
      parsed.pathname === "/api/domains/domain-1/index-checks/check-1/history" &&
      method === "GET"
    ) {
      return json(200, [
        {
          id: "hist-1",
          check_id: "check-1",
          attempt_number: 1,
          result: "success",
          created_at: new Date().toISOString()
        }
      ]);
    }

    if (
      parsed.pathname === "/api/projects/project-1/index-checks/check-1/history" &&
      method === "GET"
    ) {
      return json(200, []);
    }

    if (parsed.pathname === "/api/admin/index-checks/check-1/history" && method === "GET") {
      return json(200, []);
    }

    return json(500, { error: "unexpected request" });
  };

  const list = await listByDomain("domain-1", {
    status: "success",
    isIndexed: true,
    from: new Date("2026-02-01T00:00:00Z"),
    to: new Date("2026-02-12T00:00:00Z"),
    limit: 20,
    offset: 20,
    search: "example.com"
  });
  assert.equal(list.length, 1);
  assert.equal(list[0].domain_id, "domain-1");

  const manual = await runManual("domain-1");
  assert.equal(manual.status, "pending");

  await listByProject("project-1", { limit: 10, page: 2, search: "example" });

  const batch = await runManualProject("project-1");
  assert.equal(batch.created, 1);

  await listAdmin({ domainId: "domain-1", limit: 5 });
  await listFailed({ limit: 5, page: 1 });

  const domainHistory = await listDomainHistory("domain-1", "check-1", 25);
  assert.equal(domainHistory.length, 1);
  assert.equal(domainHistory[0].attempt_number, 1);

  await listProjectHistory("project-1", "check-1", 10);
  await listAdminHistory("check-1", 10);

  let missingDomain = false;
  try {
    await listByDomain("  ");
  } catch (err) {
    missingDomain = true;
    assert.match(String(err), /domainId is required/);
  }
  assert.ok(missingDomain, "expected error for missing domainId");

  const listCall = findCall(calls, "/api/domains/domain-1/index-checks");
  assert.ok(listCall, "expected listByDomain call");
  if (listCall) {
    const search = new URL(listCall.url).searchParams;
    assert.equal(search.get("status"), "success");
    assert.equal(search.get("is_indexed"), "true");
    assert.equal(search.get("limit"), "20");
    assert.equal(search.get("page"), "2");
    assert.equal(search.get("search"), "example.com");
  }

  const adminCall = findCall(calls, "/api/admin/index-checks");
  assert.ok(adminCall, "expected listAdmin call");
  if (adminCall) {
    const search = new URL(adminCall.url).searchParams;
    assert.equal(search.get("domain_id"), "domain-1");
    assert.equal(search.get("search"), "domain-1");
  }

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
