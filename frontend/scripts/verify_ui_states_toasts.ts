import assert from "node:assert/strict";

import { clearToasts, dismissToast, getToasts, showToast } from "../lib/toastStore";
import { createSchedule } from "../lib/schedulesApi";

async function main() {
  clearToasts();
  assert.equal(getToasts().length, 0);

  const toastId = showToast({
    type: "success",
    title: "Сохранено",
    message: "Расписание создано",
    timeoutMs: 0
  });
  assert.equal(getToasts().length, 1);
  assert.equal(getToasts()[0].id, toastId);

  const dismissed = dismissToast(toastId);
  assert.equal(dismissed, true);
  assert.equal(getToasts().length, 0);

  let errorThrown = false;
  try {
    await createSchedule("proj-1", {
      name: "",
      strategy: "daily",
      config: { cron: "0 9 * * *" }
    });
  } catch (err) {
    errorThrown = true;
    assert.match(String(err), /name is required/);
  }
  assert.ok(errorThrown, "expected createSchedule to fail on empty name");

  console.log("OK");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
