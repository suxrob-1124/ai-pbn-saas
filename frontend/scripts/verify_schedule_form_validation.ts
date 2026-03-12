import assert from "node:assert/strict";
import { buildScheduleConfig, ScheduleFormValue } from "../lib/scheduleFormValidation";

const base: ScheduleFormValue = {
  name: "Daily",
  description: "",
  strategy: "daily",
  isActive: true,
  dailyLimit: "5",
  dailyTime: "09:00",
  weeklyLimit: "3",
  weeklyDay: "mon",
  weeklyTime: "10:00",
  customCron: "0 9 * * *",
  delayMinutes: "5",
};

const daily = buildScheduleConfig({ ...base, strategy: "daily" });
assert.equal(daily.ok, true);
if (daily.ok) {
  assert.equal(daily.config.limit, 5);
}

const weekly = buildScheduleConfig({ ...base, strategy: "weekly" });
assert.equal(weekly.ok, true);
if (weekly.ok) {
  assert.equal(weekly.config.weekday, "mon");
}

const custom = buildScheduleConfig({ ...base, strategy: "custom", customCron: "*/5 * * * *" });
assert.equal(custom.ok, true);

const invalidLimit = buildScheduleConfig({ ...base, strategy: "daily", dailyLimit: "0" });
assert.equal(invalidLimit.ok, false);
if (!invalidLimit.ok) {
  assert.match(invalidLimit.error, /Лимит/i);
}

const invalidCron = buildScheduleConfig({ ...base, strategy: "custom", customCron: "" });
assert.equal(invalidCron.ok, false);

console.log("OK");
