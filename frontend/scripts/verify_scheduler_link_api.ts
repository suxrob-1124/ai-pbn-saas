import assert from "node:assert/strict";

import { apiBase } from "../lib/http";
import {
  createSchedule,
  deleteSchedule,
  getSchedule,
  listSchedules,
  triggerSchedule,
  updateSchedule
} from "../lib/schedulesApi";
import {
  deleteLinkSchedule,
  getLinkSchedule,
  triggerLinkSchedule,
  upsertLinkSchedule
} from "../lib/linkSchedulesApi";
import { deleteQueueItem, listQueue } from "../lib/queueApi";
import {
  createLinkTask,
  deleteLinkTask,
  importLinkTasks,
  listDomainLinkTasks,
  listLinkTasks,
  retryLinkTask,
  updateLinkTask
} from "../lib/linkTasksApi";

type Call = {
  url: string;
  method: string;
  body?: string;
};

async function main() {
  process.env.NEXT_PUBLIC_API_URL = "http://example.test";

  const base = apiBase();
  const calls: Call[] = [];

  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" }
    });

  globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const method = init?.method ? init.method.toUpperCase() : "GET";
    const body = typeof init?.body === "string" ? init.body : undefined;
    calls.push({ url, method, body });

    if (url === `${base}/api/refresh`) {
      return json(200, { status: "ok" });
    }

    if (url === `${base}/api/projects/proj-1/schedules` && method === "GET") {
      return json(200, [
        {
          id: "sched-1",
          project_id: "proj-1",
          name: "Daily",
          strategy: "daily",
          config: { cron: "0 9 * * *" },
          isActive: true,
          createdBy: "admin@example.com",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString()
        }
      ]);
    }

    if (url === `${base}/api/projects/proj-1/schedules` && method === "POST") {
      const payload = body ? JSON.parse(body) : null;
      if (payload?.name !== "Daily" || payload?.strategy !== "daily") {
        return json(400, { error: "invalid schedule" });
      }
      return json(201, {
        id: "sched-1",
        project_id: "proj-1",
        name: payload.name,
        strategy: payload.strategy,
        config: payload.config,
        isActive: true,
        createdBy: "admin@example.com",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString()
      });
    }

    if (url === `${base}/api/projects/proj-1/schedules/sched-1` && method === "GET") {
      return json(200, {
        id: "sched-1",
        project_id: "proj-1",
        name: "Daily",
        strategy: "daily",
        config: { cron: "0 9 * * *" },
        isActive: true,
        createdBy: "admin@example.com",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString()
      });
    }

    if (url === `${base}/api/projects/proj-1/schedules/sched-1` && method === "PATCH") {
      const payload = body ? JSON.parse(body) : null;
      if (payload?.name !== "Weekly") {
        return json(400, { error: "invalid update" });
      }
      return json(200, {
        id: "sched-1",
        project_id: "proj-1",
        name: payload.name,
        strategy: "daily",
        config: { cron: "0 9 * * *" },
        isActive: true,
        createdBy: "admin@example.com",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString()
      });
    }

    if (url === `${base}/api/projects/proj-1/schedules/sched-1` && method === "DELETE") {
      return json(200, { status: "deleted" });
    }

    if (
      url === `${base}/api/projects/proj-1/schedules/sched-1/trigger` &&
      method === "POST"
    ) {
      return json(202, { status: "queued", enqueued: 2 });
    }

    if (url === `${base}/api/projects/proj-1/link-schedule` && method === "GET") {
      return json(200, {
        id: "link-sched-1",
        project_id: "proj-1",
        name: "Links",
        strategy: "daily",
        config: { limit: 2, time: "09:00" },
        isActive: true,
        createdBy: "admin@example.com",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        nextRunAt: new Date().toISOString()
      });
    }

    if (url === `${base}/api/projects/proj-1/link-schedule` && method === "PUT") {
      const payload = body ? JSON.parse(body) : null;
      if (!payload?.name || !payload?.strategy) {
        return json(400, { error: "invalid link schedule" });
      }
      return json(200, {
        id: "link-sched-1",
        project_id: "proj-1",
        name: payload.name,
        strategy: payload.strategy,
        config: payload.config,
        isActive: payload.isActive ?? true,
        createdBy: "admin@example.com",
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        nextRunAt: new Date().toISOString()
      });
    }

    if (url === `${base}/api/projects/proj-1/link-schedule` && method === "DELETE") {
      return json(200, { status: "deleted" });
    }

    if (url === `${base}/api/projects/proj-1/link-schedule/trigger` && method === "POST") {
      return json(202, { status: "queued", enqueued: 1 });
    }

    if (url === `${base}/api/projects/proj-1/queue` && method === "GET") {
      return json(200, [
        {
          id: "queue-1",
          domain_id: "domain-1",
          schedule_id: "sched-1",
          priority: 0,
          scheduled_for: new Date().toISOString(),
          status: "pending",
          created_at: new Date().toISOString()
        }
      ]);
    }

    if (url === `${base}/api/queue/queue-1` && method === "DELETE") {
      return json(200, { status: "deleted" });
    }

    if (
      url ===
        `${base}/api/domains/domain-1/links?status=pending&scheduled_from=2026-02-01T00%3A00%3A00.000Z&limit=10` &&
      method === "GET"
    ) {
      return json(200, [
        {
          id: "task-1",
          domain_id: "domain-1",
          anchor_text: "Example",
          target_url: "https://example.com",
          scheduled_for: new Date().toISOString(),
          status: "pending",
          attempts: 0,
          created_by: "admin@example.com",
          created_at: new Date().toISOString()
        }
      ]);
    }

    if (url === `${base}/api/domains/domain-1/links` && method === "POST") {
      const payload = body ? JSON.parse(body) : null;
      if (payload?.anchor_text !== "Example" || payload?.target_url !== "https://example.com") {
        return json(400, { error: "invalid link task" });
      }
      return json(201, {
        id: "task-1",
        domain_id: "domain-1",
        anchor_text: payload.anchor_text,
        target_url: payload.target_url,
        scheduled_for: new Date().toISOString(),
        status: "pending",
        attempts: 0,
        created_by: "admin@example.com",
        created_at: new Date().toISOString()
      });
    }

    if (url === `${base}/api/domains/domain-1/links/import` && method === "POST") {
      return json(201, { created: 2 });
    }

    if (
      url === `${base}/api/links?project_id=proj-1&status=failed` &&
      method === "GET"
    ) {
      return json(200, []);
    }

    if (url === `${base}/api/links/task-1` && method === "PATCH") {
      const payload = body ? JSON.parse(body) : null;
      if (!payload?.scheduled_for) {
        return json(400, { error: "missing scheduled_for" });
      }
      return json(200, {
        id: "task-1",
        domain_id: "domain-1",
        anchor_text: "Example",
        target_url: "https://example.com",
        scheduled_for: payload.scheduled_for,
        status: "pending",
        attempts: 0,
        created_by: "admin@example.com",
        created_at: new Date().toISOString()
      });
    }

    if (url === `${base}/api/links/task-1/retry` && method === "POST") {
      return json(200, {
        id: "task-1",
        domain_id: "domain-1",
        anchor_text: "Example",
        target_url: "https://example.com",
        scheduled_for: new Date().toISOString(),
        status: "pending",
        attempts: 1,
        created_by: "admin@example.com",
        created_at: new Date().toISOString()
      });
    }

    if (url === `${base}/api/links/task-1` && method === "DELETE") {
      return json(200, { status: "deleted" });
    }

    if (url === `${base}/api/projects/missing/queue`) {
      return json(404, { error: "project not found" });
    }

    return json(500, { error: "unexpected request" });
  };

  const schedules = await listSchedules("proj-1");
  assert.equal(schedules.length, 1);

  const created = await createSchedule("proj-1", {
    name: "Daily",
    strategy: "daily",
    config: { cron: "0 9 * * *" }
  });
  assert.equal(created.id, "sched-1");

  const fetched = await getSchedule("proj-1", "sched-1");
  assert.equal(fetched.name, "Daily");

  const updated = await updateSchedule("proj-1", "sched-1", { name: "Weekly" });
  assert.equal(updated.name, "Weekly");

  const trigger = await triggerSchedule("proj-1", "sched-1");
  assert.equal(trigger.status, "queued");

  const deleted = await deleteSchedule("proj-1", "sched-1");
  assert.equal(deleted.status, "deleted");

  const linkSchedule = await getLinkSchedule("proj-1");
  assert.equal(linkSchedule?.id, "link-sched-1");

  const linkSaved = await upsertLinkSchedule("proj-1", {
    name: "Links",
    strategy: "daily",
    config: { limit: 2, time: "09:00" }
  });
  assert.equal(linkSaved.name, "Links");

  const linkTriggered = await triggerLinkSchedule("proj-1");
  assert.equal(linkTriggered.status, "queued");

  const linkScheduleDeleted = await deleteLinkSchedule("proj-1");
  assert.equal(linkScheduleDeleted.status, "deleted");

  const queue = await listQueue("proj-1");
  assert.equal(queue.length, 1);

  const queueDelete = await deleteQueueItem("queue-1");
  assert.equal(queueDelete.status, "deleted");

  const linkList = await listDomainLinkTasks("domain-1", {
    status: "pending",
    limit: 10,
    scheduledFrom: new Date("2026-02-01T00:00:00.000Z")
  });
  assert.equal(linkList.length, 1);

  const linkCreated = await createLinkTask("domain-1", {
    anchorText: "Example",
    targetUrl: "https://example.com"
  });
  assert.equal(linkCreated.id, "task-1");

  const importResult = await importLinkTasks("domain-1", {
    items: [
      { anchorText: "One", targetUrl: "https://one.example" },
      { anchorText: "Two", targetUrl: "https://two.example" }
    ]
  });
  assert.equal(importResult.created, 2);

  const allLinks = await listLinkTasks({ projectId: "proj-1", status: "failed" });
  assert.equal(allLinks.length, 0);

  const linkUpdated = await updateLinkTask("task-1", "2026-02-01T00:00:00.000Z");
  assert.equal(linkUpdated.id, "task-1");

  const linkRetry = await retryLinkTask("task-1");
  assert.equal(linkRetry.attempts, 1);

  const linkDeleted = await deleteLinkTask("task-1");
  assert.equal(linkDeleted.status, "deleted");

  let queueError = false;
  try {
    await listQueue("missing");
  } catch (err) {
    queueError = true;
    assert.match(String(err), /project not found/);
  }
  assert.ok(queueError, "expected error for missing queue project");

  const encodedProjectCall = calls.find((call) =>
    call.url.includes("/api/projects/proj-1/schedules")
  );
  assert.ok(encodedProjectCall, "expected schedule list call");

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
